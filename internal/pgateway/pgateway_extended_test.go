package pgateway_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"mirabase/internal/application"
	"mirabase/internal/pgateway"
	"mirabase/tests"
)

// ---------------------------------------------------------------------------
// Extended query protocol wire client (Parse/Bind/Describe/Execute/Sync).
// ---------------------------------------------------------------------------

// msgBuf assembles one frontend message payload.
type msgBuf struct{ bytes.Buffer }

// writeCString appends a NUL-terminated string.
func (b *msgBuf) writeCString(s string) {
	b.WriteString(s)
	b.WriteByte(0)
}

// extResult is one extended-protocol round trip's collected responses.
type extResult struct {
	parseComplete    bool
	bindComplete     bool
	closeComplete    bool
	paramOIDs        []int32 // ParameterDescription
	columns          []string
	oids             []int32
	rows             [][]*string
	tags             []string
	portalSuspended  bool
	emptyQuery       bool
	noData           bool
	err              *pgErrorWire
	notices          []*pgErrorWire
	status           byte // ReadyForQuery status
	unexpectedFields []string
}

// extendedRoundTrip sends one full Parse/Bind/Describe/Execute/Sync flow
// (steps toggled by the flags) and collects the responses up to the
// ReadyForQuery of the Sync.
func (c *pgClient) extendedRoundTrip(stmtName, portalName, sql string, paramOIDs []int32, paramValues [][]byte, doDescribe, doExecute bool, executeLimit int32) *extResult {
	c.t.Helper()
	res := &extResult{}

	// Parse
	var parse msgBuf
	parse.writeCString(stmtName)
	parse.writeCString(sql)
	var oidCount [2]byte
	binary.BigEndian.PutUint16(oidCount[:], uint16(len(paramOIDs)))
	parse.Write(oidCount[:])
	for _, oid := range paramOIDs {
		var ob [4]byte
		binary.BigEndian.PutUint32(ob[:], uint32(oid))
		parse.Write(ob[:])
	}
	c.sendMessage('P', parse.Bytes())

	// Bind
	var bind msgBuf
	bind.writeCString(portalName)
	bind.writeCString(stmtName)
	var fmtCount [2]byte
	binary.BigEndian.PutUint16(fmtCount[:], 0) // all parameters in text format
	bind.Write(fmtCount[:])
	var argCount [2]byte
	binary.BigEndian.PutUint16(argCount[:], uint16(len(paramValues)))
	bind.Write(argCount[:])
	for _, v := range paramValues {
		if v == nil {
			var null [4]byte
			binary.BigEndian.PutUint32(null[:], 0xffffffff)
			bind.Write(null[:])
			continue
		}
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(v)))
		bind.Write(lenBuf[:])
		bind.Write(v)
	}
	var resFmtCount [2]byte
	binary.BigEndian.PutUint16(resFmtCount[:], 0) // all results in text format
	bind.Write(resFmtCount[:])
	c.sendMessage('B', bind.Bytes())

	// Describe (statement) — before Execute, like libpq/pgx do. Names are
	// cstrs (the parsed statement name, and the portal name).
	if doDescribe {
		var descS msgBuf
		descS.WriteByte('S')
		descS.writeCString(stmtName)
		c.sendMessage('D', descS.Bytes())
		var descP msgBuf
		descP.WriteByte('P')
		descP.writeCString(portalName)
		c.sendMessage('D', descP.Bytes())
	}

	// Execute
	if doExecute {
		var exec msgBuf
		exec.writeCString(portalName)
		var limit [4]byte
		binary.BigEndian.PutUint32(limit[:], uint32(executeLimit))
		exec.Write(limit[:])
		c.sendMessage('E', exec.Bytes())
	}

	// Sync
	c.sendMessage('S', nil)

	for {
		mtype, payload := c.readMessage()
		switch mtype {
		case '1':
			res.parseComplete = true
		case '2':
			res.bindComplete = true
		case '3':
			res.closeComplete = true
		case 't':
			if len(payload) < 2 {
				c.t.Fatalf("ParameterDescription too short")
			}
			n := int(int16(binary.BigEndian.Uint16(payload[:2])))
			for i := 0; i < n; i++ {
				res.paramOIDs = append(res.paramOIDs, int32(binary.BigEndian.Uint32(payload[2+i*4:6+i*4])))
			}
		case 'T':
			c.parseRowDescription(res, payload)
		case 'n':
			res.noData = true
		case 'D':
			if len(payload) < 2 {
				c.t.Fatalf("DataRow too short")
			}
			n := int(int16(binary.BigEndian.Uint16(payload[:2])))
			i := 2
			row := make([]*string, 0, n)
			for k := 0; k < n; k++ {
				l := int32(binary.BigEndian.Uint32(payload[i : i+4]))
				i += 4
				if l < 0 {
					row = append(row, nil)
					continue
				}
				v := string(payload[i : i+int(l)])
				i += int(l)
				row = append(row, &v)
			}
			res.rows = append(res.rows, row)
		case 'C':
			res.tags = append(res.tags, strings.TrimSuffix(string(payload), "\x00"))
		case 's':
			res.portalSuspended = true
		case 'I':
			res.emptyQuery = true
		case 'N':
			e := parseError(payload)
			if e == nil {
				c.t.Fatalf("unparseable NoticeResponse %q", payload)
			}
			res.notices = append(res.notices, e)
		case 'E':
			e := parseError(payload)
			if e == nil {
				c.t.Fatalf("unparseable ErrorResponse %q", payload)
			}
			res.err = e
		case 'Z':
			res.status = payload[0]
			return res
		default:
			res.unexpectedFields = append(res.unexpectedFields, string(rune(mtype)))
		}
	}
}

