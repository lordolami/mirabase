package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"mirabase/core"
	"mirabase/internal/admission"
)

// Runtime is the per-server partitioning runtime: the registry service,
// the admission-coupled handle cache and the context resolver.
type Runtime struct {
	Service   *Service
	Handles   *HandleCache
	DataDir   string
	PrimaryDB *sql.DB
	Base      *core.BaseApp
}

var (
	runtimeMu sync.Mutex
	runtimes  = map[*core.BaseApp]*Runtime{}
)

// isSuperuserCollectionID recognizes the fixed system superusers
// collection id (pbc_3142635823) alongside its name.
func isSuperuserCollectionID(id string) bool {
	return id == "pbc_3142635823"
}

// baseAppOf normalizes a core.App to its concrete BaseApp, including
// wrappers that embed it — the test harness's TestApp embeds *core.BaseApp
// directly, and the launcher's MiraBase embeds the core.App interface
// (whose dynamic value is the concrete BaseApp).
func baseAppOf(app core.App) *core.BaseApp {
	if b, ok := app.(*core.BaseApp); ok {
		return b
	}
	v := reflect.ValueOf(app)
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
		if v.Kind() != reflect.Struct {
			return nil
		}
		if f := v.FieldByName("BaseApp"); f.IsValid() {
			if b, ok := f.Interface().(*core.BaseApp); ok {
				return b
			}
		}
		if f := v.FieldByName("App"); f.IsValid() && f.Kind() == reflect.Interface {
			inner, ok := f.Interface().(core.App)
			if !ok || inner == nil {
				return nil
			}
			// Recurse into the embedded interface (one level is enough for
			// the launcher wrapper; guard against self-reference).
			if iv := reflect.ValueOf(inner); iv != reflect.ValueOf(app) {
				if b := baseAppOf(inner); b != nil {
					return b
				}
			}
		}
	}
	return nil
}

// BindRuntime constructs (or returns) the partitioning runtime for a
// server app. The runtime binds the registry service to the primary
// database connection and the handle cache to the data directory.
func BindRuntime(app core.App) (*Runtime, error) {
	base := baseAppOf(app)
	if base == nil {
		return nil, errors.New("the partitioning runtime requires the concrete BaseApp")
	}
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if rt, ok := runtimes[base]; ok {
		return rt, nil
	}
	s, err := NewService(base.DBConn())
	if err != nil {
		return nil, err
	}
	rt := &Runtime{
		Service:   s,
		Handles:   NewHandleCache(base.DataDir(), admission.NewManager(admission.Bounds{})),
		DataDir:   base.DataDir(),
		PrimaryDB: base.DBConn(),
		Base:      base,
	}
	runtimes[base] = rt
	return rt, nil
}

// BaseAppOf is the exported form of baseAppOf for wire packages that must
// resolve the concrete BaseApp behind a wrapped app (launcher/test
// wrappers embed it directly or through the core.App interface).
func BaseAppOf(app core.App) *core.BaseApp { return baseAppOf(app) }

// RuntimeFor returns the bound runtime, if any.
func RuntimeFor(app core.App) *Runtime {
	base := baseAppOf(app)
	if base == nil {
		return nil
	}
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	return runtimes[base]
}

// Authorize verifies ownership or a live grant carrying the scope for a
// managed database. Invalid or unauthorized selection never falls back to
// primary; the primary database is owner-less and only routable through
// explicit bindings.
func (rt *Runtime) Authorize(ctx context.Context, databaseID, subjectCollection, subjectRecord, scope string) error {
	return rt.AuthorizeSubjects(ctx, databaseID, []string{subjectCollection}, subjectRecord, scope)
}

