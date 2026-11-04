package livequery

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// sameShape compares the evaluated result columns against the published
// shape (exact order and names).
func sameShape(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Snapshot evaluation and keyed ordered diffing.
//
// The snapshot runs the published plan's read-only SQL inside the CALLER's
// read transaction (the transaction also captured the change-journal head
// before the first row was read — the subscribe/query race closure).
// The diff is keyed by the plan's result key, ordered (deletes first, then
// inserts and updates in the new result order) and atomic per frame: a
// position change folds into an update because the wire event kinds are
// only snapshot, insert, update, delete, reset and error.
// ---------------------------------------------------------------------------

// querier is the minimal SQL read surface of *sql.DB, *sql.Tx and
// *sql.Conn (the snapshot runs over a read transaction's querier).
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// liveRow is one evaluated result row in result order.
type liveRow struct {
	Key      string
	Position int
	Row      map[string]any
	Hash     string
}

// patchOp is one keyed operation of an ordered patch (applied atomically:
// deletes first, then inserts and updates).
type patchOp struct {
	Op       string         `json:"op"`
	Key      string         `json:"key"`
	Position int            `json:"position"`
	Row      map[string]any `json:"row,omitempty"`
}

// readJournalHead reads the change journal's commit head through q. The
// call is the FIRST read of every snapshot transaction (capturing the
// consistent read boundary BEFORE the plan's rows are read).
func readJournalHead(ctx context.Context, q querier) (int64, error) {
	var head int64
	err := q.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head)
	if err == nil {
		return head, nil
	}
	// The journal may not exist yet on a target that never recorded a
	// change: install the head table (the same DDL the writer uses —
	// mirrored because the dependency direction is application →
	// livequery) and retry once.
	if db, ok := q.(interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}); ok {
		if _, derr := db.ExecContext(ctx, journalHeadDDL); derr == nil {
			err = q.QueryRowContext(ctx,
				`SELECT COALESCE(MAX(commit_seq), 0) FROM _mb_changes`).Scan(&head)
		}
	}
	return head, err
}

const journalHeadDDL = `CREATE TABLE IF NOT EXISTS _mb_changes (
 commit_seq INTEGER PRIMARY KEY,
 transaction_id TEXT NOT NULL,
 generation INTEGER NOT NULL,
 schema_revision INTEGER NOT NULL,
 source TEXT NOT NULL CHECK(source IN ('record','admin-sql','pg','native-sync','migration')),
 timestamp TEXT NOT NULL
)`

// bindParams maps the request's positional parameters onto the plan's
// declared parameters: extra request parameters are refused; a missing
// entry binds its declared default (or NULL). SUPPLIED values (from the
// request or the declared default) are validated against the declared
// parameter type BEFORE subscription allocation ("Parameter and shape
// mismatches fail before subscription allocation"); values adapt to
// engine storage (bool → int; composites → JSON text).
func bindParams(plan *Plan, request []any) ([]any, error) {
	if len(request) > len(plan.Parameters) {
		return nil, fmt.Errorf("%w: the plan declares %d parameters; %d were sent",
			ErrInvalidRequest, len(plan.Parameters), len(request))
	}
	args := make([]any, 0, len(plan.Parameters))
	for i := range plan.Parameters {
		declared := plan.Parameters[i]
		supplied := false
		value := any(nil)
		switch {
		case i < len(request):
			value, supplied = request[i], true
		case declared != nil:
			if d, ok := declared["default"]; ok {
				value, supplied = d, true
			}
		}
		if supplied {
			if err := checkParamValue(declared, value); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
			}
		}
		args = append(args, adaptValue(value))
	}
	return args, nil
}

// paramTypes are the declared parameter types a plan may declare
// ("text" is the established wire alias of "string").
var paramTypes = map[string]func(any) bool{
	"string":  stringParam,
	"text":    stringParam,
	"integer": func(v any) bool { _, ok := v.(int); ok = ok || isIntegralFloat(v); return ok },
	"real":    func(v any) bool { _, ok := v.(float64); ok = ok || isIntegralFloat(v); return ok },
	"boolean": func(v any) bool { _, ok := v.(bool); return ok },
	"json":    func(v any) bool { return true },
}

func stringParam(v any) bool {
	_, ok := v.(string)
	return ok
}

// isIntegralFloat reports whether a decoded JSON number is integral.
func isIntegralFloat(v any) bool {
	f, ok := v.(float64)
	return ok && f == float64(int64(f))
}

// checkParamValue validates one supplied value against the declared
// parameter type (unknown declarations fail closed).
func checkParamValue(declared map[string]any, value any) error {
	if declared == nil {
		return nil
	}
	t, _ := declared["type"].(string)
	if t == "" {
		t = "json"
	}
	check, ok := paramTypes[t]
	if !ok {
		return fmt.Errorf("the parameter declares an unknown type %q", t)
	}
	if value == nil {
		return nil // NULL binds regardless of type
	}
	if !check(value) {
		return fmt.Errorf("the parameter value %v does not match the declared type %q", value, t)
	}
	return nil
}

// adaptValue converts a decoded JSON value into an engine storage value.
func adaptValue(v any) any {
	switch t := v.(type) {
	case nil, string, int, int64, float64:
		return t
	case bool:
		if t {
			return 1
		}
		return 0
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// normalizeValue adapts a scanned engine value to a JSON-ready value.
func normalizeValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return v
	}
}