// parseRowDescription decodes a RowDescription payload into the result.
// Each RowDescription REPLACES the previous one (a round trip carries one
// per Describe and one per Execute; the last one is the executed shape).
func (c *pgClient) parseRowDescription(res *extResult, payload []byte) {
	if len(payload) < 2 {
		c.t.Fatalf("RowDescription too short")
	}
	n := int(int16(binary.BigEndian.Uint16(payload[:2])))
	i := 2
	var columns []string
	var oids []int32
	for k := 0; k < n; k++ {
		end := -1
		for j := i; j < len(payload); j++ {
			if payload[j] == 0 {
				end = j
				break
			}
		}
		if end < 0 {
			c.t.Fatalf("RowDescription unterminated name")
		}
		columns = append(columns, string(payload[i:end]))
		i = end + 1
		// table oid(4) attnum(2) type oid(4) typlen(2) typmod(4) format(2)
		oids = append(oids, int32(binary.BigEndian.Uint32(payload[i+6:i+10])))
		i += 18
	}
	res.columns = columns
	res.oids = oids
}

// terminate closes the session cleanly (the server answers nothing).
func (c *pgClient) terminate() {
	c.sendMessage('X', nil)
}

// ---------------------------------------------------------------------------
// Tests: the extended query protocol.
// ---------------------------------------------------------------------------

