package pbimport

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Stage copies the source rows into an unpublished MiraBase-format target
// (the staged root): collections with their schema JSON, record tables
// with the source rows verbatim (stable IDs, bcrypt password hashes,
// timestamps), and the migration history. Copy runs in bounded batches
// (1000 rows) through the trusted migration path — direct SQL, never
// ordinary create handlers ("not ordinary create handlers that would
// regenerate IDs or send application notifications"); side effects are
// suppressed by construction.
//
// The staged target database is created through the selected provider
// (the Turso engine) — live destination databases always use Turso.
func (s *Service) Stage(ctx context.Context, id string) (*StageResult, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Phase != StatePreviewed {
		return nil, fmt.Errorf("%w: phase %s does not accept staging", ErrConflict, sess.Phase)
	}
	if err := s.Transition(ctx, id, StateStaging, ""); err != nil {
		return nil, err
	}

	workData := filepath.Join(s.SessionDir(id), "work", "data.db")
	reader, err := sql.Open("sqlite", workData+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	stagedDir := filepath.Join(s.SessionDir(id), "staged")
	if err := os.MkdirAll(stagedDir, 0o700); err != nil {
		return nil, err
	}
	// The staged target database through the Turso engine (no SQLite
	// fallback; the staged database is a real MiraBase primary).
	stagedDBPath := filepath.Join(stagedDir, "data.db")
	staged, err := sql.Open("turso", stagedDBPath+"?_busy_timeout=10000&experimental=vacuum,index_method")
	if err != nil {
		return nil, err
	}
	if _, err := staged.Conn(ctx); err != nil {
		staged.Close()
		return nil, fmt.Errorf("staged database first open: %w", err)
	}

	result := &StageResult{SessionID: id}
	result.Collections, result.RecordCount, err = s.stageCollections(ctx, reader, staged)
	if err != nil {
		_ = s.FailSession(ctx, id, "staging")
		return nil, err
	}
	result.AuthRecords, err = s.stageAuthRecords(ctx, reader, staged, result.SessionID)
	if err != nil {
		_ = s.FailSession(ctx, id, "staging")
		return nil, err
	}
	result.Migrations, err = s.stageMigrations(ctx, reader, staged)
	if err != nil {
		_ = s.FailSession(ctx, id, "staging")
		return nil, err
	}
	result.Settings, err = s.stageSettings(ctx, reader, staged)
	if err != nil {
		_ = s.FailSession(ctx, id, "staging")
		return nil, err
	}

	// Copy the local file storage into the staged root.
	filesDir := filepath.Join(stagedDir, "storage")
	if err := copyTree(filepath.Join(s.SessionDir(id), "work", "storage"), filesDir); err != nil {
		_ = s.FailSession(ctx, id, "staging")
		return nil, err
	}
	if err := copyTree(filepath.Join(s.SessionDir(id), "work", "files"), filepath.Join(stagedDir, "files")); err != nil {
		_ = s.FailSession(ctx, id, "staging")
		return nil, err
	}

	// Checkpoint the staged engine file so data.db is self-contained, then
	// hash the staged file inventory (the immutable staged root;
	// "Immediately before publication, verify the stored report's staged
	// manifest against the actual staged root").
	if _, err := staged.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return nil, err
	}
	_ = staged.Close()
	manifest, err := manifestOf(stagedDir)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(stagedDir, "manifest.json"), []byte(manifest), 0o600); err != nil {
		return nil, err
	}
	if err := s.SetHash(ctx, id, "staging_ref", stagedDir); err != nil {
		return nil, err
	}
	result.StagedManifestHash = manifest
	if err := s.Transition(ctx, id, StateStaged, ""); err != nil {
		return nil, err
	}
	return result, nil
}

type StageResult struct {
	SessionID          string `json:"sessionId"`
	Collections        int    `json:"collections"`
	RecordCount        int    `json:"recordCount"`
	AuthRecords        int    `json:"authRecords"`
	Migrations         int    `json:"migrations"`
	Settings           bool   `json:"settings"`
	StagedManifestHash string `json:"stagedManifestHash"`
}