// AuthorizeSubjects verifies ownership or a live grant carrying the scope
// for a managed database. A subject may be referenced by collection name
// or by collection id (both resolve to the same identity; the fixture's
// `users` collection carries the preserved wire-format id
// `_pb_users_auth_`). Invalid or unauthorized selection never falls back
// to primary; the primary database is owner-less and only routable
// through explicit bindings.
func (rt *Runtime) AuthorizeSubjects(ctx context.Context, databaseID string, subjectCollections []string, subjectRecord, scope string) error {
	if subjectRecord == "" {
		return errors.New("the request requires valid record authorization token")
	}
	// Superuser authority: schema/lifecycle/global settings remain
	// superuser operations (partitioning.md); a superuser subject is
	// authorized on every database context.
	for _, sc := range subjectCollections {
		if sc == core.CollectionNameSuperusers || isSuperuserCollectionID(sc) {
			return nil
		}
	}
	d, err := rt.Service.GetDatabase(ctx, databaseID)
	if errors.Is(err, ErrNotFound) {
		return ErrUnknownDatabase
	}
	if err != nil {
		return err
	}
	// A request never routes to provisioning or failed state; quarantined,
	// deleting, tombstoned and purged databases do not serve either.
	if d.State != StateReady {
		return fmt.Errorf("%w: database state is %q", ErrUnknownDatabase, d.State)
	}
	if d.OwnerCollectionID != nil && d.OwnerRecordID != nil {
		for _, sc := range subjectCollections {
			if *d.OwnerCollectionID == sc && *d.OwnerRecordID == subjectRecord {
				return nil
			}
		}
	}
	for _, sc := range subjectCollections {
		if sc == "" {
			continue
		}
		g, err := rt.Service.GetGrant(ctx, databaseID, sc, subjectRecord)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if g.HasScope(scope) {
			return nil
		}
	}
	return ErrUnauthorizedDatabase
}

// ---------------------------------------------------------------------------
// Template publication (immutable revisions; v0.1: basic explicit-target
// fan-out at provisioning)
// ---------------------------------------------------------------------------

// templateTableDDL is the normative `_mb_schema_templates` DDL
// (storage-format-reference.md).
const templateTableDDL = `CREATE TABLE IF NOT EXISTS _mb_schema_templates (
 id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision > 0),
 definitions_hash TEXT NOT NULL,
 definitions_json TEXT NOT NULL CHECK(json_valid(definitions_json)),
 requirements_json TEXT NOT NULL CHECK(json_valid(requirements_json)),
 publication_state TEXT NOT NULL CHECK(publication_state IN ('draft','published')),
 PRIMARY KEY(id,revision)
)`

// TemplateDefinition is the published schema of one collection.
type TemplateDefinition struct {
	CollectionID   string            `json:"collectionId"`
	CollectionName string            `json:"collectionName"`
	Type           string            `json:"type"`
	Fields         []TemplateField   `json:"fields"`
	ListRule       *string           `json:"listRule"`
	ViewRule       *string           `json:"viewRule"`
	CreateRule     *string           `json:"createRule"`
	UpdateRule     *string           `json:"updateRule"`
	DeleteRule     *string           `json:"deleteRule"`
	Indexes        []string          `json:"indexes"`
	Extras         map[string]string `json:"extras,omitempty"`
}

// TemplateField is one field's storage contract in a template (the
// hidden flag carries the field protection the derived surfaces enforce
// before results leave the server).
type TemplateField struct {
	Name       string `json:"name"`
	ColumnType string `json:"columnType"`
	Hidden     bool   `json:"hidden,omitempty"`
}

// PublishedTemplate is one published template revision.
type PublishedTemplate struct {
	ID           string
	Revision     int64
	Definitions  []TemplateDefinition
	Requirements map[string]any
	State        string
}

// HasTemplatesTable ensures the templates table exists (used by wire
// surfaces that only list).
func (s *Service) HasTemplatesTable(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, templateTableDDL); err != nil {
		return fmt.Errorf("failed to install templates table: %w", err)
	}
	return nil
}

