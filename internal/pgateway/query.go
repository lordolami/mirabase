package pgateway

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mirabase/internal/application"
)

// statementTimeout bounds every statement the gateway executes (the same
// 3-minute bound the inherited SQL console applies, apis/sql.go).
const statementTimeout = 3 * time.Minute

// pgStatement is one split statement: the original text (executed verbatim)
// and a same-length copy with string literals, dollar-quoted strings and
// comments blanked, used for keyword/function scanning (parsed enforcement
// must never match inside a literal).
type pgStatement struct {
	text     string
	stripped string
}

// ---------------------------------------------------------------------------
// Statement splitting (semicolon-aware, engine-dialect quoting)
// ---------------------------------------------------------------------------

// splitStatements splits a simple-query string into statements. Semicolons
// inside single-quoted strings ('' escapes), double-quoted identifiers,
// dollar-quoted strings ($tag$...$tag$) and comments do not split. A
// trailing statement without a semicolon is included.
func splitStatements(query string) ([]pgStatement, error) {
	var out []pgStatement
	var text, stripped strings.Builder
	i := 0
	n := len(query)
	flush := func() {
		if strings.TrimSpace(text.String()) != "" {
			out = append(out, pgStatement{text: text.String(), stripped: stripped.String()})
		}
		text.Reset()
		stripped.Reset()
	}
	for i < n {
		c := query[i]
		switch c {
		case '\'':
			// single-quoted string ('' is the escape)
			text.WriteByte(c)
			stripped.WriteByte(' ')
			i++
			for i < n {
				if query[i] == '\'' {
					if i+1 < n && query[i+1] == '\'' {
						text.WriteString("''")
						stripped.WriteString("  ")
						i += 2
						continue
					}
					break
				}
				text.WriteByte(query[i])
				stripped.WriteByte(' ')
				i++
			}
			if i >= n {
				return nil, errProtocol("unterminated string in query")
			}
			text.WriteByte('\'')
			stripped.WriteByte(' ')
			i++
		case '"':
			// double-quoted identifier ("" is the escape)
			text.WriteByte(c)
			stripped.WriteByte(' ')
			i++
			for i < n {
				if query[i] == '"' {
					if i+1 < n && query[i+1] == '"' {
						text.WriteString(`""`)
						stripped.WriteString("  ")
						i += 2
						continue
					}
					break
				}
				text.WriteByte(query[i])
				stripped.WriteByte(' ')
				i++
			}
			if i >= n {
				return nil, errProtocol("unterminated identifier in query")
			}
			text.WriteByte('"')
			stripped.WriteByte(' ')
			i++
		case '`':
			// backtick identifier (engine extension)
			text.WriteByte(c)
			stripped.WriteByte(' ')
			i++
			for i < n && query[i] != '`' {
				text.WriteByte(query[i])
				stripped.WriteByte(' ')
				i++
			}
			if i >= n {
				return nil, errProtocol("unterminated identifier in query")
			}
			text.WriteByte('`')
			stripped.WriteByte(' ')
			i++
		case '[':
			// bracket-quoted identifier (engine extension)
			end := strings.IndexByte(query[i:], ']')
			if end < 0 {
				return nil, errProtocol("unterminated identifier in query")
			}
			text.WriteString(query[i : i+end+1])
			stripped.WriteString(strings.Repeat(" ", end+1))
			i += end + 1
		case '$':
			// dollar-quoted string: $tag$ ... $tag$ (tag possibly empty)
			if tag, ok := dollarTag(query[i:]); ok {
				text.WriteString(tag)
				stripped.WriteString(strings.Repeat(" ", len(tag)))
				i += len(tag)
				end := strings.Index(query[i:], tag)
				if end < 0 {
					return nil, errProtocol("unterminated dollar-quoted string in query")
				}
				text.WriteString(query[i : i+end+len(tag)])
				stripped.WriteString(strings.Repeat(" ", end+len(tag)))
				i += end + len(tag)
				continue
			}
			text.WriteByte('$')
			stripped.WriteByte('$')
			i++
		case '-', '/':
			if i+1 < n && query[i+1] == c {
				// line comment (-- or //)
				end := strings.IndexByte(query[i:], '\n')
				if end < 0 {
					text.WriteString(query[i:])
					stripped.WriteString(strings.Repeat(" ", len(query)-i))
					i = n
				} else {
					text.WriteString(query[i : i+end])
					stripped.WriteString(strings.Repeat(" ", end))
					text.WriteByte('\n')
					stripped.WriteByte(' ')
					i += end + 1
				}
				continue
			}
			if c == '/' && i+1 < n && query[i+1] == '*' {
				// block comment (non-nesting, engine dialect)
				end := strings.Index(query[i+2:], "*/")
				if end < 0 {
					return nil, errProtocol("unterminated comment in query")
				}
				text.WriteString(query[i : i+end+4])
				stripped.WriteString(strings.Repeat(" ", end+4))
				i += end + 4
				continue
			}
			text.WriteByte(c)
			stripped.WriteByte(c)
			i++
		case ';':
			flush()
			i++
		default:
			text.WriteByte(c)
			stripped.WriteByte(c)
			i++
		}
	}
	flush()
	return out, nil
}