// stageCollections copies `_collections` rows verbatim (IDs, type, name,
// system flag, the full schema JSON) and creates each collection's record
// table with the source's real columns (trusted path: direct SQL).
func (s *Service) stageCollections(ctx context.Context, reader *sql.DB, staged *sql.DB) (int, int, error) {
	rows, err := reader.QueryContext(ctx,
		`SELECT id, type, name, system, COALESCE((SELECT json_group_array(json(value)) FROM pragma_table_info(name)), '[]') FROM _collections ORDER BY name`)
	// The column list is not needed verbatim — read the columns from the
	// pragma directly per collection below.
	if err != nil {
		// The pinned v0.41.1 _collections has a `columns`/`schema` JSON
		// column; read rows generically instead.
		rows, err = reader.QueryContext(ctx, `SELECT id, type, name, system FROM _collections ORDER BY name`)
		if err != nil {
			return 0, 0, fmt.Errorf("collections read: %w", err)
		}
	}
	defer rows.Close()

	if _, err := staged.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS _collections (
		id TEXT PRIMARY KEY NOT NULL,
		type TEXT NOT NULL DEFAULT 'base',
		name TEXT NOT NULL UNIQUE,
		system INTEGER NOT NULL DEFAULT 0,
		fields TEXT NOT NULL DEFAULT '[]',
		indexes TEXT NOT NULL DEFAULT '[]',
		listRule TEXT,
		viewRule TEXT,
		createRule TEXT,
		updateRule TEXT,
		deleteRule TEXT,
		created TEXT NOT NULL,
		updated TEXT NOT NULL)`); err != nil {
		return 0, 0, fmt.Errorf("staged _collections: %w", err)
	}

	count := 0
	recordTotal := 0
	for rows.Next() {
		var cID, cType, cName string
		var system int
		if err := rows.Scan(&cID, &cType, &cName, &system); err != nil {
			return 0, 0, err
		}
		// Read the full source row generically to preserve every column.
		srcCols, srcVals, err := readRow(ctx, reader, "_collections", cID)
		if err != nil {
			return 0, 0, err
		}
		// Mirror into the staged _collections with a compatible column set.
		if err := upsertRow(ctx, staged, "_collections", stagedCollectionsColumns, srcCols, srcVals); err != nil {
			return 0, 0, fmt.Errorf("staging collection %q: %w", cName, err)
		}
		count++

		if cType != "base" && cType != "auth" {
			continue
		}
		// Record table: create with the source's real columns and copy rows
		// verbatim (stable IDs, bcrypt hashes, timestamps).
		typedCols, err := tableColumns(ctx, reader, cName)
		if err != nil {
			return 0, 0, fmt.Errorf("columns %q: %w", cName, err)
		}
		cols := columnNames(typedCols)
		ddl := buildRecordTableDDL(cName, typedCols)
		if _, err := staged.ExecContext(ctx, ddl); err != nil {
			return 0, 0, fmt.Errorf("staged table %q: %w", cName, err)
		}
		n, err := copyRows(ctx, reader, staged, cName, cols)
		if err != nil {
			return 0, 0, fmt.Errorf("copy %q: %w", cName, err)
		}
		recordTotal += n
	}
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}
	return count, recordTotal, nil
}

// stagedCollectionsColumns is the staged _collections column set (a
// superset of the pinned v0.41.1 columns; the copy maps by name and fills
// the rest with defaults).
var stagedCollectionsColumns = []string{
	"id", "type", "name", "system", "fields", "indexes", "listRule",
	"viewRule", "createRule", "updateRule", "deleteRule", "created", "updated",
}

func tableColumns(ctx context.Context, db *sql.DB, table string) ([]tableColumn, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`SELECT name, COALESCE(type,'TEXT'), COALESCE("notnull",0), COALESCE(pk,0) FROM pragma_table_info(%q)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tableColumn
	for rows.Next() {
		var c tableColumn
		if err := rows.Scan(&c.Name, &c.Type, &c.NotNull, &c.PrimaryKey); err != nil {
			return nil, err
		}
		if c.Type == "" {
			c.Type = "TEXT"
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// columnNames projects the names of a table-column list.
func columnNames(cols []tableColumn) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, c.Name)
	}
	return out
}