// TestPGGatewayExtendedProtocolRoundTrip proves the extended query
// protocol: Parse/Bind/Describe/Execute/Sync with a bound parameter
// round-trips rows, prepared statements report parameter and result types,
// a prepared DML statement mutates through a portal, out-of-order $N
// markers bind by parameter number (PostgreSQL semantics, not appearance
// order), and an unsupported statement degrades with an ErrorResponse
// followed by ReadyForQuery without dropping the connection.
func TestPGGatewayExtendedProtocolRoundTrip(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	// Seed through the simple protocol.
	if res := c.simpleQuery("CREATE TABLE pgw_ext (name TEXT, qty INTEGER)"); res.err != nil {
		t.Fatalf("CREATE TABLE: %s %s", res.err.code(), res.err.message())
	}
	if res := c.simpleQuery("INSERT INTO pgw_ext VALUES ('alpha', 1), ('beta', 2)"); res.err != nil {
		t.Fatalf("seed INSERT: %s %s", res.err.code(), res.err.message())
	}

	// Parameterized SELECT: Parse with a declared INT4 parameter, Bind a
	// text value, Describe the portal, Execute, Sync.
	res := c.extendedRoundTrip("s1", "p1", "SELECT name, qty FROM pgw_ext WHERE qty >= $1",
		[]int32{23}, [][]byte{[]byte("1")}, true, true, 0)
	if res.err != nil {
		t.Fatalf("extended SELECT failed: %s %s (parseComplete=%v bindComplete=%v cols=%v rows=%v tags=%v)",
			res.err.code(), res.err.message(), res.parseComplete, res.bindComplete, res.columns, res.rows, res.tags)
	}
	if !res.parseComplete || !res.bindComplete {
		t.Fatalf("expected ParseComplete/BindComplete: %+v", res)
	}
	if len(res.columns) != 2 || res.columns[0] != "name" || res.columns[1] != "qty" {
		t.Fatalf("columns: %v", res.columns)
	}
	// Declared column types (TEXT/INTEGER), not first-row inference.
	if len(res.oids) != 2 || res.oids[0] != 25 || res.oids[1] != 23 {
		t.Fatalf("reported OIDs: %v", res.oids)
	}
	if len(res.rows) != 2 || res.rows[0][0] == nil || *res.rows[0][0] != "alpha" ||
		res.rows[1][0] == nil || *res.rows[1][0] != "beta" {
		t.Fatalf("rows: %v", res.rows)
	}
	if len(res.tags) != 1 || res.tags[0] != "SELECT 2" {
		t.Fatalf("tags: %v", res.tags)
	}
	if res.status != 'I' {
		t.Fatalf("expected idle ReadyForQuery after Sync, got %q", res.status)
	}

	// Describe (statement) reports the parameter types and the result
	// shape — without executing (a second Parse under a different name).
	res = c.extendedRoundTrip("s2", "", "SELECT name, qty FROM pgw_ext WHERE qty >= $1",
		[]int32{23}, nil, true, false, 0)
	if res.err != nil {
		t.Fatalf("extended Describe failed: %s %s", res.err.code(), res.err.message())
	}
	if !res.parseComplete {
		t.Fatalf("expected ParseComplete: %+v", res)
	}
	if len(res.paramOIDs) != 1 || res.paramOIDs[0] != 23 {
		t.Fatalf("ParameterDescription OIDs: %v", res.paramOIDs)
	}
	if len(res.oids) != 2 || res.oids[0] != 25 || res.oids[1] != 23 {
		t.Fatalf("Describe reported OIDs: %v", res.oids)
	}
	// The described SELECT never ran (Describe does not execute): the
	// Count check below runs after the Execute test.
	res = c.extendedRoundTrip("s2", "p2", "SELECT name, qty FROM pgw_ext WHERE qty >= $1",
		[]int32{23}, [][]byte{[]byte("2")}, false, true, 0)
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "beta" {
		t.Fatalf("parameterized execute: err=%v rows=%v", res.err, res.rows)
	}

	// Prepared DML through a portal.
	res = c.extendedRoundTrip("ins", "pins", "INSERT INTO pgw_ext VALUES ($1, $2)",
		[]int32{25, 23}, [][]byte{[]byte("gamma"), []byte("3")}, true, true, 0)
	if res.err != nil {
		t.Fatalf("prepared INSERT failed: %s %s", res.err.code(), res.err.message())
	}
	if !res.noData {
		t.Fatalf("INSERT portal must report NoData on Describe, got columns %v rows %v", res.columns, res.rows)
	}
	if len(res.tags) != 1 || res.tags[0] != "INSERT 0 1" {
		t.Fatalf("INSERT tag: %v", res.tags)
	}

	// Out-of-order markers: $2 arrives before $1 in the statement text;
	// PostgreSQL semantics bind by NUMBER (b=$1=2, name=$2='beta'), not
	// by appearance order.
	res = c.extendedRoundTrip("s3", "p3", "SELECT name FROM pgw_ext WHERE name = $2 AND qty = $1",
		[]int32{23, 25}, [][]byte{[]byte("2"), []byte("beta")}, true, true, 0)
	if res.err != nil {
		t.Fatalf("out-of-order bind failed: %s %s", res.err.code(), res.err.message())
	}
	if len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "beta" {
		t.Fatalf("out-of-order rows: %v (values must bind by parameter number)", res.rows)
	}

	// NULL first row: the declared type is still reported (no
	// oidUnknown 705 from first-row inference).
	if res := c.simpleQuery("INSERT INTO pgw_ext VALUES ('nul', NULL)"); res.err != nil {
		t.Fatalf("NULL seed: %v", res.err)
	}
	res = c.extendedRoundTrip("s4", "p4", "SELECT name, qty FROM pgw_ext WHERE name = 'nul'",
		nil, nil, true, true, 0)
	if res.err != nil {
		t.Fatalf("NULL first row query failed: %s %s", res.err.code(), res.err.message())
	}
	if len(res.oids) != 2 || res.oids[0] != 25 || res.oids[1] != 23 {
		t.Fatalf("NULL-first-row OIDs must use the declared types: %v", res.oids)
	}
	if len(res.rows) != 1 || res.rows[0][1] != nil {
		t.Fatalf("NULL row: %v", res.rows)
	}

	// An unsupported (unclassified) statement degrades: ErrorResponse at
	// Parse, ReadyForQuery at Sync, and the session stays usable.
	res = c.extendedRoundTrip("bad", "", "CHECKPOINT", nil, nil, false, false, 0)
	if res.parseComplete || res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 at Parse, got parseComplete=%v err=%v", res.parseComplete, res.err)
	}
	if res.status != 'I' {
		t.Fatalf("expected ReadyForQuery after the refusal, got %q", res.status)
	}
	// The connection still works.
	res = c.extendedRoundTrip("ok", "okp", "SELECT 1", nil, nil, true, true, 0)
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "1" {
		t.Fatalf("session did not survive the refusal: err=%v rows=%v", res.err, res.rows)
	}
	c.terminate()
}

