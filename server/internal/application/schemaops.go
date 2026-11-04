package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mirabase/core"
	"mirabase/internal/search"
)

// ---------------------------------------------------------------------------
// Per-target schema plans (schema.plan / schema.apply; released slice:
// "Inherited schema plus vector/FTS" — collection.create/update/delete and
// btree/fts index operations against one ready target database; type/view/
// sequence operations are the roadmap surface and are rejected as blockers,
// never silently skipped).
// ---------------------------------------------------------------------------

// schemaPlansTableDDL is the durable per-target plan store (target-local:
// the plan lives beside the schema it mutates).
const schemaPlansTableDDL = `CREATE TABLE IF NOT EXISTS _mb_schema_plans (
 id TEXT PRIMARY KEY,
 expected_revision INTEGER NOT NULL CHECK(expected_revision > 0),
 plan_hash TEXT NOT NULL,
 operations_json TEXT NOT NULL CHECK(json_valid(operations_json)),
 destructive_json TEXT NOT NULL CHECK(json_valid(destructive_json)),
 blockers_json TEXT NOT NULL CHECK(json_valid(blockers_json)),
 state TEXT NOT NULL CHECK(state IN ('validated','applied','superseded')),
 created TEXT NOT NULL,
 applied_at TEXT
)`

// SchemaOp is one wire schema operation.
type SchemaOp struct {
	Kind       string          `json:"kind"`
	ObjectID   string          `json:"objectId"`
	Definition json.RawMessage `json:"definition,omitempty"`
}

// SchemaPlanBlocker is one plan validation blocker.
type SchemaPlanBlocker struct {
	ObjectID string `json:"objectId"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// TargetSchemaPlan is one validated (or applied) per-target plan.
type TargetSchemaPlan struct {
	ID                 string              `json:"id"`
	DatabaseID         string              `json:"databaseId"`
	ExpectedRevision   int64               `json:"expectedSchemaRevision"`
	PlanHash           string              `json:"planHash"`
	Operations         []SchemaOp          `json:"operations"`
	DestructiveEffects []string            `json:"destructiveEffects"`
	Blockers           []SchemaPlanBlocker `json:"blockers"`
	State              string              `json:"state"`
	Created            string              `json:"created"`
	AppliedAt          *string             `json:"appliedAt"`
}

// collectionDefinitionWire is the accepted CollectionDefinition subset.
type collectionDefinitionWire struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Fields []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"fields"`
	Indexes    []string       `json:"indexes"`
	ListRule   *string        `json:"listRule"`
	ViewRule   *string        `json:"viewRule"`
	CreateRule *string        `json:"createRule"`
	UpdateRule *string        `json:"updateRule"`
	DeleteRule *string        `json:"deleteRule"`
	Options    map[string]any `json:"options"`
}

// indexDefinitionWire is the accepted IndexDefinition subset.
type indexDefinitionWire struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"` // btree|fts
	Unique bool   `json:"unique"`
	Fields []struct {
		Expression string `json:"expression"`
		Direction  string `json:"direction"` // asc|desc
	} `json:"fields"`
	Options map[string]any `json:"options"`
}

// ErrInvalidSchemaOp reports an invalid or unsupported schema operation.
var ErrInvalidSchemaOp = errors.New("invalid schema operation")

// targetSchemaPlansEnsure installs the plan store on the target database.
func targetSchemaPlansEnsure(ctx context.Context, target *sql.DB) error {
	if _, err := target.ExecContext(ctx, schemaPlansTableDDL); err != nil {
		return fmt.Errorf("failed to install target schema plans: %w", err)
	}
	return nil
}