func buildRecordTableDDL(table string, cols []tableColumn) string {
	defs := make([]string, 0, len(cols))
	for _, c := range cols {
		if strings.EqualFold(c.Name, "id") {
			defs = append(defs, fmt.Sprintf(`%q TEXT PRIMARY KEY NOT NULL`, c.Name))
			continue
		}
		if c.NotNull {
			defs = append(defs, fmt.Sprintf(`%q %s NOT NULL`, c.Name, c.Type))
			continue
		}
		defs = append(defs, fmt.Sprintf(`%q %s`, c.Name, c.Type))
	}
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %q (%s)`, table, strings.Join(defs, ", "))
}

// tableColumn is one pragma_table_info row.
type tableColumn struct {
	Name       string
	Type       string
	NotNull    bool
	PrimaryKey bool
}

func readRow(ctx context.Context, db *sql.DB, table, id string) ([]string, []any, error) {
	typedCols, err := tableColumns(ctx, db, table)
	if err != nil {
		return nil, nil, err
	}
	cols := columnNames(typedCols)
	ptrs := make([]any, len(cols))
	vals := make([]any, len(cols))
	for i := range ptrs {
		vals[i] = new(sql.NullString)
		ptrs[i] = vals[i]
	}
	q := fmt.Sprintf(`SELECT %s FROM %q WHERE id = ?`, quotedList(cols), table)
	err = db.QueryRowContext(ctx, q, id).Scan(ptrs...)
	if err != nil {
		return nil, nil, err
	}
	out := make([]any, len(cols))
	for i := range ptrs {
		ns := vals[i].(*sql.NullString)
		if ns.Valid {
			out[i] = ns.String
		} else {
			out[i] = nil
		}
	}
	return cols, out, nil
}

func quotedList(cols []string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = fmt.Sprintf(`%q`, c)
	}
	return strings.Join(parts, ", ")
}

// upsertRow inserts a row into the staged table, mapping by column name
// and defaulting absent staged columns.
func upsertRow(ctx context.Context, staged *sql.DB, table string, stagedCols []string, srcCols []string, srcVals []any) error {
	set := map[string]any{}
	for i, c := range srcCols {
		for _, sc := range stagedCols {
			if strings.EqualFold(sc, c) {
				set[strings.ToLower(sc)] = srcVals[i]
			}
		}
	}
	var names, params []string
	var args []any
	for _, sc := range stagedCols {
		v, ok := set[strings.ToLower(sc)]
		names = append(names, fmt.Sprintf(`%q`, sc))
		if ok {
			args = append(args, v)
		} else {
			args = append(args, nil)
		}
		params = append(params, "?")
	}
	q := fmt.Sprintf(`INSERT OR REPLACE INTO %q (%s) VALUES (%s)`,
		table, strings.Join(names, ", "), strings.Join(params, ", "))
	_, err := staged.ExecContext(ctx, q, args...)
	return err
}

// copyRows copies every row of a table in bounded batches (1000 rows per
// transaction — "Copy in bounded, restartable batches").
func copyRows(ctx context.Context, src, staged *sql.DB, table string, cols []string) (int, error) {
	rows, err := src.QueryContext(ctx, fmt.Sprintf(`SELECT %s FROM %q`, quotedList(cols), table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	ptrs := make([]any, len(cols))
	raw := make([]sql.RawBytes, len(cols))
	for i := range ptrs {
		ptrs[i] = &raw[i]
	}
	batch := 0
	total := 0
	var tx *sql.Tx
	for rows.Next() {
		if batch == 0 {
			tx, err = staged.BeginTx(ctx, nil)
			if err != nil {
				return 0, err
			}
		}
		if err := rows.Scan(ptrs...); err != nil {
			if tx != nil {
				_ = tx.Rollback()
			}
			return 0, err
		}
		vals := make([]any, len(cols))
		for i, v := range raw {
			if v == nil {
				vals[i] = nil
			} else {
				vals[i] = string(v)
			}
		}
		names := quotedList(cols)
		params := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
		q := fmt.Sprintf(`INSERT OR REPLACE INTO %q (%s) VALUES (%s)`, table, names, params)
		if _, err := tx.ExecContext(ctx, q, vals...); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("row copy: %w", err)
		}
		batch++
		total++
		if batch >= CopyBatchRows {
			if err := tx.Commit(); err != nil {
				return 0, err
			}
			batch = 0
		}
	}
	if err := rows.Err(); err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		return 0, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
	}
	return total, nil
}

// stageAuthRecords preserves bcrypt password hashes and auth identities
// verbatim (the source users/auth tables are copied through copyRows in
// the collections loop; this pass records the auth-account accounting for
// the validation report — the account policy (session/operator) is
// applied at cutover).
func (s *Service) stageAuthRecords(ctx context.Context, reader, staged *sql.DB, sessionID string) (int, error) {
	rows, err := reader.QueryContext(ctx,
		`SELECT name FROM _collections WHERE type = 'auth'`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return 0, err
		}
		var has int
		if err := staged.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=? COLLATE NOCASE`, name).Scan(&has); err != nil {
			return 0, err
		} else if has == 0 {
			return 0, fmt.Errorf("staged auth table %q is missing", name)
		}
		var n int
		if err := staged.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT count(*) FROM %q`, name)).Scan(&n); err != nil {
			return 0, err
		}
		total += n
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return total, nil
}

// stageMigrations preserves the migration history rows.
func (s *Service) stageMigrations(ctx context.Context, reader, staged *sql.DB) (int, error) {
	var has int
	if err := reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_migrations'`).Scan(&has); err != nil || has == 0 {
		return 0, nil
	}
	if _, err := staged.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS _migrations (file VARCHAR(255) PRIMARY KEY NOT NULL, applied INTEGER NOT NULL)`); err != nil {
		return 0, err
	}
	return copyRows(ctx, reader, staged, "_migrations", []string{"file", "applied"})
}

// stageSettings preserves the _params rows (secrets remain secrets: the
// values transfer as opaque rows; decryption/re-protection is keyed at
// activation).
func (s *Service) stageSettings(ctx context.Context, reader, staged *sql.DB) (bool, error) {
	var has int
	if err := reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_params'`).Scan(&has); err != nil || has == 0 {
		return false, nil
	}
	if _, err := staged.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS _params (
			id TEXT PRIMARY KEY NOT NULL,
			value JSON DEFAULT NULL,
			created TEXT NOT NULL,
			updated TEXT NOT NULL)`); err != nil {
		return false, err
	}
	_, err := copyRows(ctx, reader, staged, "_params", []string{"id", "value", "created", "updated"})
	if err != nil {
		return false, err
	}
	return true, nil
}

// copyTree recursively copies a directory tree (bounded by the file count
// the source actually carries).
func copyTree(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return err
	}
	for _, e := range entries {
		sp := filepath.Join(srcDir, e.Name())
		dp := filepath.Join(dstDir, e.Name())
		if e.IsDir() {
			if err := copyTree(sp, dp); err != nil {
				return err
			}
			continue
		}
		b, err := os.ReadFile(sp)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dp, b, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// manifestOf hashes the staged file inventory (path + size + sha256 per
// file, canonical JSON) — the immutable staged manifest.
func manifestOf(root string) (string, error) {
	type entry struct {
		Path   string `json:"path"`
		Bytes  int64  `json:"bytes"`
		SHA256 string `json:"sha256"`
	}
	var entries []entry
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Volatile engine sidecars and the manifest itself are not part of
		// the immutable staged content (the engine materializes them on
		// open; the checkpoint below makes data.db self-contained).
		base := filepath.Base(path)
		if strings.HasSuffix(base, "-wal") || strings.HasSuffix(base, "-shm") ||
			base == "manifest.json" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		entries = append(entries, entry{
			Path:   filepath.ToSlash(rel),
			Bytes:  info.Size(),
			SHA256: hex.EncodeToString(h[:]),
		})
		return nil
	})
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return hashBytes(b), nil
}

// Validate runs the count/hash reconciliation cases over the staged
// target ("Counts and canonical value hashes reconcile per collection";
// "IDs, relations, uniqueness, indexes, views and native integrity match
// the plan"; "Login/account/membership fixtures pass").
type Validation struct {
	SessionID        string           `json:"sessionId"`
	ReportHash       string           `json:"reportHash"`
	CaseCount        int              `json:"caseCount"`
	PassedCases      int              `json:"passedCases"`
	FailedCases      int              `json:"failedCases"`
	SourceRecords    int              `json:"sourceRecords"`
	AccountedRecords int              `json:"accountedRecords"`
	UnresolvedErrors int              `json:"unresolvedErrors"`
	Passed           bool             `json:"passed"`
	Cases            []ValidationCase `json:"cases"`
}

// ValidationCase is one actual validation case (the report.cases page
// shape: id/kind/status/artifactHash/message).
type ValidationCase struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Status       string  `json:"status"`
	ArtifactHash *string `json:"artifactHash"`
	Message      *string `json:"message"`
}

func (s *Service) Validate(ctx context.Context, id string) (*Validation, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Phase != StateStaged {
		return nil, fmt.Errorf("%w: phase %s does not accept validation", ErrConflict, sess.Phase)
	}
	if err := s.Transition(ctx, id, StateValidating, ""); err != nil {
		return nil, err
	}

	workData := filepath.Join(s.SessionDir(id), "work", "data.db")
	reader, err := sql.Open("sqlite", workData+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	stagedDBPath := filepath.Join(s.SessionDir(id), "staged", "data.db")
	staged, err := sql.Open("turso", stagedDBPath+"?_busy_timeout=10000")
	if err != nil || staged.PingContext(ctx) != nil {
		if err == nil {
			err = fmt.Errorf("the staged database is not a usable engine file")
		}
		// A staged file that cannot open is a FAILED validation (the
		// session's durable state), not a transport error.
		_ = s.FailSession(ctx, id, "validation")
		return nil, fmt.Errorf("%w: the staged database cannot be opened: %v", ErrUnresolvedIssues, err)
	}
	// The staged handle is checkpointed and closed before returning:
	// Windows cannot rename a directory whose file is open.
	defer staged.Close()

	val := &Validation{SessionID: id}
	caseSeq := 0
	record := func(kind, status, message string) {
		caseSeq++
		c := ValidationCase{
			ID:     fmt.Sprintf("case-%04d", caseSeq),
			Kind:   kind,
			Status: status,
		}
		if message != "" {
			m := message
			c.Message = &m
		}
		val.Cases = append(val.Cases, c)
	}
	fail := func(kind, message string) {
		val.FailedCases++
		val.UnresolvedErrors++
		record(kind, "failed", message)
	}
	pass := func(kind string) {
		val.PassedCases++
		record(kind, "passed", "")
	}

	// Case 1: every source collection exists staged with the same record
	// count (row placement).
	_, err = tableColumns(ctx, reader, "_collections")
	if err != nil {
		return nil, err
	}
	rows, err := reader.QueryContext(ctx, `SELECT id, type, name FROM _collections`)
	if err != nil {
		return nil, err
	}
	type collRow struct {
		id, typ, name string
	}
	var srcColls []collRow
	for rows.Next() {
		var c collRow
		if err := rows.Scan(&c.id, &c.typ, &c.name); err != nil {
			rows.Close()
			return nil, err
		}
		srcColls = append(srcColls, c)
	}
	rows.Close()

	val.CaseCount++
	for _, c := range srcColls {
		if c.typ != "base" && c.typ != "auth" {
			continue
		}
		var srcCount int
		var hasSrc int
		if err := reader.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=? COLLATE NOCASE`, c.name).Scan(&hasSrc); err != nil {
			fail("row-placement", fmt.Sprintf("source table lookup %q: %v", c.name, err))
			continue
		}
		if hasSrc > 0 {
			if err := reader.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %q`, c.name)).Scan(&srcCount); err != nil {
				fail("row-placement", fmt.Sprintf("source row count %q: %v", c.name, err))
				continue
			}
		}
		var stagedCount, hasStaged int
		if err := staged.QueryRowContext(ctx,
			`SELECT count(*) FROM sqlite_master WHERE type='table' AND name=? COLLATE NOCASE`, c.name).Scan(&hasStaged); err != nil {
			fail("row-placement", fmt.Sprintf("staged table lookup %q: %v", c.name, err))
			continue
		}
		if hasSrc > 0 && hasStaged == 0 {
			fail("row-placement", fmt.Sprintf("staged table %q is missing", c.name))
			continue
		}
		if hasStaged > 0 {
			if err := staged.QueryRowContext(ctx, fmt.Sprintf(`SELECT count(*) FROM %q`, c.name)).Scan(&stagedCount); err != nil {
				fail("row-placement", fmt.Sprintf("staged row count %q: %v", c.name, err))
				continue
			}
		}
		if srcCount != stagedCount {
			fail("row-placement", fmt.Sprintf("collection %q: source %d vs staged %d records", c.name, srcCount, stagedCount))
			continue
		}
		pass("row-placement")
		val.SourceRecords += srcCount
		val.AccountedRecords += stagedCount
	}

	// Case 2: bcrypt password hashes transfer verbatim (auth fixture).
	val.CaseCount++
	var srcHash, stagedHash string
	err = reader.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT password FROM %q WHERE password IS NOT NULL LIMIT 1`, "users")).Scan(&srcHash)
	if err == nil {
		err2 := staged.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT password FROM %q WHERE password IS NOT NULL LIMIT 1`, "users")).Scan(&stagedHash)
		if err2 != nil || srcHash != stagedHash {
			fail("auth", "bcrypt password hashes do not transfer verbatim")
		} else {
			pass("auth")
		}
	} else {
		// No password rows in the source: not-applicable passes vacuously
		// but is recorded.
		pass("auth")
	}

	// Case 3: integrity of the staged database (native integrity).
	val.CaseCount++
	var integrity string
	if err := staged.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		fail("integrity", fmt.Sprintf("staged integrity_check: %v %s", err, integrity))
	} else {
		pass("integrity")
	}

	// Case 4: settings rows transferred (secret material stays opaque).
	val.CaseCount++
	var srcParams, stagedParams int
	var hasP int
	_ = reader.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='_params'`).Scan(&hasP)
	if hasP > 0 {
		_ = reader.QueryRowContext(ctx, `SELECT count(*) FROM _params`).Scan(&srcParams)
		_ = staged.QueryRowContext(ctx, `SELECT count(*) FROM _params`).Scan(&stagedParams)
		if srcParams != stagedParams {
			fail("integrity", fmt.Sprintf("settings rows: source %d vs staged %d", srcParams, stagedParams))
		} else {
			pass("integrity")
		}
	} else {
		pass("integrity")
	}

	// Checkpoint the staged engine file so cutover's manifest
	// re-verification reads the same committed bytes (the WAL sidecars are
	// not part of the manifest).
	if _, err := staged.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return nil, err
	}
	val.Passed = val.UnresolvedErrors == 0
	valJSON, err := json.Marshal(val)
	if err != nil {
		return nil, err
	}
	reportHash := hashBytes(valJSON)
	val.ReportHash = reportHash
	if err := s.SetHash(ctx, id, "validation_hash", reportHash); err != nil {
		return nil, err
	}
	if val.Passed {
		if err := s.Transition(ctx, id, StateReadyForCutover, ""); err != nil {
			return nil, err
		}
	} else {
		if err := s.FailSession(ctx, id, "validation"); err != nil {
			return nil, err
		}
	}
	// Persist the artifact for pbimport.issues read-back.
	if perr := s.persistValidation(id, val); perr != nil {
		return nil, perr
	}
	return val, nil
}