// dollarTag reports whether rest starts with a dollar-quote opener
// ($$ or $tag$) and returns it.
func dollarTag(rest string) (string, bool) {
	if len(rest) < 2 || rest[0] != '$' {
		return "", false
	}
	end := 1
	for end < len(rest) {
		c := rest[end]
		if c == '$' {
			return rest[:end+1], true
		}
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return "", false
		}
		end++
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Parsed classification and enforcement
// ---------------------------------------------------------------------------

// token is one word of the stripped statement with its byte offsets in the
// original text.
type token struct {
	upper string
	start int
	end   int
}

// tokens tokenizes the stripped statement into uppercase words.
func tokens(stripped string) []token {
	var out []token
	i := 0
	n := len(stripped)
	isSep := func(c byte) bool {
		return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '(' || c == ')' || c == ',' || c == ';' || c == '.'
	}
	for i < n {
		if isSep(stripped[i]) {
			i++
			continue
		}
		start := i
		for i < n && !isSep(stripped[i]) {
			i++
		}
		out = append(out, token{upper: strings.ToUpper(stripped[start:i]), start: start, end: i})
	}
	return out
}

// identifierAt slices the original text at a token's offsets and strips
// quotes/backticks/brackets and any schema prefix (native schemas map to
// the single namespace of the resolved database).
func identifierAt(text string, t token) string {
	raw := text[t.start:t.end]
	if i := strings.LastIndexByte(raw, '.'); i >= 0 {
		raw = raw[i+1:]
	}
	return strings.Trim(raw, "\"`[]")
}

// stmtClass is the parsed class of one statement.
type stmtClass int

const (
	classQuery stmtClass = iota // returns rows
	classWrite                  // data/schema mutation (sql.write required)
	classTxControl              // transaction control
	classSession                // session state control (SET/RESET; no data effects)
)

// keyword sets.
var readOnlyKeywords = map[string]bool{
	"SELECT": true, "VALUES": true, "EXPLAIN": true, "TABLE": true,
	"WITH": true,
}
var txKeywords = map[string]bool{
	"BEGIN": true, "START": true, "COMMIT": true, "END": true,
	"ROLLBACK": true, "ABORT": true, "SAVEPOINT": true, "RELEASE": true,
}
// sessionKeywords are the gateway session-state statements (SET, RESET and
// SHOW): they are implemented BY the gateway (session state never reaches
// the engine connection — the pinned engine's SQL dialect has no SET/SHOW
// and engine-side session state must not leak through the pool,
// postgresql-access.md session isolation), so their execution lives in
// executeSessionStatement, not in a verbatim engine round trip.
var sessionKeywords = map[string]bool{
	"SET": true, "RESET": true, "SHOW": true,
}
var writeKeywords = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true,
	"CREATE": true, "ALTER": true, "DROP": true, "GRANT": true,
	"REVOKE": true, "TRUNCATE": true, "MERGE": true, "REINDEX": true,
	"ANALYZE": true, "VACUUM": true,
}
var alwaysDeniedKeywords = map[string]bool{
	"ATTACH": true, "DETACH": true, "LOAD": true,
}

// dataMutationKeywords are the DML statements whose commit generates CDC
// frames (postgresql-access.md: "Native DML generates CDC but does not run
// record validation/hooks").
var dataMutationKeywords = map[string]bool{
	"INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true,
}

// denylistedFunctions is the file/extension access the gateway refuses on
// every statement (postgresql-access.md: sql.read denies "extension/file
// access through parsed native enforcement"; sql.write cannot acquire it
// either).
var denylistedFunctions = []string{
	"load_extension", "readfile", "writefile", "fts3_tokenizer",
	"sqlite3_tokenizer", "zipfile", "csv_read", "sha3_query",
}

// pragmaReadAllowlist is the read-only PRAGMA allowlist; every other pragma
// name and every assignment form is refused (unsafe PRAGMAs).
var pragmaReadAllowlist = map[string]bool{
	"table_info": true, "table_xinfo": true, "table_list": true,
	"index_list": true, "index_info": true, "index_xinfo": true,
	"foreign_key_list": true, "function_list": true, "module_list": true,
	"pragma_list": true, "compile_options": true, "encoding": true,
	"page_count": true, "freelist_count": true, "integrity_check": true,
	"quick_check": true, "auto_vacuum": true, "cache_size": true,
	"foreign_keys": true, "user_version": true, "application_id": true,
}

// classifyStatement performs the parsed enforcement for one statement
// (postgresql-access.md: whole-database administrative SQL with parsed
// scope denial). It returns the class, whether the statement returns rows,
// and an enforcement error.
func classifyStatement(st pgStatement, write bool, managed map[string]bool) (stmtClass, bool, *PGError) {
	toks := tokens(st.stripped)
	if len(toks) == 0 {
		return classQuery, false, nil
	}
	first := toks[0].upper

	// File/extension access is refused on every path.
	lower := strings.ToLower(st.stripped)
	for _, fn := range denylistedFunctions {
		if strings.Contains(lower, fn+"(") {
			return 0, false, errPrivilege("function %q (file/extension access) is refused through the PostgreSQL gateway", fn)
		}
	}

	switch {
	case first == "PRAGMA":
		return classifyPragma(st, toks)
	case first == "COPY":
		// COPY is refused in every form (postgresql-access.md: "COPY FROM
		// STDIN, CSV/binary and unsupported protocol combinations must fail
		// safely rather than hang. Disallow server filesystem COPY except a
		// trusted configured administrative path"). No COPY form is
		// qualified in this release: the statement fails safely here
		// instead of reaching the engine or the filesystem.
		return 0, false, errPrivilege("COPY is refused through the PostgreSQL gateway: no COPY form (STDOUT, file, CSV or binary) is qualified in this release")
	case txKeywords[first]:
		return classTxControl, false, nil
	case sessionKeywords[first]:
		// Gateway session-state control (SET/RESET/SHOW): never a data
		// write and never an engine round trip; the state lives on the
		// session so nothing can leak into the next session that acquires
		// the connection. SHOW returns rows.
		return classSession, first == "SHOW", nil
	case alwaysDeniedKeywords[first]:
		return 0, false, errPrivilege("%s (file access) is refused through the PostgreSQL gateway", first)
	case readOnlyKeywords[first]:
		if first == "WITH" {
			return classifyWith(toks)
		}
		return classQuery, true, nil
	case writeKeywords[first]:
		if !write {
			return 0, false, errPrivilege("permission denied for statement %q: the credential does not carry sql.write", first)
		}
		return classifyWrite(st, toks, managed)
	default:
		// Unclassified statements are denied, not executed (parsed
		// enforcement: "Reject unknown mutating classes in restricted
		// channels" — a keyword prefix is not authorization). SET-like
		// session control and the classified families above are the only
		// executable classes.
		return 0, false, errPrivilege("unsupported statement class %q through the PostgreSQL gateway", first)
	}
}

// classifyPragma enforces the read-only pragma policy: assignment forms
// and unknown pragma names are refused.
func classifyPragma(st pgStatement, toks []token) (stmtClass, bool, *PGError) {
	if len(toks) < 2 {
		return 0, false, errPrivilege("pragma requires a name")
	}
	name := strings.ToLower(identifierAt(st.text, toks[1]))
	if strings.Contains(st.stripped, "=") {
		return 0, false, errPrivilege("pragma assignment (PRAGMA %s=...) is refused through the PostgreSQL gateway", name)
	}
	if !pragmaReadAllowlist[name] {
		return 0, false, errPrivilege("pragma %q is refused through the PostgreSQL gateway", name)
	}
	return classQuery, true, nil
}