// TestPGGatewayExtendedDescribeDoesNotExecute proves Describe of a
// mutating prepared statement never executes it: an INSERT..RETURNING can
// be Parsed/Described repeatedly with no row appearing until the portal is
// actually Executed.
func TestPGGatewayExtendedDescribeDoesNotExecute(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	if res := c.simpleQuery("CREATE TABLE pgw_desc (v TEXT)"); res.err != nil {
		t.Fatalf("CREATE TABLE: %v", res.err)
	}

	// Parse + Describe (twice, as pgx does when caching statements) + Sync.
	for i := 0; i < 2; i++ {
		res := c.extendedRoundTrip("ins", "", "INSERT INTO pgw_desc VALUES ($1) RETURNING v",
			[]int32{25}, nil, true, false, 0)
		if res.err != nil {
			t.Fatalf("Describe round %d failed: %s %s", i, res.err.code(), res.err.message())
		}
		if !res.noData && len(res.oids) == 0 {
			t.Fatalf("expected NoData or a RowDescription for RETURNING, got %+v", res)
		}
		if len(res.tags) != 0 || len(res.rows) != 0 {
			t.Fatalf("Describe executed the statement: tags=%v rows=%v", res.tags, res.rows)
		}
	}

	// Nothing was executed by the Describes.
	handle, err := fx.rt.Handles.Open(context.Background(), fx.target1)
	if err != nil {
		t.Fatalf("open handle: %v", err)
	}
	var count int
	if err := handle.QueryRowContext(context.Background(), "SELECT count(*) FROM pgw_desc").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("Describe executed the INSERT: %d rows exist", count)
	}

	// The actual Execute mutates exactly once.
	res := c.extendedRoundTrip("ins", "p", "INSERT INTO pgw_desc VALUES ($1) RETURNING v",
		[]int32{25}, [][]byte{[]byte("once")}, false, true, 0)
	if res.err != nil {
		t.Fatalf("Execute failed: %s %s", res.err.code(), res.err.message())
	}
	if len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "once" {
		t.Fatalf("RETURNING rows: %v", res.rows)
	}
	// PostgreSQL reports INSERT with RETURNING as "INSERT 0 <inserted>".
	if len(res.tags) != 1 || res.tags[0] != "INSERT 0 1" {
		t.Fatalf("RETURNING tag: %v", res.tags)
	}
	if err := handle.QueryRowContext(context.Background(), "SELECT count(*) FROM pgw_desc").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 row after Execute, got %d", count)
	}
	c.terminate()
}