// targetTables introspects the target's user tables (PB record tables use
// the collection id; internal rows are the _mb_* store).
func targetTables(ctx context.Context, target *sql.DB) (map[string]bool, error) {
	rows, err := target.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
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

// targetColumns returns the column names of one target table.
func targetColumns(ctx context.Context, target *sql.DB, table string) (map[string]bool, error) {
	rows, err := target.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, quoteIdentGo(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

// quoteIdentGo quotes one SQL identifier defensively.
func quoteIdentGo(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// coreFieldColumnType maps a wire field type onto the registered field
// column type ("A native SQL type without these adapters is not a
// completed field feature" — the adapter here IS the core field or the R1
// engine-kind adapter).
func coreFieldColumnType(app core.App, fieldID, fieldType string) (string, bool) {
	if adapter := R1FieldAdapter(fieldType); adapter != nil {
		return adapter.ColumnType, true
	}
	switch fieldType {
	case "text":
		return (&core.TextField{Id: fieldID}).ColumnType(app), true
	case "number":
		return (&core.NumberField{Id: fieldID}).ColumnType(app), true
	case "bool":
		return (&core.BoolField{Id: fieldID}).ColumnType(app), true
	case "email":
		return (&core.EmailField{Id: fieldID}).ColumnType(app), true
	case "url":
		return (&core.URLField{Id: fieldID}).ColumnType(app), true
	case "editor":
		return (&core.EditorField{Id: fieldID}).ColumnType(app), true
	case "date":
		return (&core.DateField{Id: fieldID}).ColumnType(app), true
	case "autodate":
		return (&core.AutodateField{Id: fieldID}).ColumnType(app), true
	case "select":
		return (&core.SelectField{Id: fieldID}).ColumnType(app), true
	case "json":
		return (&core.JSONField{Id: fieldID}).ColumnType(app), true
	case "relation":
		return (&core.RelationField{Id: fieldID}).ColumnType(app), true
	case "file":
		return (&core.FileField{Id: fieldID}).ColumnType(app), true
	case "password":
		return (&core.PasswordField{Id: fieldID}).ColumnType(app), true
	case "geoPoint":
		return (&core.GeoPointField{Id: fieldID}).ColumnType(app), true
	default:
		// vector, int64, decimal, uuid, array, binary, typed-*, network-address,
		// struct, union, domain, custom: the native adapters are not completed
		// field features in this release — never a silent pass-through.
		return "", false
	}
}

// isSafeName validates one identifier (safe characters).
func isSafeName(name string) bool {
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

// isSafeCollectionID validates the collection id used as the target table
// name (PB identifiers; the id never becomes a filesystem component).
func isSafeCollectionID(id string) bool {
	if len(id) < 3 || len(id) > 40 {
		return false
	}
	for _, r := range id {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

// PlanTargetSchema validates the operations against the target's live
// schema ("Validate/prepare plan without modifying application data"),
// hashes and stores the plan durably. Plans with blockers are stored
// state=validated but are refused at apply.
func (rt *Runtime) PlanTargetSchema(ctx context.Context, app core.App, target *sql.DB, databaseID string, expectedRevision int64, ops []SchemaOp) (*TargetSchemaPlan, error) {
	if err := targetSchemaPlansEnsure(ctx, target); err != nil {
		return nil, err
	}
	tables, err := targetTables(ctx, target)
	if err != nil {
		return nil, err
	}
	blockers := []SchemaPlanBlocker{}
	destructive := []string{}

	block := func(objectID, code, msg string) {
		blockers = append(blockers, SchemaPlanBlocker{ObjectID: objectID, Code: code, Message: msg})
	}

	// Track simulated state so multi-op plans see their own effects.
	simulated := map[string]*collectionDefinitionWire{}
	simulatedNames := map[string]string{} // collection id -> physical table name
	simulatedDropped := map[string]bool{}
	simIndexes := map[string]bool{}

	// resolveTableName maps a wire object reference onto the physical
	// table name (the simulated plan state first, then the live tables —
	// the wire object may carry the collection name directly).
	resolveTableName := func(ref string) string {
		if simulatedNames[ref] != "" {
			return simulatedNames[ref]
		}
		if simulated[ref] != nil {
			return simulated[ref].Name
		}
		if tables[ref] {
			return ref
		}
		return ref
	}

	// Known table + column snapshot for validation.
	type tableInfo struct{ columns map[string]bool }
	snap := map[string]*tableInfo{}
	for t := range tables {
		cols, err := targetColumns(ctx, target, t)
		if err != nil {
			return nil, err
		}
		snap[t] = &tableInfo{columns: cols}
	}

	for i, op := range ops {
		switch op.Kind {
		case "collection.create":
			var def collectionDefinitionWire
			if err := json.Unmarshal(op.Definition, &def); err != nil {
				block(op.ObjectID, "invalid_definition", "collection.create definition is not a valid CollectionDefinition")
				continue
			}
			if !isSafeCollectionID(def.ID) || !isSafeCollectionID(def.Name) {
				block(op.ObjectID, "invalid_object_id", "collection ids and names must be 3..40 safe identifier characters")
				continue
			}
			if tables[def.Name] || tables[def.ID] || simulated[def.ID] != nil || simulatedDropped[def.ID] {
				block(op.ObjectID, "already_exists", fmt.Sprintf("table %q already exists in the target schema", def.Name))
				continue
			}
			if def.Type != "base" {
				block(op.ObjectID, "type_not_released", "target schema slice supports base collections only (auth/view are inherited-surface)")
				continue
			}
			cols := map[string]bool{}
			badField := false
			for _, f := range def.Fields {
				if len(f.Name) < 1 || len(f.Name) > 40 || !isSafeName(f.Name) {
					block(op.ObjectID, "invalid_field_name", fmt.Sprintf("field %q is not a safe identifier", f.Name))
					badField = true
					continue
				}
				if _, ok := coreFieldColumnType(app, f.ID, f.Type); !ok {
					block(op.ObjectID, "field_type_not_released", fmt.Sprintf("field type %q has no completed field adapter in this release", f.Type))
					badField = true
					continue
				}
				cols[f.Name] = true
			}
			if badField {
				continue
			}
			simulated[def.ID] = &collectionDefinitionWire{ID: def.ID, Name: def.Name, Type: def.Type, Fields: def.Fields}
			simulatedNames[def.ID] = def.Name
			snap[def.Name] = &tableInfo{columns: cols}
		case "collection.update":
			var def collectionDefinitionWire
			if err := json.Unmarshal(op.Definition, &def); err != nil {
				block(op.ObjectID, "invalid_definition", "collection.update definition is not a valid CollectionDefinition")
				continue
			}
			updateTable := def.Name
			if simulatedNames[def.ID] != "" {
				updateTable = simulatedNames[def.ID]
			}
			info, ok := snap[updateTable]
			if !ok || simulatedDropped[def.ID] {
				block(op.ObjectID, "not_found", fmt.Sprintf("table %q does not exist in the target schema", def.Name))
				continue
			}
			cols := map[string]bool{}
			badField := false
			for _, f := range def.Fields {
				if len(f.Name) < 1 || len(f.Name) > 40 || !isSafeName(f.Name) {
					block(op.ObjectID, "invalid_field_name", fmt.Sprintf("field %q is not a safe identifier", f.Name))
					badField = true
					continue
				}
				if _, ok := coreFieldColumnType(app, f.ID, f.Type); !ok {
					block(op.ObjectID, "field_type_not_released", fmt.Sprintf("field type %q has no completed field adapter in this release", f.Type))
					badField = true
					continue
				}
				cols[f.Name] = true
			}
			if badField {
				continue
			}
			for existing := range info.columns {
				if !cols[existing] {
					destructive = append(destructive, fmt.Sprintf("column:%s.%s", updateTable, existing))
				}
			}
			info.columns = cols
		case "collection.delete":
			delTable := resolveTableName(op.ObjectID)
			if _, ok := snap[delTable]; !ok || simulatedDropped[delTable] {
				block(op.ObjectID, "not_found", fmt.Sprintf("table %q does not exist in the target schema", delTable))
				continue
			}
			destructive = append(destructive, "table:"+delTable)
			simulatedDropped[delTable] = true
			delete(snap, delTable)
		case "index.create":
			var def indexDefinitionWire
			if err := json.Unmarshal(op.Definition, &def); err != nil {
				block(op.ObjectID, "invalid_definition", "index.create definition is not a valid IndexDefinition")
				continue
			}
			if !isSafeCollectionID(def.Name) {
				block(op.ObjectID, "invalid_object_id", "index names must be safe identifier characters")
				continue
			}
			if simIndexes[def.Name] {
				block(op.ObjectID, "already_exists", fmt.Sprintf("index %q already exists in the plan", def.Name))
				continue
			}
			switch def.Kind {
			case "btree", "":
				// The index target resolves through the plan state: the
				// object reference may be the collection id or the name.
				target := resolveTableName(op.ObjectID)
				info, ok := snap[target]
				if !ok || simulatedDropped[target] {
					block(op.ObjectID, "not_found", fmt.Sprintf("table %q does not exist in the target schema", target))
					continue
				}
				for _, f := range def.Fields {
					if !info.columns[f.Expression] {
						block(op.ObjectID, "unknown_column", fmt.Sprintf("index column %q is not a column of %q", f.Expression, target))
					}
				}
			case "fts":
				if len(def.Fields) == 0 {
					block(op.ObjectID, "invalid_definition", "the fts index requires at least one field")
					continue
				}
			default:
				block(op.ObjectID, "invalid_definition", fmt.Sprintf("index kind %q is not btree|fts", def.Kind))
				continue
			}
			simIndexes[def.Name] = true
		case "index.drop":
			// Dropping a nonexistent index is a no-op for SQLite DROP INDEX
			// IF EXISTS but is still recorded, never silently skipped.
		case "type.define", "type.drop", "view.define", "sequence.define":
			block(op.ObjectID, "not_in_released_slice", fmt.Sprintf("%s is roadmap surface (released slice: inherited schema plus vector/FTS)", op.Kind))
		default:
			block(op.ObjectID, "unknown_kind", fmt.Sprintf("unknown schema operation kind %q", op.Kind))
		}
		_ = i
	}

	opsJSON, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	blockersJSON, err := json.Marshal(blockers)
	if err != nil {
		return nil, err
	}
	destructiveJSON, err := json.Marshal(destructive)
	if err != nil {
		return nil, err
	}
	hash := targetSchemaPlanHash(databaseID, expectedRevision, ops)

	id := NewID("spl-")
	now := nowRFC3339()
	if _, err := target.ExecContext(ctx,
		`INSERT INTO _mb_schema_plans (id, expected_revision, plan_hash, operations_json, destructive_json, blockers_json, state, created)
		 VALUES (?, ?, ?, ?, ?, ?, 'validated', ?)`,
		id, expectedRevision, hash, string(opsJSON), string(destructiveJSON), string(blockersJSON), now); err != nil {
		return nil, err
	}
	return &TargetSchemaPlan{
		ID: id, DatabaseID: databaseID, ExpectedRevision: expectedRevision, PlanHash: hash,
		Operations: ops, DestructiveEffects: destructive, Blockers: blockers,
		State: "validated", Created: now,
	}, nil
}

// targetSchemaPlanHash computes the deterministic plan hash.
func targetSchemaPlanHash(databaseID string, expectedRevision int64, ops []SchemaOp) string {
	canonical := map[string]any{"databaseId": databaseID, "expectedSchemaRevision": expectedRevision, "operations": ops}
	b, err := json.Marshal(canonical)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// GetTargetSchemaPlan loads one stored target plan.
func (rt *Runtime) GetTargetSchemaPlan(ctx context.Context, target *sql.DB, id string) (*TargetSchemaPlan, error) {
	if err := targetSchemaPlansEnsure(ctx, target); err != nil {
		return nil, err
	}
	var p TargetSchemaPlan
	var opsJSON, destructiveJSON, blockersJSON string
	var appliedAt sql.NullString
	err := target.QueryRowContext(ctx,
		`SELECT id, expected_revision, plan_hash, operations_json, destructive_json, blockers_json, state, created, applied_at
		 FROM _mb_schema_plans WHERE id = ?`, id,
	).Scan(&p.ID, &p.ExpectedRevision, &p.PlanHash, &opsJSON, &destructiveJSON, &blockersJSON, &p.State, &p.Created, &appliedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(opsJSON), &p.Operations); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(destructiveJSON), &p.DestructiveEffects); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(blockersJSON), &p.Blockers); err != nil {
		return nil, err
	}
	if appliedAt.Valid {
		p.AppliedAt = &appliedAt.String
	}
	return &p, nil
}

// ApplyTargetSchema executes a validated plan's operations on the target in
// one transaction ("Apply verified same plan under schema lease") and
// returns the new schema revision. The caller has checked the hash/revision
// guards; blockers always refuse.
func (rt *Runtime) ApplyTargetSchema(ctx context.Context, app core.App, target *sql.DB, plan *TargetSchemaPlan) (newRevision int64, err error) {
	if len(plan.Blockers) > 0 {
		return 0, fmt.Errorf("%w: the plan carries %d unresolved blocker(s)", ErrInvalidSchemaOp, len(plan.Blockers))
	}
	tables, err := targetTables(ctx, target)
	if err != nil {
		return 0, err
	}

	tx, err := target.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, op := range plan.Operations {
		switch op.Kind {
		case "collection.create":
			var def collectionDefinitionWire
			if err := json.Unmarshal(op.Definition, &def); err != nil {
				return 0, fmt.Errorf("collection.create %s: %w", op.ObjectID, err)
			}
			cols := []string{}
			for _, f := range def.Fields {
				ct, ok := coreFieldColumnType(app, f.ID, f.Type)
				if !ok {
					return 0, fmt.Errorf("collection.create %s: field type %q not released", op.ObjectID, f.Type)
				}
				cols = append(cols, quoteIdentGo(f.Name)+" "+ct)
			}
			// PB system columns exist on every record table. The physical
			// table is named by the collection NAME (the PB convention the
			// record store, sync and reference surfaces all use).
			cols = append(cols,
				quoteIdentGo("id")+" TEXT PRIMARY KEY",
				quoteIdentGo("created")+" TEXT",
				quoteIdentGo("updated")+" TEXT")
			ddl := fmt.Sprintf(`CREATE TABLE %s (%s)`, quoteIdentGo(def.Name), strings.Join(cols, ", "))
			if _, err := tx.ExecContext(ctx, ddl); err != nil {
				return 0, fmt.Errorf("collection.create %s: %w", op.ObjectID, err)
			}
		case "collection.update":
			var def collectionDefinitionWire
			if err := json.Unmarshal(op.Definition, &def); err != nil {
				return 0, fmt.Errorf("collection.update %s: %w", op.ObjectID, err)
			}
			// The physical table is named by the collection NAME (the
			// plan's prior create determines it; otherwise the name is
			// the live table).
			updateTable := def.Name
			if prior := simulatedPlanLookup(plan, def.ID); prior != nil && prior.Name != "" {
				updateTable = prior.Name
			}
			existing, err := targetColumns(ctx, target, updateTable)
			if err != nil {
				return 0, err
			}
			desired := map[string]bool{}
			for _, f := range def.Fields {
				ct, ok := coreFieldColumnType(app, f.ID, f.Type)
				if !ok {
					return 0, fmt.Errorf("collection.update %s: field type %q not released", op.ObjectID, f.Type)
				}
				desired[f.Name] = true
				if !existing[f.Name] {
					if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`,
						quoteIdentGo(updateTable), quoteIdentGo(f.Name), ct)); err != nil {
						return 0, fmt.Errorf("collection.update %s: %w", op.ObjectID, err)
					}
				}
			}
			for col := range existing {
				if !desired[col] && !isSystemColumn(col) {
					// SQLite ALTER TABLE DROP COLUMN.
					if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN %s`,
						quoteIdentGo(updateTable), quoteIdentGo(col))); err != nil {
						return 0, fmt.Errorf("collection.update %s: drop %s: %w", op.ObjectID, col, err)
					}
				}
			}
		case "collection.delete":
			delTable := op.ObjectID
			if def := simulatedPlanLookup(plan, op.ObjectID); def != nil {
				delTable = def.Name
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, quoteIdentGo(delTable))); err != nil {
				return 0, fmt.Errorf("collection.delete %s: %w", op.ObjectID, err)
			}
		case "index.create":
			var def indexDefinitionWire
			if err := json.Unmarshal(op.Definition, &def); err != nil {
				return 0, fmt.Errorf("index.create %s: %w", op.ObjectID, err)
			}
			switch def.Kind {
			case "btree", "":
				fields := make([]string, 0, len(def.Fields))
				for _, f := range def.Fields {
					expr := f.Expression
					if f.Direction == "desc" {
						expr += " DESC"
					}
					fields = append(fields, quoteIdentGo(expr))
				}
				unique := ""
				if def.Unique {
					unique = "UNIQUE "
				}
				idxTable := op.ObjectID
				if def := simulatedPlanLookup(plan, op.ObjectID); def != nil {
					idxTable = def.Name
				}
				if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)`,
					unique, quoteIdentGo(def.Name), quoteIdentGo(idxTable), strings.Join(fields, ", "))); err != nil {
					return 0, fmt.Errorf("index.create %s: %w", op.ObjectID, err)
				}
			case "fts":
				names := make([]string, 0, len(def.Fields))
				for _, f := range def.Fields {
					names = append(names, f.Expression)
				}
				ftsTable := op.ObjectID
				if def := simulatedPlanLookup(plan, op.ObjectID); def != nil {
					ftsTable = def.Name
				}
				if _, err := search.EnsureFTSIndex(ctx, target, ftsTable, def.Name, names, "", nil); err != nil {
					return 0, fmt.Errorf("index.create fts %s: %w", op.ObjectID, err)
				}
			}
		case "index.drop":
			if tables[op.ObjectID] {
				if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, quoteIdentGo(op.ObjectID))); err != nil {
					return 0, fmt.Errorf("index.drop %s: %w", op.ObjectID, err)
				}
			} else {
				if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS %s`, quoteIdentGo(op.ObjectID))); err != nil {
					return 0, fmt.Errorf("index.drop %s: %w", op.ObjectID, err)
				}
			}
		default:
			return 0, fmt.Errorf("operation kind %q is not executable", op.Kind)
		}
	}

	// Target schema revision bump (the registry row remains the authority
	// and is bumped by the wire layer after a successful commit).
	newRevision = plan.ExpectedRevision + 1
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	now := nowRFC3339()
	if _, err := target.ExecContext(ctx,
		`UPDATE _mb_schema_plans SET state = 'applied', applied_at = ? WHERE id = ?`, now, plan.ID); err != nil {
		return 0, err
	}
	return newRevision, nil
}

// simulatedPlanLookup resolves a wire object reference to its planned
// definition (id -> definition; the physical table is the definition's
// Name).
func simulatedPlanLookup(plan *TargetSchemaPlan, objectRef string) *collectionDefinitionWire {
	for _, op := range plan.Operations {
		if op.Kind != "collection.create" && op.Kind != "collection.update" {
			continue
		}
		if op.ObjectID != objectRef {
			continue
		}
		var def collectionDefinitionWire
		if json.Unmarshal(op.Definition, &def) == nil {
			return &def
		}
	}
	return nil
}

// isSystemColumn reports whether a column is a PB system column.
func isSystemColumn(col string) bool {
	switch col {
	case "id", "created", "updated":
		return true
	}
	return false
}

var _ = time.Now // keep time linked for future retention-style helpers