// classifyWith checks the CTE actions of a WITH statement: every "AS ("
// action must start with a read-only keyword.
func classifyWith(toks []token) (stmtClass, bool, *PGError) {
	for i, t := range toks {
		if t.upper != "AS" || i+1 >= len(toks) {
			continue
		}
		switch toks[i+1].upper {
		case "SELECT", "VALUES", "WITH", "TABLE":
			// read-only action
		case "INSERT", "UPDATE", "DELETE", "REPLACE", "MERGE":
			return 0, false, errPrivilege("permission denied: WITH carries a %q action and the credential does not carry sql.write", toks[i+1].upper)
		default:
			// Unclassified action body: denied like any other unclassified
			// statement (a keyword prefix is not authorization).
			return 0, false, errPrivilege("unsupported statement class %q through the PostgreSQL gateway", toks[i+1].upper)
		}
	}
	return classQuery, true, nil
}

// classifyWrite performs the write-path enforcement: the protected
// metadata namespace and managed-object DDL rejection. DML on managed
// collection tables stays allowed (native DML without record validation or
// hooks, documented); DDL against managed objects is refused until the
// reconciliation adapter exists.
func classifyWrite(st pgStatement, toks []token, managed map[string]bool) (stmtClass, bool, *PGError) {
	first := toks[0].upper
	switch first {
	case "CREATE":
		return classifyCreate(st, toks, managed)
	case "ALTER":
		// ALTER TABLE <name>
		if len(toks) >= 3 && toks[1].upper == "TABLE" {
			name := identifierAt(st.text, toks[2])
			if isReservedTable(name) {
				return 0, false, errPrivilege("table %q is protected application metadata", name)
			}
			if managed[name] {
				return 0, false, errManagedDDL(name)
			}
		}
		return classWrite, false, nil
	case "DROP":
		// DROP TABLE|INDEX|VIEW|TRIGGER [IF EXISTS] <name>
		if len(toks) >= 3 {
			idx := 2
			if toks[2].upper == "IF" && len(toks) >= 4 {
				idx = 4 // IF EXISTS
			}
			if idx < len(toks) {
				name := identifierAt(st.text, toks[idx])
				if isReservedTable(name) {
					return 0, false, errPrivilege("table %q is protected application metadata", name)
				}
				if managed[name] {
					return 0, false, errManagedDDL(name)
				}
			}
		}
		return classWrite, false, nil
	case "INSERT", "REPLACE":
		// INSERT [OR ...] INTO <table> / REPLACE INTO <table>
		idx := 1
		if first == "INSERT" {
			for idx < len(toks) && toks[idx].upper != "INTO" && toks[idx].upper != "DEFAULT" && toks[idx].upper != "VALUES" {
				idx++
			}
			if idx < len(toks) {
				idx++ // INTO
			}
		} else {
			idx++ // REPLACE INTO
		}
		if idx < len(toks) {
			name := identifierAt(st.text, toks[idx])
			if isReservedTable(name) {
				return 0, false, errPrivilege("table %q is protected application metadata", name)
			}
		}
		return classWrite, hasReturning(st), nil
	case "UPDATE":
		// UPDATE <table>
		if len(toks) >= 2 {
			name := identifierAt(st.text, toks[1])
			if isReservedTable(name) {
				return 0, false, errPrivilege("table %q is protected application metadata", name)
			}
		}
		return classWrite, hasReturning(st), nil
	case "DELETE":
		// DELETE FROM <table>
		if len(toks) >= 3 && toks[1].upper == "FROM" {
			name := identifierAt(st.text, toks[2])
			if isReservedTable(name) {
				return 0, false, errPrivilege("table %q is protected application metadata", name)
			}
		}
		return classWrite, hasReturning(st), nil
	default:
		return classWrite, false, nil
	}
}

// hasReturning reports whether a DML statement carries a RETURNING clause
// (the stripped copy never contains literals, so the scan is exact).
func hasReturning(st pgStatement) bool {
	toks := tokens(st.stripped)
	for _, t := range toks {
		if t.upper == "RETURNING" {
			return true
		}
	}
	return false
}

// isTempDDL reports whether a write statement creates a temporary object
// (CREATE [TEMP|TEMPORARY] TABLE/VIEW/INDEX/TRIGGER). Temporary objects are
// engine CONNECTION state: the owning connection is reset (temp objects
// dropped) before it returns to the pool, or discarded, so a later session
// cannot inherit them (postgresql-access.md session isolation).
func isTempDDL(st pgStatement) bool {
	toks := tokens(st.stripped)
	if len(toks) == 0 || toks[0].upper != "CREATE" {
		return false
	}
	for i := 1; i < len(toks) && i <= 4; i++ {
		switch toks[i].upper {
		case "TEMP", "TEMPORARY":
			return true
		case "TABLE", "VIEW", "INDEX", "TRIGGER", "VIRTUAL", "UNIQUE", "IF":
			// the object kind (or an earlier modifier): stop scanning
			return false
		}
	}
	return false
}

// pgParameterSQL rewrites PostgreSQL parameter markers ($N) in the original
// statement text into the engine's explicitly numbered markers (?N).
//
// The pinned engine numbers $-prefixed markers like SQLite: by order of
// appearance, NOT by the number in the name (turso sqlite/parser
// create_variable assigns named variables the next sequential index), while
// PostgreSQL binds $N to parameter N. Rewriting to ?N gives every marker an
// explicit index the engine honors (create_variable -> Variable::numbered),
// so out-of-order statements such as "SELECT ... WHERE a = $2 AND b = $1"
// bind exactly as a PostgreSQL client expects.
//
// The rewrite is driven by the statement's same-length stripped copy (literals,
// quoted identifiers and dollar-quoted strings are blanked there), so a $N
// inside a literal or identifier is never rewritten. A $N in the stripped
// copy is a marker only when the preceding character is not an identifier
// character (a $ mid-identifier, e.g. tbl$1, is part of the name).
func pgParameterSQL(text, stripped string) string {
	var out strings.Builder
	i := 0
	n := len(stripped)
	for i < n {
		if stripped[i] == '$' {
			prev := byte(' ')
			if i > 0 {
				prev = stripped[i-1]
			}
			if !(prev >= 'a' && prev <= 'z' || prev >= 'A' && prev <= 'Z' ||
				prev >= '0' && prev <= '9' || prev == '_' || prev == '$') {
				j := i + 1
				for j < n && stripped[j] >= '0' && stripped[j] <= '9' && j-i <= 7 {
					j++
				}
				if j > i+1 { // at least one digit: a numbered marker
					// The original text carries the same $N span (both copies
					// are index-aligned); replace it with ?N (the digits only).
					out.WriteString("?")
					out.WriteString(text[i+1 : j])
					i = j
					continue
				}
			}
		}
		out.WriteByte(text[i])
		i++
	}
	return out.String()
}