// PublishTemplate snapshots the named collections into a new immutable
// template revision ("Immutable template revisions") and marks it
// published.
func (s *Service) PublishTemplate(ctx context.Context, app core.App, id string, collectionNames []string) (*PublishedTemplate, error) {
	if _, err := s.db.ExecContext(ctx, templateTableDDL); err != nil {
		return nil, fmt.Errorf("failed to install templates table: %w", err)
	}

	definitions := make([]TemplateDefinition, 0, len(collectionNames))
	for _, name := range collectionNames {
		coll, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return nil, fmt.Errorf("template collection %q: %w", name, err)
		}
		def := TemplateDefinition{
			CollectionID:   coll.Id,
			CollectionName: coll.Name,
			Type:           coll.Type,
			ListRule:       coll.ListRule,
			ViewRule:       coll.ViewRule,
			CreateRule:     coll.CreateRule,
			UpdateRule:     coll.UpdateRule,
			DeleteRule:     coll.DeleteRule,
			Indexes:        append([]string(nil), coll.Indexes...),
		}
		for _, f := range coll.Fields {
			def.Fields = append(def.Fields, TemplateField{
				Name:       f.GetName(),
				ColumnType: f.ColumnType(app),
				Hidden:     f.GetHidden(),
			})
		}
		definitions = append(definitions, def)
	}

	defsJSON, err := json.Marshal(definitions)
	if err != nil {
		return nil, err
	}
	hashRaw := sha256.Sum256(defsJSON)
	hash := fmt.Sprintf("%x", hashRaw)
	reqsJSON, _ := json.Marshal(map[string]any{"v": 1})

	// Revision = next for this id (immutable: existing revisions are never
	// rewritten).
	var revision int64
	err = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM _mb_schema_templates WHERE id = ?`, id).Scan(&revision)
	if err != nil {
		return nil, err
	}
	revision++
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO _mb_schema_templates (id, revision, definitions_hash, definitions_json, requirements_json, publication_state)
		 VALUES (?, ?, ?, ?, ?, 'published')`,
		id, revision, hash, string(defsJSON), string(reqsJSON))
	if err != nil {
		return nil, err
	}
	return &PublishedTemplate{
		ID:          id,
		Revision:    revision,
		Definitions: definitions,
		State:       "published",
	}, nil
}

// Hash returns the template's definitions hash.
func (tpl *PublishedTemplate) Hash() string {
	b, err := json.Marshal(tpl.Definitions)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("%x", h)
}

// LoadTemplate loads a published template revision.
func (s *Service) LoadTemplate(ctx context.Context, id string, revision int64) (*PublishedTemplate, error) {
	var defsJSON, state string
	var reqsJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT definitions_json, requirements_json, publication_state FROM _mb_schema_templates
		 WHERE id = ? AND revision = ? AND publication_state = 'published'`,
		id, revision).Scan(&defsJSON, &reqsJSON, &state)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var defs []TemplateDefinition
	if err := json.Unmarshal([]byte(defsJSON), &defs); err != nil {
		return nil, err
	}
	var reqs map[string]any
	_ = json.Unmarshal([]byte(reqsJSON), &reqs)
	return &PublishedTemplate{ID: id, Revision: revision, Definitions: defs, Requirements: reqs, State: state}, nil
}

// LoadLatestPublishedTemplate loads the latest published revision of a
// template (the target's collection/rule source for derived surfaces).
func (s *Service) LoadLatestPublishedTemplate(ctx context.Context, id string) (*PublishedTemplate, error) {
	var revision int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM _mb_schema_templates WHERE id = ? AND publication_state = 'published'`, id).Scan(&revision); err != nil {
		return nil, err
	}
	if revision == 0 {
		return nil, ErrNotFound
	}
	return s.LoadTemplate(ctx, id, revision)
}