// TestPGGatewayDMLGeneratesCDC proves native DML through the PostgreSQL
// gateway generates CDC: _mb_changes/_mb_change_rows frames with source
// 'pg', the affected table (bare name) and the action — for the simple
// protocol, the extended protocol and DML-with-RETURNING — with capture
// and data atomic per statement.
func TestPGGatewayDMLGeneratesCDC(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	if res := c.simpleQuery("CREATE TABLE pgw_cdc (id TEXT PRIMARY KEY, n INTEGER)"); res.err != nil {
		t.Fatalf("CREATE TABLE: %v", res.err)
	}

	// Simple-protocol INSERT.
	if res := c.simpleQuery("INSERT INTO pgw_cdc VALUES ('r1', 1)"); res.err != nil || len(res.tags) != 1 || res.tags[0] != "INSERT 0 1" {
		t.Fatalf("simple INSERT: err=%v tags=%v", res.err, res.tags)
	}
	// Extended-protocol INSERT (prepared statement).
	res := c.extendedRoundTrip("i", "p", "INSERT INTO pgw_cdc VALUES ($1, $2)",
		[]int32{25, 23}, [][]byte{[]byte("r2"), []byte("2")}, true, true, 0)
	if res.err != nil {
		t.Fatalf("extended INSERT: %s %s", res.err.code(), res.err.message())
	}
	// DML with RETURNING (simple path).
	if res := c.simpleQuery("UPDATE pgw_cdc SET n = 10 WHERE id = 'r1' RETURNING id"); res.err != nil || len(res.rows) != 1 {
		t.Fatalf("UPDATE RETURNING: err=%v rows=%v", res.err, res.rows)
	}
	// DELETE.
	if res := c.simpleQuery("DELETE FROM pgw_cdc WHERE id = 'r2'"); res.err != nil {
		t.Fatalf("DELETE: %v", res.err)
	}

	handle, err := fx.rt.Handles.Open(context.Background(), fx.target1)
	if err != nil {
		t.Fatalf("open handle: %v", err)
	}
	rows, err := handle.QueryContext(context.Background(),
		`SELECT c.source, r.table_name, r.record_id, r.action
		 FROM _mb_changes c JOIN _mb_change_rows r ON r.commit_seq = c.commit_seq
		 WHERE r.table_name = 'pgw_cdc' ORDER BY c.commit_seq, r.ordinal`)
	if err != nil {
		t.Fatalf("journal query: %v", err)
	}
	type frame struct{ source, table, recordID, action string }
	var frames []frame
	for rows.Next() {
		var f frame
		if err := rows.Scan(&f.source, &f.table, &f.recordID, &f.action); err != nil {
			rows.Close()
			t.Fatalf("scan: %v", err)
		}
		frames = append(frames, f)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	want := []frame{
		{source: "pg", table: "pgw_cdc", action: "create"}, // simple INSERT (record id resolved)
		{source: "pg", table: "pgw_cdc", action: "create"}, // extended INSERT
		{source: "pg", table: "pgw_cdc", action: "update"}, // UPDATE RETURNING
		{source: "pg", table: "pgw_cdc", action: "delete"}, // DELETE
	}
	if len(frames) != len(want) {
		t.Fatalf("expected %d CDC frames, got %d: %+v", len(want), len(frames), frames)
	}
	for i, w := range want {
		if frames[i].source != w.source || frames[i].table != w.table || frames[i].action != w.action {
			t.Fatalf("frame %d: got %+v want %+v", i, frames[i], w)
		}
	}
	// The single-row INSERT resolves its record id through the engine's
	// last-insert identity.
	if frames[0].recordID == "" {
		t.Fatalf("the INSERT frame carries no record id: %+v", frames[0])
	}
	// UPDATE/DELETE frames keep the table-scoped shape (empty record id is
	// acceptable); the atomicity invariant: every 'pg' frame's capture and
	// its data commit in the same transaction, so the journal cannot
	// reference data that never landed — the UPDATE frame's row exists.
	var n int
	if err := handle.QueryRowContext(context.Background(),
		"SELECT count(*) FROM pgw_cdc WHERE id = 'r1' AND n = 10").Scan(&n); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if n != 1 {
		t.Fatalf("CDC data verification failed: %d", n)
	}
	c.terminate()
}

// TestPGGatewayRefusesCopyForReadOnly proves the COPY policy: server
// filesystem COPY (TO/FROM a path) is always refused, COPY is refused for
// sql.read-only credentials with 42501, every refusal degrades with an
// ErrorResponse followed by ReadyForQuery (never a hang or a drop), and a
// stray COPY-data message without a COPY exchange fails safely.
func TestPGGatewayRefusesCopyForReadOnly(t *testing.T) {
	requireEngine(t)
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()
	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}
	d, err := rt.Service.ProvisionManual(context.Background(), app.DataDir(), "pgw-copy", "wal", "")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	cred, err := rt.IssueCredential(context.Background(), d.ID, "pg", []string{"sql.read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	server := pgateway.New(rt, pgateway.Config{Addr: "127.0.0.1:0", TLS: testTLSConfig(t)},
		debugTestLogger())
	ln, err := server.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close(); server.Close() }()

	c := dialPG(t, ln.Addr().String())
	c.sendStartup(cred.ID, d.ID)
	if _, status := c.scramAuthenticate(cred.ID, cred.Secret); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	// A read-only credential may read.
	if res := c.simpleQuery("SELECT 1"); res.err != nil {
		t.Fatalf("SELECT: %v", res.err)
	}
	// COPY TO STDOUT for a read-only credential: refused, session alive.
	res := c.simpleQuery("COPY pgw_copy TO STDOUT")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for COPY TO, got %+v", res.err)
	}
	if res.status != 'I' {
		t.Fatalf("expected ReadyForQuery 'I' after the COPY refusal, got %q", res.status)
	}
	// COPY FROM a server path: refused on every credential shape.
	res = c.simpleQuery("COPY pgw_copy FROM '/etc/passwd'")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for COPY FROM a path, got %+v", res.err)
	}
	// COPY FROM STDIN: fails safely (no hang, no protocol desync).
	res = c.simpleQuery("COPY pgw_copy FROM STDIN")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for COPY FROM STDIN, got %+v", res.err)
	}
	// The session survived every refusal.
	if res := c.simpleQuery("SELECT 2"); res.err != nil || len(res.rows) != 1 || *res.rows[0][0] != "2" {
		t.Fatalf("session did not survive COPY refusals: %v", res.err)
	}

	// Extended protocol: COPY is refused at Parse (never a prepared
	// statement), the error degrades with ReadyForQuery.
	extRes := c.extendedRoundTrip("cp", "", "COPY pgw_copy TO STDOUT", nil, nil, false, false, 0)
	if extRes.parseComplete || extRes.err == nil || extRes.err.code() != "42501" {
		t.Fatalf("expected 42501 at Parse for COPY, got parseComplete=%v err=%v", extRes.parseComplete, extRes.err)
	}
	if extRes.status != 'I' {
		t.Fatalf("expected ReadyForQuery after the COPY Parse refusal, got %q", extRes.status)
	}
	// Stray COPY-data ('d') without a CopyInResponse: one error, skip
	// until Sync, ReadyForQuery — never a hang.
	c.sendMessage('d', []byte("copy-data\n"))
	c.sendMessage('S', nil) // the skip state ends at Sync
	derr := readUntilSyncRFQ(t, c)
	if derr == nil {
		t.Fatalf("expected an ErrorResponse for stray COPY data")
	}
	// The session is still aligned and usable.
	extRes = c.extendedRoundTrip("ok", "okp", "SELECT 3", nil, nil, true, true, 0)
	if extRes.err != nil || len(extRes.rows) != 1 || *extRes.rows[0][0] != "3" {
		t.Fatalf("session did not survive the stray COPY data: err=%v rows=%v", extRes.err, extRes.rows)
	}
	c.terminate()
}