// sourceTablesOf extracts the table identifiers a statement reads or writes
// (FROM/JOIN sources of a query, the INTO/UPDATE/DELETE-FROM target of a
// write) in first-appearance order, from the stripped copy so literals never
// contribute. Subqueries (FROM (SELECT ...)) contribute no table. Aliases
// (FROM tbl AS a / FROM tbl a) are not table names. Unknown shapes simply
// contribute no table (the column-type resolution then falls back to TEXT,
// the pinned frontend's undetermined-column default).
func sourceTablesOf(stripped string) []string {
	// clauseStarts introduce a table; clauseEnders terminate a table list;
	// soft words are skipped without ending anything (join operators,
	// aliases, lateral).
	clauseStarts := map[string]bool{"FROM": true, "JOIN": true, "INTO": true, "UPDATE": true}
	clauseEnders := map[string]bool{
		"WHERE": true, "GROUP": true, "HAVING": true, "ORDER": true,
		"LIMIT": true, "OFFSET": true, "WINDOW": true, "UNION": true,
		"EXCEPT": true, "INTERSECT": true, "RETURNING": true, "SET": true,
		"VALUES": true, "DEFAULT": true, "SELECT": true, "WITH": true,
		"INDEXED": true,
	}
	softWords := map[string]bool{
		"AS": true, "ON": true, "USING": true, "CROSS": true,
		"NATURAL": true, "INNER": true, "LEFT": true, "RIGHT": true,
		"FULL": true, "OUTER": true, "LATERAL": true,
	}
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	// Token walker over words and separator punctuation of the stripped
	// copy (commas, parens and dots must be visible), with one-token
	// pushback so table names are disambiguated from their aliases.
	i := 0
	n := len(stripped)
	type srcTok struct {
		word string
		sep  byte
	}
	var pending *srcTok
	next := func() (srcTok, bool) {
		if pending != nil {
			t := *pending
			pending = nil
			return t, true
		}
		for i < n {
			c := stripped[i]
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				i++
				continue
			}
			if c == ',' || c == '(' || c == ')' || c == '.' {
				i++
				return srcTok{sep: c}, true
			}
			start := i
			for i < n {
				c := stripped[i]
				if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
					c == ',' || c == '(' || c == ')' || c == '.' {
					break
				}
				i++
			}
			return srcTok{word: strings.ToUpper(stripped[start:i])}, true
		}
		return srcTok{}, false
	}
	unread := func(t srcTok) { pending = &t }

	expectTable := false
	inClause := false
	for {
		t, ok := next()
		if !ok {
			break
		}
		if t.sep == '(' {
			// A subquery (or function call) where a table could appear:
			// skip its contents, then keep scanning for later clauses.
			depth := 1
			for i < n && depth > 0 {
				switch stripped[i] {
				case '(':
					depth++
				case ')':
					depth--
				}
				i++
			}
			expectTable = false
			continue
		}
		if t.sep == ',' {
			// Comma-separated table list inside a FROM clause
			// (FROM a, b): the next word is the next table.
			if inClause {
				expectTable = true
			}
			continue
		}
		if t.sep == '.' {
			continue // qualified names are assembled in the table branch
		}
		if t.sep != 0 {
			continue // ')': list of a skipped subquery is done
		}
		if clauseStarts[t.word] {
			expectTable = true
			inClause = true
			continue
		}
		if inClause && clauseEnders[t.word] {
			// The table list ended (WHERE/GROUP/VALUES/...); a later
			// clauseStart re-arms it.
			inClause = false
			expectTable = false
			continue
		}
		if expectTable && (softWords[t.word] || t.word == "JOIN") {
			// Join operators and lateral keep the clause armed; a JOIN
			// introduces its own table next.
			if t.word == "JOIN" {
				expectTable = true
			}
			continue
		}
		if expectTable {
			// A candidate table identifier; a qualified name (schema.table)
			// continues through dots — the last component is the table
			// (native schemas map to the single namespace of the resolved
			// database — postgresql-access.md).
			name := strings.ToLower(t.word)
			for {
				t2, ok2 := next()
				if !ok2 {
					add(lastComponent(name))
					break
				}
				if t2.sep == '.' {
					t3, ok3 := next()
					if !ok3 {
						add(lastComponent(name))
						break
					}
					if t3.sep != 0 {
						unread(t3)
						add(lastComponent(name))
						break
					}
					name = name + "." + strings.ToLower(t3.word)
					continue
				}
				// Not a dot: the table name is complete; the token is the
				// alias or the next clause — put it back.
				unread(t2)
				add(lastComponent(name))
				break
			}
			expectTable = false
			continue
		}
		// Outside a table clause: ignore the word.
	}
	return out
}

// lastComponent returns the table name of a possibly qualified identifier
// (schema.table -> table; engine namespaces resolve to the single namespace
// of the resolved database — postgresql-access.md).
func lastComponent(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return name[i+1:]
	}
	return name
}