// planEvalQuery composes the evaluation query: the published plan SQL
// wrapped, when the plan carries a single-collection policy and a current
// actor predicate exists, by the EXISTS correlation that applies the
// ACTOR'S CURRENT collection rules to every result row (D-14 policy
// injection). The predicate is compiled by the resolver through the
// parsed filter machinery — parameterized, never interpolated.
func planEvalQuery(plan *Plan, args []any, predicate string, predArgs []any) (string, []any) {
	if predicate == "" || len(plan.Policy) == 0 {
		return plan.SQL, args
	}
	coll := plan.Policy[0].Collection
	injected := fmt.Sprintf(
		`SELECT * FROM (%s) AS _mb_plan WHERE EXISTS (SELECT 1 FROM %s WHERE %s AND %s."id" = _mb_plan.%s)`,
		plan.SQL, quoteIdent(coll), predicate, quoteIdent(coll), quoteIdent(plan.ResultKey))
	combined := make([]any, 0, len(args)+len(predArgs))
	combined = append(combined, args...)
	combined = append(combined, predArgs...)
	return injected, combined
}

// quoteIdent renders a SQL identifier (inner quotes are doubled).
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// evalSnapshot runs the plan (with the actor's injected policy predicate)
// inside the caller's read scope and returns the bounded, keyed rows in
// result order. The row and byte bounds are ERRORS, never silent
// truncation; the result key must be non-null and unique in every row,
// and the result columns must match the published shape (a drift is a
// policy error, never a silent re-shape).
func evalSnapshot(ctx context.Context, q querier, plan *Plan, args []any, predicate string, predArgs []any) ([]liveRow, error) {
	query, evalArgs := planEvalQuery(plan, args, predicate, predArgs)
	rows, err := q.QueryContext(ctx, query, evalArgs...)
	if err != nil {
		return nil, fmt.Errorf("the plan evaluation failed: %w", err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	if len(plan.Shape) > 0 && !sameShape(cols, plan.Shape) {
		return nil, fmt.Errorf("%w: the plan result shape no longer matches the published policy", ErrShapeMismatch)
	}
	keyColumn := -1
	for i, c := range cols {
		if c == plan.ResultKey {
			keyColumn = i
			break
		}
	}
	if keyColumn < 0 {
		return nil, fmt.Errorf("%w: the result key %q is not among the plan's result columns",
			ErrResultKey, plan.ResultKey)
	}
	out := make([]liveRow, 0, 16)
	seen := make(map[string]bool, 16)
	totalBytes := 0
	maxBytes := plan.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	for rows.Next() {
		if len(out) >= plan.MaxRows {
			return nil, fmt.Errorf("%w: the plan result exceeds maxRows %d", ErrExhausted, plan.MaxRows)
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = normalizeValue(vals[i])
		}
		rawKey := vals[keyColumn]
		if rawKey == nil {
			return nil, fmt.Errorf("%w: the result key %q is null in the plan result", ErrResultKey, plan.ResultKey)
		}
		key := ""
		switch k := rawKey.(type) {
		case string:
			key = k
		case []byte:
			key = string(k)
		default:
			key = fmt.Sprintf("%v", rawKey)
		}
		if key == "" {
			return nil, fmt.Errorf("%w: the result key %q is empty in the plan result", ErrResultKey, plan.ResultKey)
		}
		if seen[key] {
			return nil, fmt.Errorf("%w: the result key %q is not unique in the plan result", ErrResultKey, key)
		}
		seen[key] = true
		encoded, hash := rowFingerprint(row)
		totalBytes += len(encoded)
		if totalBytes > maxBytes {
			return nil, fmt.Errorf("%w: the plan result exceeds maxBytes %d", ErrExhausted, maxBytes)
		}
		out = append(out, liveRow{Key: key, Position: len(out), Row: row, Hash: hash})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// rowFingerprint encodes one row canonically (map keys are sorted by
// encoding/json) and returns its encoding plus digest.
func rowFingerprint(row map[string]any) (encoded []byte, hash string) {
	b, err := json.Marshal(row)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", row))
	}
	sum := sha256.Sum256(b)
	return b, hex.EncodeToString(sum[:])
}

// diffRows computes the keyed ordered patch between two result states:
// deletes first (in the old order), then inserts and updates in the NEW
// result order. A content change or a pure position change folds into an
// update carrying the row at its new position.
func diffRows(oldRows, newRows []liveRow) []patchOp {
	oldByKey := make(map[string]liveRow, len(oldRows))
	for _, r := range oldRows {
		oldByKey[r.Key] = r
	}
	newByKey := make(map[string]liveRow, len(newRows))
	for _, r := range newRows {
		newByKey[r.Key] = r
	}
	ops := []patchOp{}
	for _, r := range oldRows {
		if _, ok := newByKey[r.Key]; !ok {
			ops = append(ops, patchOp{Op: KindDelete, Key: r.Key, Position: r.Position})
		}
	}
	for _, r := range newRows {
		o, ok := oldByKey[r.Key]
		if !ok {
			ops = append(ops, patchOp{Op: KindInsert, Key: r.Key, Position: r.Position, Row: r.Row})
			continue
		}
		if o.Hash != r.Hash || o.Position != r.Position {
			ops = append(ops, patchOp{Op: KindUpdate, Key: r.Key, Position: r.Position, Row: r.Row})
		}
	}
	return ops
}