// ApplyTemplate creates the published collection tables in a target
// database ("Private DBs contain only their published collection set and
// required local metadata"). Idempotent: existing tables are left as-is.
func ApplyTemplate(ctx context.Context, target *sql.DB, tpl *PublishedTemplate) error {
	for _, def := range tpl.Definitions {
		cols := []string{`"id" TEXT PRIMARY KEY NOT NULL DEFAULT ('r'||lower(hex(randomblob(7))))`}
		mandatory := map[string]bool{"id": true, "created": true, "updated": true}
		for _, f := range def.Fields {
			if mandatory[strings.ToLower(f.Name)] {
				continue // already present as the system column
			}
			name := `"` + strings.ReplaceAll(f.Name, `"`, `""`) + `"`
			cols = append(cols, name+" "+f.ColumnType)
		}
		cols = append(cols,
			`"created" TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ'))`,
			`"updated" TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ'))`,
		)
		ddl := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (%s)`,
			quoteIdent(def.CollectionName), strings.Join(cols, ", "))
		if _, err := target.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("template table %q: %w", def.CollectionName, err)
		}
		for _, idx := range def.Indexes {
			if _, err := target.ExecContext(ctx, idx); err != nil {
				// Re-applying an immutable template may encounter its existing
				// index. Every other failure prevents a ready result.
				if strings.Contains(strings.ToLower(err.Error()), "already exists") {
					continue
				}
				return fmt.Errorf("template index for %q: %w", def.CollectionName, err)
			}
		}
	}
	return nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// ManagedCollectionNames returns the managed collection tables of one
// database (postgresql-access.md: "Managed collection DDL must reconcile
// _collections/type/template/index metadata ... Until a statement has that
// reconciliation adapter, reject it for managed objects"). The primary
// database keeps the live `_collections` catalog; a managed target maps to
// the union of collection names across every published revision of its
// template (a superset, so schema drift between revisions stays refused).
// Databases without a template own no managed objects.
func (s *Service) ManagedCollectionNames(ctx context.Context, databaseID string) (map[string]bool, error) {
	if databaseID == "" || databaseID == PrimaryDatabaseID {
		rows, err := s.db.QueryContext(ctx, `SELECT name FROM _collections`)
		if err != nil {
			// A primary without a catalog carries no managed objects.
			return map[string]bool{}, nil
		}
		defer rows.Close()
		out := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			out[name] = true
		}
		return out, rows.Err()
	}
	d, err := s.GetDatabase(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	if d.TemplateID == nil || *d.TemplateID == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT definitions_json FROM _mb_schema_templates WHERE id = ? AND publication_state = 'published'`,
		*d.TemplateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var defsJSON string
		if err := rows.Scan(&defsJSON); err != nil {
			return nil, err
		}
		var defs []TemplateDefinition
		if err := json.Unmarshal([]byte(defsJSON), &defs); err != nil {
			return nil, err
		}
		for _, def := range defs {
			out[def.CollectionName] = true
		}
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Record store over a target database
// ---------------------------------------------------------------------------

// RecordStore performs context-bound record CRUD on a target database
// table with the template's column shape. The store is permission-checked
// by the caller (ownership or a live grant); ordinary rules still filter
// per the template's rule configuration at the API layer.
type RecordStore struct {
	target *sql.DB
	table  string
}

// NewRecordStore binds the store to a target handle and collection table.
func NewRecordStore(target *sql.DB, collectionName string) *RecordStore {
	return &RecordStore{target: target, table: collectionName}
}

// Columns lists the collection table's columns.
func (r *RecordStore) Columns(ctx context.Context) ([]string, error) {
	rows, err := r.target.QueryContext(ctx,
		`SELECT name FROM PRAGMA_TABLE_INFO(`+quoteIdent(r.table)+`)`)
	if err != nil {
		return nil, fmt.Errorf("collection %q is not published in this database: %w", r.table, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if len(out) == 0 && rows.Err() == nil {
		return nil, ErrUnknownCollection
	}
	return out, rows.Err()
}

// ErrUnknownCollection reports a collection missing from the target.
var ErrUnknownCollection = errors.New("unknown collection in the target database")

// ChangeSink receives the CDC captures of record writes (nil disables
// capture).
type ChangeSink interface {
	Table() string
}

// Create inserts a record; id is generated server-side. Returns the
// stored record as a JSON-ready map.
func (r *RecordStore) Create(ctx context.Context, data map[string]any) (map[string]any, error) {
	return r.CreateWithCapture(ctx, data, nil)
}

// CreateWithCapture inserts a record and captures the change when the
// sink is non-nil (atomic capture inside the same transaction).
func (r *RecordStore) CreateWithCapture(ctx context.Context, data map[string]any, capture *TableCapture) (map[string]any, error) {
	cols, err := r.Columns(ctx)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, c := range cols {
		known[strings.ToLower(c)] = true
	}
	colNames := []string{`"id"`}
	values := []any{randomHex(15)}
	for k, v := range data {
		if !known[strings.ToLower(k)] {
			return nil, fmt.Errorf("unknown field %q for collection %q", k, r.table)
		}
		if strings.EqualFold(k, "id") {
			return nil, errors.New("the record id is server-generated")
		}
		colNames = append(colNames, quoteIdent(k))
		values = append(values, encodeRecordValue(v))
	}
	if len(colNames) == 1 {
		return nil, errors.New("the record requires at least one field value")
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(colNames)), ", ")
	stmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		quoteIdent(r.table), strings.Join(colNames, ", "), placeholders)
	// Atomic capture: the CDC change row commits with the data it
	// describes (MB-CDC).
	if capture != nil {
		tx, terr := r.target.BeginTx(ctx, nil)
		if terr != nil {
			return nil, terr
		}
		if _, xerr := tx.ExecContext(ctx, stmt, values...); xerr != nil {
			_ = tx.Rollback()
			return nil, xerr
		}
		after, aerr := readRecordRow(ctx, tx, r.table, values[0].(string))
		if aerr != nil {
			_ = tx.Rollback()
			return nil, aerr
		}
		colID := ""
		if capture != nil {
			colID = capture.CollectionID
		}
		if _, cerr := CaptureChanges(ctx, tx, "record", []ChangeCapture{{
			Table: r.table, RecordID: values[0].(string), Action: "create", After: after,
			CollectionID: collectionIDPtr(colID),
		}}); cerr != nil {
			_ = tx.Rollback()
			return nil, cerr
		}
		if cerr := tx.Commit(); cerr != nil {
			return nil, cerr
		}
		return r.Get(ctx, values[0].(string))
	}
	if _, err := r.target.ExecContext(ctx, stmt, values...); err != nil {
		return nil, err
	}
	return r.Get(ctx, values[0].(string))
}

// readRecordRow reads one row through the transaction (the capture's
// after-image).
func readRecordRow(ctx context.Context, tx *sql.Tx, table, id string) (map[string]any, error) {
	row, err := tx.QueryContext(ctx,
		fmt.Sprintf(`SELECT * FROM %s WHERE "id" = ? LIMIT 1`, quoteIdent(table)), id)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	return rowToMap(row)
}

// CreateWithID inserts a record with the CLIENT-specified stable id (the
// sync protocol: "preserving IDs"). The capture semantics match Create.
func (r *RecordStore) CreateWithID(ctx context.Context, recordID string, data map[string]any, capture *TableCapture) (map[string]any, error) {
	cols, err := r.Columns(ctx)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, c := range cols {
		known[strings.ToLower(c)] = true
	}
	if recordID == "" {
		return nil, errors.New("the sync create requires a record id")
	}
	colNames := []string{`"id"`}
	values := []any{recordID}
	for k, v := range data {
		if !known[strings.ToLower(k)] {
			return nil, fmt.Errorf("unknown field %q for collection %q", k, r.table)
		}
		if strings.EqualFold(k, "id") {
			continue // the client id is authoritative
		}
		colNames = append(colNames, quoteIdent(k))
		values = append(values, encodeRecordValue(v))
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(colNames)), ", ")
	stmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`,
		quoteIdent(r.table), strings.Join(colNames, ", "), placeholders)
	if capture != nil {
		tx, terr := r.target.BeginTx(ctx, nil)
		if terr != nil {
			return nil, terr
		}
		if _, xerr := tx.ExecContext(ctx, stmt, values...); xerr != nil {
			_ = tx.Rollback()
			return nil, xerr
		}
		after, aerr := readRecordRow(ctx, tx, r.table, recordID)
		if aerr != nil {
			_ = tx.Rollback()
			return nil, aerr
		}
		colID := ""
		if capture != nil {
			colID = capture.CollectionID
		}
		if _, cerr := CaptureChanges(ctx, tx, "record", []ChangeCapture{{
			Table: r.table, RecordID: recordID, Action: "create", After: after,
			CollectionID: collectionIDPtr(colID),
		}}); cerr != nil {
			_ = tx.Rollback()
			return nil, cerr
		}
		if cerr := tx.Commit(); cerr != nil {
			return nil, cerr
		}
		return r.Get(ctx, recordID)
	}
	if _, err := r.target.ExecContext(ctx, stmt, values...); err != nil {
		return nil, err
	}
	return r.Get(ctx, recordID)
}

// TableCapture binds one capture to its collection id.
type TableCapture struct{ CollectionID string }

// collectionIDPtr returns a string pointer (nil for empty).
func collectionIDPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Table implements ChangeSink.
func (t *TableCapture) Table() string { return t.CollectionID }

// Get reads one record by id.
func (r *RecordStore) Get(ctx context.Context, id string) (map[string]any, error) {
	row, err := r.target.QueryContext(ctx,
		fmt.Sprintf(`SELECT * FROM %s WHERE "id" = ? LIMIT 1`, quoteIdent(r.table)), id)
	if err != nil {
		return nil, err
	}
	defer row.Close()
	m, err := rowToMap(row)
	if errors.Is(err, sql.ErrNoRows) || (m != nil && len(m) == 0) {
		return nil, ErrNotFound
	}
	return m, err
}

// List returns records ordered by created desc with a bound.
func (r *RecordStore) List(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.target.QueryContext(ctx,
		fmt.Sprintf(`SELECT * FROM %s ORDER BY created DESC LIMIT %d`, quoteIdent(r.table), limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rowsToMaps(rows)
}

// Update patches the given fields on a record.
func (r *RecordStore) Update(ctx context.Context, id string, data map[string]any) (map[string]any, error) {
	return r.UpdateWithCapture(ctx, id, data, nil)
}

// UpdateWithCapture patches the given fields on a record and captures the
// change when the sink is non-nil (atomic capture: the CDC change row
// commits with the data it describes, so every transport — record PATCH,
// offline sync push — journals update commits and live-query subscribers
// re-evaluate on them, D-14: "Durable changes remain the interface for
// every transaction").
func (r *RecordStore) UpdateWithCapture(ctx context.Context, id string, data map[string]any, capture *TableCapture) (map[string]any, error) {
	cols, err := r.Columns(ctx)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, c := range cols {
		known[strings.ToLower(c)] = true
	}
	sets := []string{`"updated" = ?`}
	values := []any{time.Now().UTC().Format("2006-01-02 15:04:05.000Z")}
	for k, v := range data {
		if !known[strings.ToLower(k)] {
			return nil, fmt.Errorf("unknown field %q for collection %q", k, r.table)
		}
		if strings.EqualFold(k, "id") {
			return nil, errors.New("the record id is immutable")
		}
		sets = append(sets, quoteIdent(k)+" = ?")
		values = append(values, encodeRecordValue(v))
	}
	values = append(values, id)
	stmt := fmt.Sprintf(`UPDATE %s SET %s WHERE "id" = ?`, quoteIdent(r.table), strings.Join(sets, ", "))

	if capture == nil {
		res, err := r.target.ExecContext(ctx, stmt, values...)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, ErrNotFound
		}
		return r.Get(ctx, id)
	}

	tx, terr := r.target.BeginTx(ctx, nil)
	if terr != nil {
		return nil, terr
	}
	before, berr := readRecordRow(ctx, tx, r.table, id)
	if berr != nil {
		_ = tx.Rollback()
		if errors.Is(berr, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, berr
	}
	res, xerr := tx.ExecContext(ctx, stmt, values...)
	if xerr != nil {
		_ = tx.Rollback()
		return nil, xerr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_ = tx.Rollback()
		return nil, ErrNotFound
	}
	after, aerr := readRecordRow(ctx, tx, r.table, id)
	if aerr != nil {
		_ = tx.Rollback()
		return nil, aerr
	}
	if _, cerr := CaptureChanges(ctx, tx, "record", []ChangeCapture{{
		Table: r.table, RecordID: id, Action: "update", Before: before, After: after,
		CollectionID: collectionIDPtr(capture.CollectionID),
	}}); cerr != nil {
		_ = tx.Rollback()
		return nil, cerr
	}
	if cerr := tx.Commit(); cerr != nil {
		return nil, cerr
	}
	return r.Get(ctx, id)
}

// Delete removes a record.
func (r *RecordStore) Delete(ctx context.Context, id string) error {
	return r.DeleteWithCapture(ctx, id, nil)
}

// DeleteWithCapture removes a record and captures the tombstone when the
// sink is non-nil: the before-image and the deletion commit in one
// transaction, so pull cursors and live-query subscribers observe the
// deletion (AT-LIVE's required deletion checks).
func (r *RecordStore) DeleteWithCapture(ctx context.Context, id string, capture *TableCapture) error {
	stmt := fmt.Sprintf(`DELETE FROM %s WHERE "id" = ?`, quoteIdent(r.table))

	if capture == nil {
		res, err := r.target.ExecContext(ctx, stmt, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		return nil
	}

	tx, terr := r.target.BeginTx(ctx, nil)
	if terr != nil {
		return terr
	}
	before, berr := readRecordRow(ctx, tx, r.table, id)
	if berr != nil {
		_ = tx.Rollback()
		if errors.Is(berr, sql.ErrNoRows) {
			return ErrNotFound
		}
		return berr
	}
	res, xerr := tx.ExecContext(ctx, stmt, id)
	if xerr != nil {
		_ = tx.Rollback()
		return xerr
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_ = tx.Rollback()
		return ErrNotFound
	}
	if _, cerr := CaptureChanges(ctx, tx, "record", []ChangeCapture{{
		Table: r.table, RecordID: id, Action: "delete", Before: before,
		CollectionID: collectionIDPtr(capture.CollectionID),
	}}); cerr != nil {
		_ = tx.Rollback()
		return cerr
	}
	if cerr := tx.Commit(); cerr != nil {
		return cerr
	}
	return nil
}

// encodeRecordValue adapts Go values to engine storage values (bool →
// int, maps/slices → JSON text; typed slices like []float64 marshal as
// JSON text so vector embeddings round-trip).
func encodeRecordValue(v any) any {
	switch t := v.(type) {
	case bool:
		if t {
			return 1
		}
		return 0
	case nil:
		return nil
	case string, int, int64, float64:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

func rowToMap(rows *sql.Rows) (map[string]any, error) {
	m, err := rowsToMaps(rows)
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, sql.ErrNoRows
	}
	return m[0], nil
}

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		ptrs := make([]any, len(cols))
		raw := make([]sql.RawBytes, len(cols))
		for i := range ptrs {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			if raw[i] == nil {
				m[c] = nil
			} else {
				m[c] = string(raw[i])
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