// readUntilSyncRFQ reads messages until ReadyForQuery and returns the
// first ErrorResponse seen (tests the skip-until-Sync recovery).
func readUntilSyncRFQ(t *testing.T, c *pgClient) *pgErrorWire {
	t.Helper()
	var firstErr *pgErrorWire
	for {
		mtype, payload := c.readMessage()
		switch mtype {
		case 'E':
			if firstErr == nil {
				firstErr = parseError(payload)
			}
		case 'N':
		case 'Z':
			if firstErr != nil && firstErr.code() != "08P01" {
				t.Fatalf("expected 08P01 protocol violation for stray COPY data, got %s", firstErr.code())
			}
			return firstErr
		default:
			t.Fatalf("unexpected message %q during skip recovery", string(rune(mtype)))
		}
	}
}

// TestPGGatewaySessionStateIsolation proves the session-state isolation:
// SET/RESET/SHOW are implemented by the gateway (per session), a session's
// settings never leak into another session, and temporary engine objects
// never leak through the pooled connection to a later session.
func TestPGGatewaySessionStateIsolation(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	// Session A: set custom parameters, read them back, create a
	// temporary table, then disconnect.
	a := dialPG(t, fx.addr)
	a.sendStartup(user, fx.target1)
	if _, status := a.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("A: expected idle ReadyForQuery, got %q", status)
	}
	res := a.simpleQuery("SET pgw_custom = 'leaked'")
	if res.err != nil || len(res.tags) != 1 || res.tags[0] != "SET" {
		t.Fatalf("A SET: err=%v tags=%v", res.err, res.tags)
	}
	res = a.simpleQuery("SHOW pgw_custom")
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "leaked" {
		t.Fatalf("A SHOW: err=%v rows=%v", res.err, res.rows)
	}
	if len(res.columns) != 1 || res.columns[0] != "pgw_custom" {
		t.Fatalf("A SHOW columns: %v", res.columns)
	}
	res = a.simpleQuery("CREATE TEMP TABLE pgw_leak (x INTEGER)")
	if res.err != nil {
		t.Fatalf("A CREATE TEMP TABLE: %s %s", res.err.code(), res.err.message())
	}
	res = a.simpleQuery("INSERT INTO pgw_leak VALUES (7)")
	if res.err != nil {
		t.Fatalf("A temp INSERT: %v", res.err)
	}
	res = a.simpleQuery("SELECT x FROM pgw_leak")
	if res.err != nil || len(res.rows) != 1 || *res.rows[0][0] != "7" {
		t.Fatalf("A temp SELECT: err=%v rows=%v", res.err, res.rows)
	}
	a.terminate()

	// Session B (same credential): the gateway settings of A are gone and
	// the temporary table is not visible.
	b := dialPG(t, fx.addr)
	b.sendStartup(user, fx.target1)
	if _, status := b.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("B: expected idle ReadyForQuery, got %q", status)
	}
	res = b.simpleQuery("SHOW pgw_custom")
	if res.err == nil || res.err.code() != "42704" {
		t.Fatalf("B SHOW must fail with 42704 (A's setting leaked: %+v rows=%v)", res.err, res.rows)
	}
	res = b.simpleQuery("SELECT * FROM pgw_leak")
	if res.err == nil || res.err.code() != "42P01" {
		t.Fatalf("B temp-table SELECT must fail with 42P01 (A's temp table leaked: %+v rows=%v)", res.err, res.rows)
	}
	// SHOW ALL reports the gateway's parameter set (shape only).
	res = b.simpleQuery("SHOW ALL")
	if res.err != nil || len(res.columns) != 3 {
		t.Fatalf("B SHOW ALL: err=%v columns=%v", res.err, res.columns)
	}
	// RESET ALL and the transaction scope: SET LOCAL outside a block is
	// refused; inside a block it is scoped and reverts at COMMIT.
	res = b.simpleQuery("SET LOCAL pgw_local = 'x'")
	if res.err == nil || res.err.code() != "25P01" {
		t.Fatalf("SET LOCAL outside a block must fail with 25P01, got %+v", res.err)
	}
	res = b.simpleQuery("BEGIN")
	if res.err != nil || res.status != 'T' {
		t.Fatalf("BEGIN: err=%v status=%q", res.err, res.status)
	}
	res = b.simpleQuery("SET LOCAL pgw_local = 'y'")
	if res.err != nil {
		t.Fatalf("SET LOCAL in block: %v", res.err)
	}
	res = b.simpleQuery("SHOW pgw_local")
	if res.err != nil || len(res.rows) != 1 || *res.rows[0][0] != "y" {
		t.Fatalf("SHOW local in block: err=%v rows=%v", res.err, res.rows)
	}
	res = b.simpleQuery("COMMIT")
	if res.err != nil || res.status != 'I' {
		t.Fatalf("COMMIT: err=%v status=%q", res.err, res.status)
	}
	res = b.simpleQuery("SHOW pgw_local")
	if res.err == nil || res.err.code() != "42704" {
		t.Fatalf("SET LOCAL must revert at COMMIT: err=%v rows=%v", res.err, res.rows)
	}
	// RESET ALL clears a session override.
	res = b.simpleQuery("SET pgw_custom2 = 'z'")
	if res.err != nil {
		t.Fatalf("SET: %v", res.err)
	}
	res = b.simpleQuery("RESET ALL")
	if res.err != nil || len(res.tags) != 1 || res.tags[0] != "RESET" {
		t.Fatalf("RESET ALL: err=%v tags=%v", res.err, res.tags)
	}
	res = b.simpleQuery("SHOW pgw_custom2")
	if res.err == nil || res.err.code() != "42704" {
		t.Fatalf("RESET ALL must clear the override: err=%v rows=%v", res.err, res.rows)
	}
	// The default parameter set still reads back.
	res = b.simpleQuery("SHOW TimeZone")
	if res.err != nil || len(res.rows) != 1 || *res.rows[0][0] != "UTC" {
		t.Fatalf("SHOW TimeZone: err=%v rows=%v", res.err, res.rows)
	}
	b.terminate()
}