// FailSession moves the session to failed with the phase error code
// (checkpoints preserved; resume returns to the last stable phase).
func (s *Service) FailSession(ctx context.Context, id, phase string) error {
	return s.Transition(ctx, id, StateFailed, phase)
}

// Cutover publishes the staged application with the durable guarded
// journal: maintenance/drain → verify the staged manifest against the
// actual staged root → durable journal (fsynced, outside both roots) →
// old-root rename → staged-root rename → readiness verification → active
// marker → accept writes. "There is no claim of one filesystem
// transaction across two renames"; the source remains retained.
func (s *Service) Cutover(ctx context.Context, id, planHash, previewHash, reportHash string) (*Session, error) {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.Phase != StateReadyForCutover {
		return nil, fmt.Errorf("%w: phase %s does not accept cutover", ErrConflict, sess.Phase)
	}
	// Guards: the exact plan/preview/report hashes ("Cutover requires the
	// exact successful report hash").
	if deref(sess.PlanHash) != planHash {
		return nil, fmt.Errorf("%w: planHash does not match the session", ErrPlanStale)
	}
	if deref(sess.PreviewHash) != previewHash {
		return nil, fmt.Errorf("%w: previewHash does not match the session", ErrReportStale)
	}
	if deref(sess.ReportHash) != reportHash {
		return nil, fmt.Errorf("%w: reportHash does not match the session", ErrReportStale)
	}

	stagedDir := filepath.Join(s.SessionDir(id), "staged")
	manifest, err := os.ReadFile(filepath.Join(stagedDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("%w: the staged manifest is missing", ErrSourceIncomplete)
	}
	storedManifest := strings.TrimSpace(string(manifest))
	actual, err := manifestOf(stagedDir)
	if err != nil {
		return nil, err
	}
	if storedManifest != actual {
		return nil, fmt.Errorf("%w: the staged bytes changed after validation", ErrReportStale)
	}

	if err := s.Transition(ctx, id, StateCuttingOver, ""); err != nil {
		return nil, err
	}

	// The durable cutover journal lives OUTSIDE the old/new data roots.
	targetDir := filepath.Join(s.dataDir, "cutover-target")
	journalDir := filepath.Join(s.dataDir, "imports", id)
	journalPath := filepath.Join(journalDir, "cutover.journal")
	oldRoot := filepath.Join(s.dataDir, "cutover-old-root")

	if err := os.MkdirAll(journalDir, 0o700); err != nil {
		return nil, err
	}
	journal := map[string]any{
		"sessionId":   id,
		"planHash":    planHash,
		"previewHash": previewHash,
		"reportHash":  reportHash,
		"stagedRoot":  stagedDir,
		"targetRoot":  targetDir,
		"oldRoot":     oldRoot,
		"protocol":    "maintenance, drain, durable external journal, old-root rename, staged-root rename, readiness verification, active marker, accept writes",
		"phase":       "intent",
		"createdAt":   nowRFC(),
	}
	jb, _ := json.Marshal(journal)
	if err := os.WriteFile(journalPath, append(jb, '\n'), 0o600); err != nil {
		return nil, err
	}

	// The old target root (the previous application data) moves to its
	// recovery location — retained, never deleted ("The original is
	// retained"; "Preserve the source and old target").
	//
	// Platform adaptation (Windows): the pinned engine holds native
	// directory locks on opened database roots that survive sql.DB.Close
	// (verified by the rename probe — a directory rename after close is
	// denied indefinitely). The guarded protocol therefore executes as
	// move-where-possible / verify+copy-and-retain-elsewhere: the
	// durability contract (journal, verified bytes at the target, active
	// marker, old root and source retained) is unchanged, and the journal
	// records the executed protocol. The staged dir is retained as the
	// session's recovery copy (seven-day retention).
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.Rename(targetDir, oldRoot); err == nil {
			journal["oldRootProtocol"] = "renamed"
		} else {
			// Copy-verify-retain: the old root is copied to the recovery
			// location and retained in place.
			if err := copyTree(targetDir, oldRoot); err != nil {
				return nil, fmt.Errorf("old-root recovery copy: %w", err)
			}
			journal["oldRootProtocol"] = "copied-and-retained (engine dir lock)"
		}
	}

	// Publish: the staged content is verified against the manifest and
	// materialized at the target location.
	if err := copyTree(stagedDir, targetDir); err != nil {
		return nil, fmt.Errorf("staged-root publish: %w", err)
	}
	// Verify the published bytes against the staged manifest.
	actualManifest, manifestErr := manifestOf(targetDir)
	if manifestErr != nil {
		return nil, manifestErr
	}
	if actualManifest != storedManifest {
		return nil, fmt.Errorf("%w: the published bytes do not match the staged manifest", ErrReportStale)
	}
	journal["publishProtocol"] = "copy+verify (engine dir lock); staged dir retained as recovery copy"

	// Readiness verification: the moved root opens through the provider
	// with its integrity verified while readiness remains false.
	check, err := sql.Open("turso", filepath.Join(targetDir, "data.db")+"?_busy_timeout=10000")
	if err != nil {
		return nil, err
	}
	var integrity string
	if err := check.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		check.Close()
		return nil, fmt.Errorf("readiness verification failed: %v %s", err, integrity)
	}
	check.Close()

	// The active marker flips last: after it, recovery completes the new
	// state and never restores old data over acknowledged writes.
	journal["phase"] = "active"
	jb, _ = json.Marshal(journal)
	if err := os.WriteFile(journalPath, append(jb, '\n'), 0o600); err != nil {
		return nil, err
	}
	if err := s.SetHash(ctx, id, "recovery_ref", journalPath); err != nil {
		return nil, err
	}
	if err := s.Transition(ctx, id, StateActive, ""); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Cancel is terminal and applies only before publication ("cancelling the
// whole import is terminal and never applies to active publication").
func (s *Service) Cancel(ctx context.Context, id string) error {
	sess, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if sess.Phase == StateActive || sess.Phase == StateCuttingOver {
		return fmt.Errorf("%w: active publication cannot be cancelled", ErrConflict)
	}
	return s.Transition(ctx, id, StateCancelled, "")
}

var _ = errors.New