// declaredColumnOIDs resolves the wire type of each result column from the
// DECLARED column types (postgresql-access.md: "prepared statements,
// parameter types, text/binary values" with the pinned compatibility
// mapping), not from row values: a NULL in the first row reports the
// column's type, and the same query reports the same OIDs regardless of its
// rows.
//
// Resolution: the statement's source tables are looked up in the engine
// catalog through PRAGMA table_info ON THE SAME RUNNER (the implicit
// batch's transaction or the session's connection, so uncommitted DDL and
// temporary objects are visible), and a result column whose name matches
// exactly one table's column takes that declared type through
// sqliteTypeToPGType (the pinned frontend's declared-name mapping). Anything
// unresolvable — expressions, ambiguous join columns, unknown tables —
// reports TEXT, the pinned frontend's safe default for undetermined columns
// (turso postgres/server/lib.rs resolve_pg_type_for_column).
func declaredColumnOIDs(ctx context.Context, runner engineRunner, colNames []string, stripped string) []int32 {
	oids := make([]int32, len(colNames))
	if len(colNames) == 0 {
		return oids
	}
	for i := range oids {
		oids[i] = oidText
	}
	type columnDecl struct {
		decl string
	}
	// table -> column name -> declared type; and per-column-name table count
	// (ambiguity across joined tables falls back to TEXT).
	tables := map[string]map[string]columnDecl{}
	nameHits := map[string]int{}
	for _, table := range sourceTablesOf(stripped) {
		rows, err := runner.QueryContext(ctx, `PRAGMA table_info(`+quoteEngineIdentifier(table)+`)`)
		if err != nil {
			return oids // catalog unreadable: everything stays TEXT
		}
		cols := map[string]columnDecl{}
		for rows.Next() {
			var cid int64
			var cname, decl string
			var notNull int64
			var dflt, pk any
			if err := rows.Scan(&cid, &cname, &decl, &notNull, &dflt, &pk); err != nil {
				rows.Close()
				return oids
			}
			if decl != "" {
				cols[strings.ToLower(cname)] = columnDecl{decl: decl}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return oids
		}
		rows.Close()
		if len(cols) > 0 {
			tables[table] = cols
			for name := range cols {
				nameHits[name]++
			}
		}
	}
	for i, name := range colNames {
		if nameHits[strings.ToLower(name)] != 1 {
			continue // no match or ambiguous across the statement's tables
		}
		for _, cols := range tables {
			if decl, ok := cols[strings.ToLower(name)]; ok {
				oids[i] = sqliteTypeToPGType(decl.decl)
				break
			}
		}
	}
	return oids
}

// classifyCreate enforces managed-object DDL rejection for CREATE.
func classifyCreate(st pgStatement, toks []token, managed map[string]bool) (stmtClass, bool, *PGError) {
	i := 1
	for i < len(toks) {
		switch toks[i].upper {
		case "TEMP", "TEMPORARY", "UNIQUE", "VIRTUAL":
			i++
			continue
		case "IF":
			i += 2 // IF NOT EXISTS
			continue
		}
		break
	}
	if i >= len(toks) {
		return classWrite, false, nil
	}
	kind := toks[i].upper
	i++
	switch kind {
	case "TABLE", "VIEW":
		if i >= len(toks) {
			return classWrite, false, nil
		}
		name := identifierAt(st.text, toks[i])
		if isReservedTable(name) {
			return 0, false, errPrivilege("table %q is protected application metadata", name)
		}
		if managed[name] {
			return 0, false, errManagedDDL(name)
		}
		return classWrite, false, nil
	case "INDEX", "TRIGGER":
		// the affected table follows ON <table>
		for j := i; j < len(toks)-1; j++ {
			if toks[j].upper == "ON" {
				name := identifierAt(st.text, toks[j+1])
				if isReservedTable(name) {
					return 0, false, errPrivilege("table %q is protected application metadata", name)
				}
				if managed[name] {
					return 0, false, errManagedDDL(name)
				}
				break
			}
		}
		return classWrite, false, nil
	default:
		return classWrite, false, nil
	}
}

// isReservedTable reports whether a table name is in the protected
// metadata namespace: the MiraBase registry (`_mb_*`), the inherited
// system tables (`_collections`, `_superusers`, ...) and the SQLite
// internals (sqlite_*). Writes and DDL there are refused on every access
// path ("sql.write still cannot alter protected application metadata/
// schema outside the controlled administrative workflow").
func isReservedTable(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "_") || strings.HasPrefix(lower, "sqlite_") {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Execution
// ---------------------------------------------------------------------------

// engineRunner is the statement-execution surface shared by *sql.Conn and
// *sql.Tx: the implicit multi-statement batch runs over ONE transaction so
// a statement's CDC frame commits atomically with the data it describes.
type engineRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// execResult carries the state transitions of one executed batch.
type execResult struct {
	empty        bool   // empty query (EmptyQueryResponse)
	started      bool   // a BEGIN opened a transaction block
	ended        bool   // a transaction block ended
	failed       bool   // the batch left a failed transaction block
	tag          string // CommandComplete tag of the last statement
	rowsReturned int    // rows returned by the last statement
}

// executeBatch runs one simple-query batch over the held connection and
// writes the wire responses (RowDescription/DataRow/CommandComplete per
// statement). It implements PostgreSQL batch semantics: multiple
// statements without explicit transaction control run in one implicit
// transaction; an error skips the remaining statements.
//
// Native DML generates CDC (postgresql-access.md: "Native DML generates
// CDC but does not run record validation/hooks"): every classified data
// mutation appends its _mb_changes frame INSIDE the transaction that
// commits it — the implicit batch's shared transaction, or the statement's
// own / the session's open transaction block on the single-statement path.
// A failed capture rolls back the DML (capture and data are atomic).
func (s *session) executeBatch(ctx context.Context, conn *sql.Conn, stmts []pgStatement) (execResult, error) {
	res := execResult{}
	if len(stmts) == 0 {
		res.empty = true
		return res, nil
	}
	implicit := len(stmts) > 1 && !s.inTx
	if implicit {
		hasTxControl := false
		for _, st := range stmts {
			toks := tokens(st.stripped)
			if len(toks) > 0 && txKeywords[toks[0].upper] {
				hasTxControl = true
				break
			}
		}
		implicit = !hasTxControl
	}
	var runner engineRunner = conn
	if implicit {
		// The whole implicit block is ONE transaction object: statements
		// and their CDC frames commit (or roll back) together.
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return res, classifyEngineError(err)
		}
		runner = tx
	}
	failed := s.failed
	for _, st := range stmts {
		if failed {
			toks := tokens(st.stripped)
			kw := ""
			if len(toks) > 0 {
				kw = toks[0].upper
			}
			if kw != "COMMIT" && kw != "ROLLBACK" && kw != "END" {
				if implicit {
					_ = runner.(*sql.Tx).Rollback()
				}
				return res, newPGError(stateInFailedSQLTransaction,
					"current transaction is aborted, commands ignored until end of transaction block")
			}
		}
		r, err := s.executeStatement(ctx, runner, st, failed)
		if err != nil {
			if implicit {
				// The whole implicit block rolls back: no state survives
				// to report.
				_ = runner.(*sql.Tx).Rollback()
				return execResult{}, err
			}
			// An explicit transaction block keeps its earlier statements:
			// the block stays open and enters the failed state.
			return res, err
		}
		if r.started {
			res.started = true
		}
		if r.ended {
			res.ended = true
			failed = false
		}
		if r.failed {
			failed = true
			res.failed = true
		}
	}
	if implicit {
		if err := runner.(*sql.Tx).Commit(); err != nil {
			_ = runner.(*sql.Tx).Rollback()
			return res, classifyEngineError(err)
		}
	}
	return res, nil
}

// executeStatement executes one classified statement on the current
// execution runner (the implicit batch's transaction or the held
// connection) and writes its wire responses.
func (s *session) executeStatement(ctx context.Context, runner engineRunner, st pgStatement, inFailedTx bool) (execResult, error) {
	res := execResult{}
	class, returnsRows, cerr := classifyStatement(st, s.writeScope, s.managed)
	if cerr != nil {
		return res, cerr
	}
	toks := tokens(st.stripped)
	first := ""
	if len(toks) > 0 {
		first = toks[0].upper
	}

	// Transaction boundaries re-check the credential's durable state
	// (postgresql-access.md: "Credential scope, expiry and policy epoch
	// are checked at connection and transaction boundaries").
	if class == classTxControl {
		if err := s.credentialBoundary(ctx); err != nil {
			return res, err
		}
	}

	sctx, cancel := context.WithTimeout(ctx, statementTimeout)
	defer cancel()

	// COMMIT in a failed block performs a rollback and reports the
	// ROLLBACK tag (the pinned frontend's command-tag override).
	isCommit := first == "COMMIT"
	isRollback := first == "ROLLBACK" || first == "END"
	if inFailedTx && isCommit {
		if _, err := runner.ExecContext(sctx, "ROLLBACK"); err != nil {
			return res, classifyEngineError(err)
		}
		res.ended = true
		res.tag = "ROLLBACK"
		if err := s.writeCommandComplete("ROLLBACK"); err != nil {
			return res, err
		}
		return res, nil
	}

	switch class {
	case classTxControl:
		if _, err := runner.ExecContext(sctx, st.text); err != nil {
			pe := classifyEngineError(err)
			// PostgreSQL answers COMMIT/ROLLBACK outside a transaction
			// block with a WARNING notice ("there is no transaction in
			// progress") and a CommandComplete, not with an error. Mirror
			// that when the engine reports no active transaction and the
			// session holds no open block (the notice is the wire-visible
			// degraded behavior, postgresql-access.md "errors/notices").
			if (isCommit || isRollback) && !s.inTx {
				if p, ok := pe.(*PGError); ok && p.Code == stateNoActiveSQLTransaction {
					if nerr := s.writeNotice(newPGError(stateNoActiveSQLTransaction,
						"there is no transaction in progress")); nerr != nil {
						return res, nerr
					}
					res.tag = "COMMIT"
					if isRollback {
						res.tag = "ROLLBACK"
					}
					if err := s.writeCommandComplete(res.tag); err != nil {
						return res, err
					}
					return res, nil
				}
			}
			return res, pe
		}
		switch {
		case first == "BEGIN" || first == "START":
			res.started = true
			res.tag = "BEGIN"
		case isCommit:
			res.ended = true
			res.tag = "COMMIT"
		case isRollback:
			res.ended = true
			res.tag = "ROLLBACK"
		case first == "SAVEPOINT":
			res.tag = "SAVEPOINT"
		case first == "RELEASE":
			res.tag = "RELEASE"
		default:
			res.tag = first
		}
		if err := s.writeCommandComplete(res.tag); err != nil {
			return res, err
		}
		return res, nil
	case classSession:
		// Gateway session-state control (SET/RESET/SHOW): the state lives
		// on the SESSION (never on the engine connection), so nothing can
		// leak into the next session that acquires the pooled connection
		// (postgresql-access.md session isolation; the pinned engine's SQL
		// dialect has no SET/SHOW statements).
		return s.executeSessionStatement(st)
	case classQuery:
		r, err := s.executeQueryStatement(sctx, runner, st)
		if err != nil {
			return r, err
		}
		r.tag = selectTag(first, r.rowsReturned)
		if err := s.writeCommandComplete(r.tag); err != nil {
			return r, err
		}
		return r, nil
	default: // classWrite
		if dataMutationKeywords[first] {
			r, err := s.executeDataMutation(sctx, runner, st, first, toks, returnsRows)
			if err != nil {
				return r, err
			}
			if err := s.writeCommandComplete(r.tag); err != nil {
				return r, err
			}
			return r, nil
		}
		if returnsRows {
			r, err := s.executeQueryStatement(sctx, runner, st)
			if err != nil {
				return r, err
			}
			// DML-with-RETURNING reports the mutation tag shape.
			r.tag = refineTag(first, writeTag(first, int64(r.rowsReturned)), toks)
			if err := s.writeCommandComplete(r.tag); err != nil {
				return r, err
			}
			return r, nil
		}
		result, err := runner.ExecContext(sctx, st.text)
		if err != nil {
			return res, classifyEngineError(err)
		}
		// Temporary objects are engine CONNECTION state: the owning
		// connection is reset (temp objects dropped) — or discarded —
		// before it returns to the pool (postgresql-access.md session
		// isolation; releaseConn).
		if isTempDDL(st) {
			s.sessionDirty = true
		}
		affected, aerr := result.RowsAffected()
		if aerr != nil {
			affected = 0
		}
		res.tag = refineTag(first, writeTag(first, affected), toks)
		if err := s.writeCommandComplete(res.tag); err != nil {
			return res, err
		}
		return res, nil
	}
}

// changeCaptureOf builds the CDC frame of one executed DML statement:
// the affected table from the parsed target (the BARE table name, matching
// the record-path frames and the pull-side authorized-table matching) and
// the record id resolved through the engine's last-insert identity inside
// the same transaction.
func changeCaptureOf(ctx context.Context, runner engineRunner, first string, toks []token, text string) application.ChangeCapture {
	action := map[string]string{
		"INSERT": "create", "REPLACE": "create", "UPDATE": "update", "DELETE": "delete",
	}[first]
	table := writeTargetTable(first, toks, text)
	return application.ChangeCapture{
		Table:    table,
		RecordID: recordIDOfMutation(ctx, runner, first, table),
		Action:   action,
	}
}

// runDataMutation executes one DML statement on the correct transaction
// scope and appends its CDC frame in the SAME transaction that commits it:
// the implicit batch's shared transaction, the session's open transaction
// block, or a statement-scoped transaction (postgresql-access.md: "Native
// DML generates CDC but does not run record validation/hooks"; capture and
// data are atomic — a failed capture fails the statement and its DML).
// run performs the statement on the given runner; captureOf builds the
// frame ON the runner that executed the statement (the last-insert
// identity and the affected table are read in the same transaction, after
// the statement ran).
func (s *session) runDataMutation(ctx context.Context, runner engineRunner, captureOf func(engineRunner) application.ChangeCapture, run func(engineRunner) (execResult, error)) (execResult, error) {
	captureAndCommit := func(r engineRunner, out execResult, runErr error) (execResult, error) {
		if runErr != nil {
			return out, runErr
		}
		capture := captureOf(r)
		if _, cerr := application.CaptureChangesOnRunner(ctx, r, "pg", []application.ChangeCapture{capture}); cerr != nil {
			return out, newPGError(stateInternalError, "failed to record the CDC frame for the statement: "+cerr.Error())
		}
		return out, nil
	}
	if tx, ok := runner.(*sql.Tx); ok {
		out, err := run(tx)
		return captureAndCommit(tx, out, err)
	}
	conn := runner.(*sql.Conn)
	if s.inTx {
		out, err := run(conn)
		return captureAndCommit(conn, out, err)
	}
	tx, terr := conn.BeginTx(ctx, nil)
	if terr != nil {
		return execResult{}, classifyEngineError(terr)
	}
	out, err := run(tx)
	out, err = captureAndCommit(tx, out, err)
	if err != nil {
		_ = tx.Rollback()
		return out, err
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return out, classifyEngineError(err)
	}
	return out, nil
}

// executeDataMutation runs one classified DML statement of the simple
// query path (no arguments, no row limit) with atomic CDC capture — both
// the plain form and the RETURNING form (the returned rows are streamed
// inside the same transaction scope the frame is appended to).
func (s *session) executeDataMutation(ctx context.Context, runner engineRunner, st pgStatement, first string, toks []token, returnsRows bool) (execResult, error) {
	res := execResult{}

	if returnsRows {
		captureOf := func(r engineRunner) application.ChangeCapture {
			return changeCaptureOf(ctx, r, first, toks, st.text)
		}
		r, err := s.runDataMutation(ctx, runner, captureOf, func(r engineRunner) (execResult, error) {
			return s.executeQueryStatement(ctx, r, st)
		})
		if err != nil {
			return r, err
		}
		r.tag = refineTag(first, writeTag(first, int64(r.rowsReturned)), toks)
		return r, nil
	}

	captureOf := func(r engineRunner) application.ChangeCapture {
		return changeCaptureOf(ctx, r, first, toks, st.text)
	}
	return s.runDataMutation(ctx, runner, captureOf, func(r engineRunner) (execResult, error) {
		result, err := r.ExecContext(ctx, st.text)
		if err != nil {
			return res, classifyEngineError(err)
		}
		affected, aerr := result.RowsAffected()
		if aerr != nil {
			affected = 0
		}
		res.tag = refineTag(first, writeTag(first, affected), toks)
		return res, nil
	})
}

// writeTargetTable extracts the DML target table name (INSERT INTO
// <table>, REPLACE INTO <table>, UPDATE <table>, DELETE FROM <table>) as
// the bare identifier; "" when the shape is unrecognized (no frame is
// recorded for an unknown target).
func writeTargetTable(first string, toks []token, text string) string {
	idx := -1
	switch first {
	case "INSERT":
		for i := 1; i < len(toks); i++ {
			if toks[i].upper == "INTO" {
				idx = i + 1
				break
			}
		}
	case "REPLACE":
		idx = 2
	case "UPDATE":
		idx = 1
	case "DELETE":
		if len(toks) >= 3 && toks[1].upper == "FROM" {
			idx = 2
		}
	}
	if idx < 0 || idx >= len(toks) {
		return ""
	}
	return identifierAt(text, toks[idx])
}

// quoteEngineIdentifier renders a parsed identifier as a safely quoted SQL
// identifier (inner quotes are doubled).
func quoteEngineIdentifier(name string) string {
	if name == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// recordIDOfMutation resolves the affected record id of a single-row
// INSERT through the engine's last-insert identity (best effort; frames of
// other statements or multi-row inserts carry an empty record id).
func recordIDOfMutation(ctx context.Context, r engineRunner, first, table string) string {
	if first != "INSERT" || table == "" {
		return ""
	}
	var rowid int64
	if err := r.QueryRowContext(ctx, `SELECT last_insert_rowid()`).Scan(&rowid); err != nil || rowid <= 0 {
		return ""
	}
	var id any
	if err := r.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT "id" FROM %s WHERE rowid = ?`, quoteEngineIdentifier(table)), rowid).Scan(&id); err != nil {
		return ""
	}
	if s, ok := id.(string); ok {
		return s
	}
	if b, ok := id.([]byte); ok {
		return string(b)
	}
	return fmt.Sprintf("%v", id)
}

// selectTag renders the CommandComplete tag of a rows-returning statement.
func selectTag(first string, count int) string {
	if first == "EXPLAIN" {
		return "EXPLAIN"
	}
	return "SELECT " + strconv.Itoa(count)
}

// executeQueryStatement runs a rows-returning statement over the current
// execution runner: RowDescription from the DECLARED column types (resolved
// through the engine catalog on the same runner), then streaming DataRows.
// The driver prepares the statement lazily — reading rows.Columns() steps
// nothing — so the description is row-independent: a NULL first row reports
// the column's declared type and the OIDs never vary with the rows
// (postgresql-access.md: declared compatibility matrix; expressions and
// ambiguous columns report TEXT, the pinned frontend's undetermined-column
// default). The CommandComplete is written by the caller (the tag depends
// on the statement class).
func (s *session) executeQueryStatement(ctx context.Context, runner engineRunner, st pgStatement) (execResult, error) {
	res := execResult{}
	rows, err := runner.QueryContext(ctx, st.text)
	if err != nil {
		return res, classifyEngineError(err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return res, classifyEngineError(err)
	}
	oids := declaredColumnOIDs(ctx, runner, cols, st.stripped)
	columns := make([]pgColumn, len(cols))
	for i, name := range cols {
		columns[i] = pgColumn{Name: name, OID: oids[i]}
	}
	if err := s.writeRowDescription(columns); err != nil {
		return res, err
	}

	vals := make([]any, len(cols))
	dests := make([]any, len(cols))
	count := 0
	for {
		for i := range dests {
			vals[i] = nil
			dests[i] = &vals[i]
		}
		if !rows.Next() {
			break
		}
		if serr := rows.Scan(dests...); serr != nil {
			return res, classifyEngineError(serr)
		}
		count++
		if err := s.writeDataRow(vals); err != nil {
			return res, err
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return res, classifyEngineError(rerr)
	}

	res.rowsReturned = count
	return res, nil
}

// writeTag renders the CommandComplete tag of a write statement
// (mirroring wire_tag in the pinned frontend: INSERT reports OID 0;
// DML-with-RETURNING reports the number of returned rows).
func writeTag(first string, affected int64) string {
	switch first {
	case "INSERT", "REPLACE":
		return "INSERT 0 " + strconv.FormatInt(affected, 10)
	case "UPDATE":
		return "UPDATE " + strconv.FormatInt(affected, 10)
	case "DELETE":
		return "DELETE " + strconv.FormatInt(affected, 10)
	case "CREATE", "DROP", "ALTER":
		return first
	case "VACUUM":
		return "VACUUM"
	case "REINDEX":
		return "REINDEX"
	case "ANALYZE":
		return "ANALYZE"
	default:
		return first
	}
}

// refineTag narrows CREATE/DROP/ALTER tags with the object kind
// (CREATE TABLE, DROP INDEX, ALTER TABLE — as the pinned frontend emits).
func refineTag(first, tag string, toks []token) string {
	if first != "CREATE" && first != "DROP" && first != "ALTER" {
		return tag
	}
	if len(toks) >= 2 {
		kind := toks[1].upper
		switch kind {
		case "TABLE", "INDEX", "VIEW", "TRIGGER":
			return first + " " + kind
		}
	}
	return tag
}

// classifyEngineError maps an engine/driver error to a wire error
// (mirroring the SQLSTATE classes the pinned frontend reports; the
// engine's own message is preserved after the driver prefix).
func classifyEngineError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return newPGError(stateQueryCanceled, "canceling statement due to user request")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newPGError(stateQueryCanceled, "canceling statement due to statement timeout")
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	msg = strings.TrimPrefix(msg, "turso: ")
	// Resolution failures come first: the engine phrases a missing table
	// (even in a SELECT) as "Parse error: no such table ...", and
	// PostgreSQL reports it as undefined_table (42P01), not a syntax
	// error — the generic parse-error class must not shadow it.
	switch {
	case strings.Contains(lower, "no such table"), strings.Contains(lower, "no such view"):
		return newPGError(stateUndefinedTable, msg)
	case strings.Contains(lower, "no such column"), strings.Contains(lower, "has no column named"):
		return newPGError(stateUndefinedColumn, msg)
	case strings.Contains(lower, "no such function"):
		return newPGError(stateUndefinedFunction, msg)
	case strings.Contains(lower, "syntax error"), strings.Contains(lower, "parse error"),
		strings.Contains(lower, "unrecognized token"), strings.Contains(lower, "incomplete input"):
		return newPGError(stateSyntaxError, msg)
	case strings.Contains(lower, "unique constraint"):
		return newPGError(stateUniqueViolation, msg)
	case strings.Contains(lower, "foreign key constraint"):
		return newPGError(stateForeignKeyViolation, msg)
	case strings.Contains(lower, "not null constraint"):
		return newPGError(stateNotNullViolation, msg)
	case strings.Contains(lower, "check constraint"), strings.Contains(lower, "constraint failed"):
		return newPGError(stateCheckViolation, msg)
	case strings.Contains(lower, "division by zero"):
		return newPGError(stateDivisionByZero, msg)
	case strings.Contains(lower, "already exists"):
		return newPGError(stateDuplicateObject, msg)
	case strings.Contains(lower, "within a transaction"):
		return newPGError(stateActiveSQLTransaction, msg)
	case strings.Contains(lower, "no transaction is active"), strings.Contains(lower, "no such savepoint"):
		return newPGError(stateNoActiveSQLTransaction, msg)
	case strings.Contains(lower, "interrupted"):
		return newPGError(stateQueryCanceled, msg)
	default:
		return newPGError(stateInternalError, msg)
	}
}

// encodeDataRow renders one row in text format.
func encodeDataRow(values []any) ([]byte, error) {
	buf := make([]byte, 0, 64)
	var colCount [2]byte
	binary.BigEndian.PutUint16(colCount[:], uint16(len(values)))
	buf = append(buf, colCount[:]...)
	for _, v := range values {
		if v == nil {
			var nullLen [4]byte
			binary.BigEndian.PutUint32(nullLen[:], 0xffffffff)
			buf = append(buf, nullLen[:]...)
			continue
		}
		enc, err := encodeTextValue(v)
		if err != nil {
			return nil, err
		}
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(enc)))
		buf = append(buf, lenBuf[:]...)
		buf = append(buf, enc...)
	}
	return buf, nil
}

// encodeRowDescription renders the RowDescription payload.
func encodeRowDescription(columns []pgColumn) []byte {
	buf := make([]byte, 0, 32*len(columns)+2)
	var colCount [2]byte
	binary.BigEndian.PutUint16(colCount[:], uint16(len(columns)))
	buf = append(buf, colCount[:]...)
	for _, c := range columns {
		buf = append(buf, c.Name...)
		buf = append(buf, 0)
		var fixed [18]byte
		binary.BigEndian.PutUint32(fixed[0:4], 0) // table oid
		binary.BigEndian.PutUint16(fixed[4:6], 0) // column attnum
		binary.BigEndian.PutUint32(fixed[6:10], uint32(c.OID))
		binary.BigEndian.PutUint16(fixed[10:12], uint16(typeLen(c.OID)))
		binary.BigEndian.PutUint32(fixed[12:16], 0xffffffff) // typmod
		binary.BigEndian.PutUint16(fixed[16:18], uint16(c.Format))
		buf = append(buf, fixed[:]...)
	}
	return buf
}

// typeLen is the wire typlen for the reported OIDs.
func typeLen(oid int32) int16 {
	switch oid {
	case oidUnknown:
		return -2
	default:
		return -1
	}
}