// TestPGGatewayDeniesUnclassifiedStatements proves the parsed enforcement
// default: statements whose leading keyword is not a classified family are
// REFUSED (42501), for write-scoped credentials too — the engine is never
// consulted for them.
func TestPGGatewayDeniesUnclassifiedStatements(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	for _, q := range []string{
		"CHECKPOINT",
		"REFRESH MATERIALIZED VIEW somewhere",
		"LISTEN channel",
		"NOTIFY channel",
		"LOCK TABLE pgw_items",
		"CALL some_proc()",
		"DISCARD ALL",
	} {
		res := c.simpleQuery(q)
		if res.err == nil || res.err.code() != "42501" {
			t.Fatalf("expected 42501 for %q, got %+v", q, res.err)
		}
		if res.status != 'I' {
			t.Fatalf("expected ReadyForQuery after refusing %q, got %q", q, res.status)
		}
	}
	// Session-state statements remain available (and never refuse).
	if res := c.simpleQuery("SET statement_timeout = 1000"); res.err != nil {
		t.Fatalf("SET must remain available: %v", res.err)
	}
	if res := c.simpleQuery("RESET statement_timeout"); res.err != nil {
		t.Fatalf("RESET must remain available: %v", res.err)
	}
	// The session is alive and the engine was never consulted for the
	// denied statements (a SELECT proves alignment).
	if res := c.simpleQuery("SELECT 4"); res.err != nil || len(res.rows) != 1 || *res.rows[0][0] != "4" {
		t.Fatalf("session did not survive the denials: %v", res.err)
	}
	c.terminate()
}

// TestPGGatewayPrimaryPolicyEpochBoundary proves the generation/policy
// epoch checks apply to PRIMARY-alias credentials too: a live primary
// session is terminated at the next transaction boundary when the
// credential's recorded policy epoch changes (a re-issued credential row
// never keeps authenticating old sessions).
func TestPGGatewayPrimaryPolicyEpochBoundary(t *testing.T) {
	requireEngine(t)
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()
	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}
	cred, err := rt.IssueCredential(context.Background(), "", "pg", []string{"sql.read", "sql.write"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue primary credential: %v", err)
	}
	server := pgateway.New(rt, pgateway.Config{Addr: "127.0.0.1:0", TLS: testTLSConfig(t)},
		debugTestLogger())
	ln, err := server.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close(); server.Close() }()

	c := dialPG(t, ln.Addr().String())
	c.sendStartup(cred.ID, "primary")
	if _, status := c.scramAuthenticate(cred.ID, cred.Secret); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	// The primary alias accepts administrative reads.
	if res := c.simpleQuery("SELECT 5"); res.err != nil || len(res.rows) != 1 || *res.rows[0][0] != "5" {
		t.Fatalf("primary SELECT: err=%v rows=%v", res.err, res.rows)
	}

	// Bump the credential's recorded policy epoch (simulating a re-issue
	// with a rotated policy). The live session must be terminated at the
	// next transaction boundary (BEGIN), with 28000 — the same enforcement
	// database-scoped credentials already get.
	if _, err := rt.PrimaryDB.ExecContext(context.Background(),
		`UPDATE _mb_credentials SET policy_epoch = 2 WHERE id = ?`, cred.ID); err != nil {
		t.Fatalf("epoch bump: %v", err)
	}
	res := c.simpleQuery("BEGIN")
	if res.err == nil || res.err.code() != "28000" {
		t.Fatalf("expected 28000 termination at the boundary after the epoch change, got %+v", res.err)
	}
}

// TestPGGatewayNotices proves NoticeResponse traffic reaches the wire:
// COMMIT outside a transaction block answers with a 25P01 warning notice
// followed by the CommandComplete (PostgreSQL behavior), and a Bind that
// requests binary results degrades to text with a notice while the rows
// still arrive.
func TestPGGatewayNotices(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	if res := c.simpleQuery("CREATE TABLE pgw_notice (v TEXT)"); res.err != nil {
		t.Fatalf("CREATE TABLE: %v", res.err)
	}

	// COMMIT with no open transaction block: warning notice + COMMIT tag.
	c.sendMessage('Q', append([]byte("COMMIT"), 0))
	var sawNotice *pgErrorWire
	var tag string
	var status byte
	for {
		mtype, payload := c.readMessage()
		switch mtype {
		case 'N':
			e := parseError(payload)
			if e == nil {
				t.Fatalf("unparseable NoticeResponse %q", payload)
			}
			sawNotice = e
		case 'C':
			tag = strings.TrimSuffix(string(payload), "\x00")
		case 'Z':
			status = payload[0]
			goto commitDone
		default:
			t.Fatalf("unexpected message %q in COMMIT response", string(rune(mtype)))
		}
	}
commitDone:
	if sawNotice == nil {
		t.Fatalf("expected a NoticeResponse for COMMIT outside a transaction block")
	}
	if sawNotice.code() != "25P01" || !strings.Contains(sawNotice.message(), "no transaction in progress") {
		t.Fatalf("unexpected notice: %+v", sawNotice.Fields)
	}
	if sawNotice.Fields['S'] == "ERROR" {
		t.Fatalf("the notice must not be ERROR severity: %+v", sawNotice.Fields)
	}
	if tag != "COMMIT" || status != 'I' {
		t.Fatalf("expected COMMIT tag and idle status, got tag=%q status=%q", tag, status)
	}

	// Binary result format request: the values still arrive (text) with a
	// notice explaining the degradation.
	var parse msgBuf
	parse.writeCString("s")
	parse.writeCString("SELECT v FROM pgw_notice WHERE v IS NULL")
	var noOids [2]byte
	parse.Write(noOids[:])
	c.sendMessage('P', parse.Bytes())
	var bind msgBuf
	bind.writeCString("p")
	bind.writeCString("s")
	var zeroFmt [2]byte
	bind.Write(zeroFmt[:]) // parameter formats: none
	bind.Write(zeroFmt[:]) // parameter count: 0
	var oneFmt [2]byte
	binary.BigEndian.PutUint16(oneFmt[:], 1)
	bind.Write(oneFmt[:]) // one result format
	var binFmt [2]byte
	binary.BigEndian.PutUint16(binFmt[:], 1)
	bind.Write(binFmt[:]) // binary results requested
	c.sendMessage('B', bind.Bytes())
	var exec msgBuf
	exec.writeCString("p")
	exec.Write([]byte{0, 0, 0, 0}) // row limit 0
	c.sendMessage('E', exec.Bytes())
	c.sendMessage('S', nil)

	sawNotice = nil
	tag = ""
	status = 0
	for {
		mtype, payload := c.readMessage()
		switch mtype {
		case 'N':
			e := parseError(payload)
			if e == nil {
				t.Fatalf("unparseable NoticeResponse %q", payload)
			}
			sawNotice = e
		case 'C':
			tag = strings.TrimSuffix(string(payload), "\x00")
		case 'Z':
			status = payload[0]
			goto bindDone
		default:
		}
	}
bindDone:
	if sawNotice == nil {
		t.Fatalf("expected a NoticeResponse for the binary result format request")
	}
	if !strings.Contains(sawNotice.message(), "binary result format") {
		t.Fatalf("unexpected notice: %+v", sawNotice.Fields)
	}
	if tag != "SELECT 0" || status != 'I' {
		t.Fatalf("expected the rows to still arrive as text: tag=%q status=%q", tag, status)
	}
	c.terminate()
}
