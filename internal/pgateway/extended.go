package pgateway

// The Extended Query protocol (postgresql-access.md wire qualification:
// "Extended Parse/Bind/Describe/Execute/Sync, prepared statements,
// parameter types, text/binary values"). The state machine mirrors the
// protocol: Parse/Bind/Describe/Execute/Close/Flush queue their
// responses; Sync flushes the queue, restores the error state and answers
// ReadyForQuery. An error inside the extended phase sets the skip state —
// every further extended message is ignored until Sync (Sync answers
// ReadyForQuery with the failed/idle status), so the connection never
// desyncs and an unsupported piece degrades with an ErrorResponse followed
// by ReadyForQuery, never by dropping the connection. Cancellation reaches
// a running Execute through the same per-batch cancel path the simple
// protocol uses.
//
// Executed statements run through the SAME parsed enforcement and CDC
// capture as the simple protocol: classification at Parse (a refused
// statement never becomes a prepared statement), DML frames inside the
// transaction that commits them.
//
// Describe never executes: the result shape is resolved from the engine's
// prepare step (the driver binds without stepping, so reading rows.Columns()
// executes nothing) with the DECLARED column types, matching the pinned
// upstream's prepare-only do_describe_statement/do_describe_portal.

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"mirabase/internal/application"
)

// preparedStmt is one Parse-produced statement.
type preparedStmt struct {
	sql       string // engine SQL (PostgreSQL $N markers rewritten to ?N)
	stripped  string
	paramOIDs []int32
	cls       stmtClass
	returns   bool
	empty     bool // empty statement text (EmptyQueryResponse on Execute)

	// session carries the gateway session-state shape (SET/RESET/SHOW);
	// these statements never reach the engine (sessionstate.go).
	session sessStmt

	// Result shape, resolved ONCE from the engine's prepare step with the
	// declared column types (row-independent; expressions and ambiguous
	// columns report TEXT, the pinned frontend's undetermined-column
	// default).
	shapeKnown bool
	colNames   []string
	oids       []int32
}

// suspendedPortal holds an Execute's partially consumed row stream: the
// open *sql.Rows, the rows already delivered and (when the row limit was
// hit) the peeked row that proved more data exists — it is delivered
// first on the resuming Execute.
type suspendedPortal struct {
	rows     *sql.Rows
	count    int
	buffered []any
}

// portal is one Bind-produced portal.
type portal struct {
	stmt      *preparedStmt
	args      []any
	binaryRes bool // the Bind requested binary result formats
	noticed   bool // the binary-result notice was sent
	suspended *suspendedPortal
}

// extendedInit allocates the extended-protocol state.
func (s *session) extendedInit() {
	s.stmts = map[string]*preparedStmt{}
	s.portals = map[string]*portal{}
}

// portalRowsOpen reports whether any portal holds a suspended result —
// the engine connection stays held while rows stream across Syncs.
func (s *session) portalRowsOpen() bool {
	for _, p := range s.portals {
		if p.suspended != nil {
			return true
		}
	}
	return false
}

// closePortal releases one portal (an Execute row stream still open on
// the engine is closed with it).
func (s *session) closePortal(name string) {
	if p, ok := s.portals[name]; ok {
		if p.suspended != nil {
			_ = p.suspended.rows.Close()
		}
		delete(s.portals, name)
	}
}

// closeStatement releases one named statement and every portal bound to
// it (PostgreSQL drops portals depending on the statement).
func (s *session) closeStatement(name string) {
	if ps, ok := s.stmts[name]; ok {
		for pname, p := range s.portals {
			if p.stmt == ps {
				s.closePortal(pname)
			}
		}
		delete(s.stmts, name)
	}
}

// handleExtended processes one extended-protocol message (Parse, Bind,
// Describe, Execute, Flush, Sync, Close). Every failure inside the phase
// answers one ErrorResponse and enters the skip state (messages are
// ignored until Sync; the connection never desyncs). A write failure is
// the only transport-fatal outcome.
func (s *session) handleExtended(mtype byte) error {
	if s.skipUntilSync && mtype != msgSync {
		// Error state: consume the message without processing it (the
		// protocol's skip-until-Sync rule).
		if err := s.discardExtendedPayload(); err != nil {
			return err
		}
		return nil
	}

	switch mtype {
	case msgParse:
		return s.handleParse()
	case msgBind:
		return s.handleBind()
	case msgDescribe:
		return s.handleDescribe()
	case msgExecute:
		return s.handleExecute()
	case msgCloseStmt:
		return s.handleClose()
	case msgFlush:
		// Responses are written unbuffered as produced; Flush is a no-op.
		return s.discardExtendedPayload()
	case msgSync:
		if err := s.discardExtendedPayload(); err != nil {
			return err
		}
		s.skipUntilSync = false
		// Portals survive Sync; statements and portals are the session's
		// own state. Release the engine connection when nothing streams.
		if !s.portalRowsOpen() {
			s.releaseConn()
		}
		return s.writeReadyStatus()
	case 'd', 'c', 'f':
		// COPY-data/Done/Fail without a CopyInResponse: COPY is refused at
		// Parse, so these messages are a protocol violation. Fail safely
		// (one error, skip until Sync — never hang).
		if err := s.discardExtendedPayload(); err != nil {
			return err
		}
		s.writeError(newPGError(stateProtocolViolation,
			"COPY data received without a COPY exchange; COPY is refused by this gateway"))
		s.skipUntilSync = true
		return nil
	default:
		s.writeError(errProtocol("unknown message type %q", string(rune(mtype))))
		return errProtocol("unknown message type %q", string(rune(mtype)))
	}
}

// readExtendedPayload reads one framed message payload.
func (s *session) readExtendedPayload() ([]byte, error) {
	n, err := readMessageLength(s.br, maxRegularMessageLen)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, n-4)
	if _, err := io.ReadFull(s.br, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// discardExtendedPayload consumes one framed message payload.
func (s *session) discardExtendedPayload() error {
	n, err := readMessageLength(s.br, maxRegularMessageLen)
	if err != nil {
		return err
	}
	if _, err := io.CopyN(io.Discard, s.br, int64(n-4)); err != nil {
		return err
	}
	return nil
}

// extendedError writes one ErrorResponse and enters the skip state.
func (s *session) extendedError(err error) {
	s.skipUntilSync = true
	if pe, ok := err.(*PGError); ok {
		s.writeError(pe)
		return
	}
	s.writeError(newPGError(stateInternalError, err.Error()))
}

// handleParse processes Parse: statement name, query text, parameter
// OIDs. The statement is classified through the SAME parsed enforcement
// as the simple protocol BEFORE it is stored (a refused statement never
// becomes a prepared statement) and compile-checked against the engine.
func (s *session) handleParse() error {
	payload, err := s.readExtendedPayload()
	if err != nil {
		return err
	}
	name, err := readCStrBytes(&payload)
	if err != nil {
		return err
	}
	query, err := readCStrBytes(&payload)
	if err != nil {
		return err
	}
	if len(payload) < 2 {
		return errProtocol("Parse without a parameter count")
	}
	n := int(int16(binary.BigEndian.Uint16(payload[:2])))
	payload = payload[2:]
	if n < 0 || n > 65535 || len(payload) < 4*n {
		return errProtocol("invalid Parse parameter list")
	}
	oids := make([]int32, n)
	for i := 0; i < n; i++ {
		oids[i] = int32(binary.BigEndian.Uint32(payload[i*4 : i*4+4]))
	}

	// Split and enforce: a Parse carries exactly one statement (the
	// extended protocol's unit); multi-statement strings are refused.
	stmts, serr := splitStatements(query)
	if serr != nil {
		s.extendedError(newPGError(stateSyntaxError, serr.Error()))
		return nil
	}
	if len(stmts) > 1 {
		s.extendedError(newPGError(stateSyntaxError, "cannot insert multiple commands into a prepared statement"))
		return nil
	}
	st := pgStatement{}
	if len(stmts) == 1 {
		st = stmts[0]
	}
	cls, returnsRows, cerr := classifyStatement(st, s.writeScope, s.managed)
	if cerr != nil {
		s.extendedError(cerr)
		return nil
	}

	// PostgreSQL parameter markers ($N) are rewritten to the engine's
	// explicitly numbered markers (?N): the pinned engine numbers
	// $-prefixed markers by appearance order (SQLite semantics) while
	// PostgreSQL binds $N to parameter N (see pgParameterSQL).
	ps := &preparedStmt{
		sql:       pgParameterSQL(query, st.stripped),
		stripped:  st.stripped,
		paramOIDs: oids,
		cls:       cls,
		returns:   returnsRows,
		empty:     len(stmts) == 0,
	}
	if toks := tokens(ps.stripped); len(toks) > 0 && sessionKeywords[toks[0].upper] {
		// Gateway session-state statement: its behavior is implemented by
		// the gateway (never the engine), so the shape is static and no
		// engine compile check applies (the engine dialect cannot parse
		// SET/RESET/SHOW).
		ss, perr := parseSessionStatement(st.text)
		if perr != nil {
			s.extendedError(perr)
			return nil
		}
		ps.session = ss
		} else if !ps.empty {
			// Compile-check against the engine (an unprepareable statement
			// fails at Parse, before any Bind/Execute — the protocol's error
			// placement). Transaction control skips the check: BEGIN/COMMIT/
			// ROLLBACK are engine-parseable trivially and some engines refuse
			// EXPLAIN on them, while clients (pgx) drive transactions through
			// the extended protocol.
			//
			// The parameter count is ps.paramCount(), NOT len(oids): a client
			// may declare zero parameter types (PostgreSQL infers them from
			// context; node-postgres always sends none) while the statement
			// still carries $N markers — binding nothing to a prepared
			// statement fails the engine's bind with "got 0 args, want N".
			if cls != classTxControl {
				if cerr := s.compileCheck(ps.sql, ps.paramCount()); cerr != nil {
				if pe, ok := cerr.(*PGError); ok {
					s.extendedError(pe)
				} else {
					s.extendedError(classifyEngineError(cerr))
				}
				return nil
			}
		}
	}
	// Replacing an existing statement drops its portals.
	if _, exists := s.stmts[name]; exists {
		s.closeStatement(name)
	}
	s.stmts[name] = ps
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = writeMessage(s.w(), respParseComplete, nil)
	return err
}

// compileCheck proves the statement prepares against the engine (EXPLAIN
// does not execute the statement; parameter markers bind NULL).
func (s *session) compileCheck(query string, paramCount int) error {
	conn, err := s.ensureConn()
	if err != nil {
		return newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+err.Error())
	}
	args := make([]any, paramCount)
	for i := range args {
		args[i] = nil
	}
	rows, err := conn.QueryContext(s.ctx, "EXPLAIN "+query, args...)
	if err != nil {
		return classifyEngineError(err)
	}
	return rows.Close()
}

// countParameterMarkers infers the parameter count from $N markers when
// the client declared none.
func countParameterMarkers(stripped string) int {
	max := 0
	i := 0
	n := len(stripped)
	for i < n {
		if stripped[i] == '$' {
			j := i + 1
			for j < n && stripped[j] >= '0' && stripped[j] <= '9' {
				j++
			}
			if j > i+1 {
				if v, perr := strconv.Atoi(stripped[i+1 : j]); perr == nil && v > max {
					max = v
				}
				i = j
				continue
			}
		}
		i++
	}
	return max
}

// paramCount is the statement's parameter count: the client-declared OIDs
// when given (PostgreSQL validates the declared count at Parse through
// the engine's own parameter-count check), else the marker count.
func (ps *preparedStmt) paramCount() int {
	if len(ps.paramOIDs) > 0 {
		return len(ps.paramOIDs)
	}
	return countParameterMarkers(ps.stripped)
}

// resolveStatementShape resolves the statement's result shape ONCE from
// the engine's prepare step (prepare-only: the driver binds without
// stepping, so reading rows.Columns() executes nothing — an
// INSERT..RETURNING described by the client does not mutate), storing the
// column names and their DECLARED wire types.
func (s *session) resolveStatementShape(ps *preparedStmt) *PGError {
	if ps.shapeKnown {
		return nil
	}
	if ps.empty || ps.session.kind != "" {
		ps.shapeKnown = true // gateway-implemented or empty: no engine shape
		return nil
	}
	if ps.cls != classQuery && !(ps.cls == classWrite && ps.returns) {
		ps.shapeKnown = true // no rows: NoData
		return nil
	}
	conn, err := s.ensureConn()
	if err != nil {
		return newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+err.Error())
	}
	pctx, pcancel := context.WithTimeout(s.ctx, statementTimeout)
	defer pcancel()
	args := make([]any, ps.paramCount())
	for i := range args {
		args[i] = nil
	}
	rows, qerr := conn.QueryContext(pctx, ps.sql, args...)
	if qerr != nil {
		return asPGError(classifyEngineError(qerr))
	}
	defer rows.Close()
	cols, cerr := rows.Columns() // names without stepping
	if cerr != nil {
		return asPGError(classifyEngineError(cerr))
	}
	ps.colNames = cols
	ps.oids = declaredColumnOIDs(pctx, conn, cols, ps.stripped)
	ps.shapeKnown = true
	return nil
}

// asPGError narrows an error to *PGError.
func asPGError(err error) *PGError {
	if pe, ok := err.(*PGError); ok {
		return pe
	}
	return newPGError(stateInternalError, err.Error())
}

// handleBind processes Bind: portal name, statement name, parameter
// formats, parameter values, result formats. Values travel in text or
// binary (both supported for the mapped types); a binary RESULT format
// request degrades to text with a notice.
func (s *session) handleBind() error {
	payload, err := s.readExtendedPayload()
	if err != nil {
		return err
	}
	portalName, err := readCStrBytes(&payload)
	if err != nil {
		return err
	}
	stmtName, err := readCStrBytes(&payload)
	if err != nil {
		return err
	}
	ps, ok := s.stmts[stmtName]
	if !ok {
		s.extendedError(newPGError(stateUndefinedStatement, fmt.Sprintf("prepared statement %q does not exist", stmtName)))
		return nil
	}
	args, rest, aerr := decodeBindArgs(payload, ps.paramOIDs)
	if aerr != nil {
		s.extendedError(aerr)
		return nil
	}
	binaryRes := false
	if len(rest) >= 2 {
		nFmt := int(int16(binary.BigEndian.Uint16(rest[:2])))
		rest = rest[2:]
		if nFmt > len(rest)/2 {
			s.extendedError(errProtocol("invalid Bind result format list"))
			return nil
		}
		for i := 0; i < nFmt; i++ {
			if int16(binary.BigEndian.Uint16(rest[i*2:i*2+2])) != 0 {
				binaryRes = true
			}
		}
	}
	s.closePortal(portalName)
	s.portals[portalName] = &portal{stmt: ps, args: args, binaryRes: binaryRes}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = writeMessage(s.w(), respBindComplete, nil)
	return err
}

// decodeBindArgs decodes the Bind parameter values (text and binary
// formats for the mapped types) and returns the unconsumed payload (the
// result-format list follows the parameter values).
func decodeBindArgs(payload []byte, paramOIDs []int32) ([]any, []byte, error) {
	if len(payload) < 2 {
		return nil, nil, errProtocol("Bind without a format count")
	}
	nFmt := int(int16(binary.BigEndian.Uint16(payload[:2])))
	payload = payload[2:]
	if nFmt > len(payload)/2 {
		return nil, nil, errProtocol("invalid Bind format list")
	}
	formats := make([]int16, nFmt)
	for i := range formats {
		if len(payload) < 2 {
			return nil, nil, errProtocol("invalid Bind format list")
		}
		formats[i] = int16(binary.BigEndian.Uint16(payload[:2]))
		payload = payload[2:]
	}
	if len(payload) < 2 {
		return nil, nil, errProtocol("Bind without a parameter count")
	}
	nArgs := int(int16(binary.BigEndian.Uint16(payload[:2])))
	payload = payload[2:]
	args := make([]any, nArgs)
	for i := 0; i < nArgs; i++ {
		if len(payload) < 4 {
			return nil, nil, errProtocol("invalid Bind parameter values")
		}
		valLen := int(int32(binary.BigEndian.Uint32(payload[:4])))
		payload = payload[4:]
		if valLen < 0 {
			args[i] = nil // NULL
			continue
		}
		if valLen > len(payload) {
			return nil, nil, errProtocol("invalid Bind parameter values")
		}
		raw := payload[:valLen]
		payload = payload[valLen:]
		format := int16(0)
		if nFmt == 1 {
			format = formats[0]
		} else if i < nFmt {
			format = formats[i]
		}
		oid := int32(0)
		if i < len(paramOIDs) {
			oid = paramOIDs[i]
		}
		v, derr := decodeParamValue(oid, format, raw)
		if derr != nil {
			return nil, nil, derr
		}
		args[i] = v
	}
	return args, payload, nil
}

// decodeParamValue converts one Bind parameter (text or binary) to the
// engine storage value.
func decodeParamValue(oid int32, format int16, raw []byte) (any, error) {
	if format == 0 {
		// Text: declared numeric types are parsed; anything else travels
		// as text (the engine stores text).
		if oid == oidInt2 || oid == oidInt4 || oid == oidInt8 {
			if v, perr := strconv.ParseInt(string(raw), 10, 64); perr == nil {
				return v, nil
			}
			return nil, newPGError(stateInvalidParameterValue, fmt.Sprintf("invalid integer parameter %q", string(raw)))
		}
		if oid == oidFloat4 || oid == oidFloat8 || oid == oidNumeric {
			if v, perr := strconv.ParseFloat(string(raw), 64); perr == nil {
				return v, nil
			}
			return nil, newPGError(stateInvalidParameterValue, fmt.Sprintf("invalid float parameter %q", string(raw)))
		}
		if oid == oidBool {
			switch strings.ToLower(string(raw)) {
			case "t", "true", "1", "y", "yes", "on":
				return 1, nil
			case "f", "false", "0", "n", "no", "off":
				return 0, nil
			}
			return nil, newPGError(stateInvalidParameterValue, fmt.Sprintf("invalid boolean parameter %q", string(raw)))
		}
		return string(raw), nil
	}
	// Binary format.
	switch oid {
	case oidInt2:
		if len(raw) != 2 {
			return nil, newPGError(stateInvalidParameterValue, "invalid binary int2 length")
		}
		return int64(int16(binary.BigEndian.Uint16(raw))), nil
	case oidInt4, oidInt8:
		if len(raw) != 8 {
			return nil, newPGError(stateInvalidParameterValue, "invalid binary integer length")
		}
		return int64(binary.BigEndian.Uint64(raw)), nil
	case oidFloat4, oidFloat8, oidNumeric:
		if len(raw) != 8 {
			return nil, newPGError(stateInvalidParameterValue, "invalid binary float length")
		}
		return math.Float64frombits(binary.BigEndian.Uint64(raw)), nil
	case oidBool:
		if len(raw) != 1 {
			return nil, newPGError(stateInvalidParameterValue, "invalid binary bool length")
		}
		if raw[0] != 0 {
			return 1, nil
		}
		return 0, nil
	case oidBytea:
		return append([]byte(nil), raw...), nil
	case oidText, oidName, oidVARCHAR, oidJSON, oidJSONB, oidUUID, 0, oidUnknown:
		return string(raw), nil
	default:
		return nil, newPGError(stateFeatureNotSupported, fmt.Sprintf(
			"binary parameter format for type oid %d is not supported; send the parameter as text", oid))
	}
}

// handleDescribe processes Describe ('S' statement / 'P' portal):
// ParameterDescription + RowDescription/NoData for a statement;
// RowDescription/NoData for a portal. Describe never executes.
func (s *session) handleDescribe() error {
	payload, err := s.readExtendedPayload()
	if err != nil {
		return err
	}
	if len(payload) < 1 {
		return errProtocol("Describe without a kind byte")
	}
	kind := payload[0]
	rest := payload[1:]
	name, err := readCStrBytes(&rest)
	if err != nil {
		return err
	}
	switch kind {
	case 'S':
		ps, ok := s.stmts[name]
		if !ok {
			s.extendedError(newPGError(stateUndefinedStatement, fmt.Sprintf("prepared statement %q does not exist", name)))
			return nil
		}
		if err := s.writeParameterDescription(ps); err != nil {
			return err
		}
		return s.writeStatementRowDescription(ps)
	case 'P':
		p, ok := s.portals[name]
		if !ok {
			s.extendedError(newPGError(stateUndefinedStatement, fmt.Sprintf("portal %q does not exist", name)))
			return nil
		}
		return s.writeStatementRowDescription(p.stmt)
	default:
		return errProtocol("invalid Describe kind %q", string(kind))
	}
}

// writeParameterDescription writes ParameterDescription with the declared
// (or inferred) parameter OIDs.
func (s *session) writeParameterDescription(ps *preparedStmt) error {
	oids := ps.paramOIDs
	if len(oids) == 0 {
		n := countParameterMarkers(ps.stripped)
		oids = make([]int32, n)
		for i := range oids {
			oids[i] = 0 // unspecified: the client may send any format
		}
	}
	buf := make([]byte, 0, 2+4*len(oids))
	var nBuf [2]byte
	binary.BigEndian.PutUint16(nBuf[:], uint16(len(oids)))
	buf = append(buf, nBuf[:]...)
	for _, oid := range oids {
		var oBuf [4]byte
		binary.BigEndian.PutUint32(oBuf[:], uint32(oid))
		buf = append(buf, oBuf[:]...)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := writeMessage(s.w(), respParameterDescription, buf)
	return err
}

// writeStatementRowDescription writes the result shape of a statement or
// portal: NoData for statements that return no rows; otherwise the shape
// resolved from the engine's prepare step with the declared column types
// (Describe never executes — no statement runs here).
func (s *session) writeStatementRowDescription(ps *preparedStmt) error {
	switch {
	case ps.empty:
		return s.writeNoData()
	case ps.session.kind == "set" || ps.session.kind == "reset":
		return s.writeNoData()
	case ps.session.kind == "show":
		if ps.session.all {
			cols := []pgColumn{
				{Name: "name", OID: oidText},
				{Name: "setting", OID: oidText},
				{Name: "description", OID: oidText},
			}
			return s.writeRowDescription(cols)
		}
		return s.writeRowDescription([]pgColumn{{Name: parameterStatusKey(ps.session.name), OID: oidText}})
	}
	if ps.cls != classQuery && !(ps.cls == classWrite && ps.returns) {
		return s.writeNoData()
	}
	if perr := s.resolveStatementShape(ps); perr != nil {
		s.extendedError(perr)
		return nil
	}
	columns := make([]pgColumn, len(ps.colNames))
	for i, name := range ps.colNames {
		columns[i] = pgColumn{Name: name, OID: ps.oids[i]}
	}
	return s.writeRowDescription(columns)
}

// writeNoData writes a NoData message under the write mutex.
func (s *session) writeNoData() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := writeMessage(s.w(), respNoData, nil)
	return err
}

// handleExecute processes Execute: portal, max rows. The statement runs
// through the same classification, transaction semantics, statement
// timeout and CDC capture as the simple protocol.
func (s *session) handleExecute() error {
	payload, err := s.readExtendedPayload()
	if err != nil {
		return err
	}
	name, err := readCStrBytes(&payload)
	if err != nil {
		return err
	}
	if len(payload) < 4 {
		return errProtocol("Execute without a row limit")
	}
	maxRows := int32(binary.BigEndian.Uint32(payload[:4]))
	p, ok := s.portals[name]
	if !ok {
		s.extendedError(newPGError(stateUndefinedStatement, fmt.Sprintf("portal %q does not exist", name)))
		return nil
	}
	if p.stmt.empty {
		// An empty statement executes to EmptyQueryResponse.
		s.writeMu.Lock()
		_, werr := writeMessage(s.w(), respEmptyQuery, nil)
		s.writeMu.Unlock()
		return werr
	}
	ps := p.stmt

	// A binary RESULT format request degrades to text with a notice (the
	// RowDescription keeps format 0, so the client decodes the text
	// values correctly; postgresql-access.md pins the upstream's
	// binary-result restriction).
	if p.binaryRes && !p.noticed {
		p.noticed = true
		if nerr := s.writeNotice(newPGError(stateFeatureNotSupported,
			"binary result format is not supported; the values are returned as text")); nerr != nil {
			return nerr
		}
	}

	// Cancellation reaches the running Execute through the same batch
	// cancel slot the simple protocol uses; the statement timeout bounds
	// the engine work.
	s.batchMu.Lock()
	batchCtx, batchCancel := context.WithCancel(s.ctx)
	s.batchCancel = batchCancel
	s.batchMu.Unlock()
	defer func() {
		s.batchMu.Lock()
		s.batchCancel = nil
		s.batchMu.Unlock()
		batchCancel()
	}()
	execCtx, execCancel := context.WithTimeout(batchCtx, statementTimeout)
	defer execCancel()

	toks := tokens(ps.stripped)
	first := ""
	if len(toks) > 0 {
		first = toks[0].upper
	}

	limit := int(maxRows) // 0 = no limit
	switch {
	case ps.session.kind != "":
		return s.executeSessionPortal(ps)
	case ps.cls == classTxControl:
		if err := s.credentialBoundary(execCtx); err != nil {
			if s.terminated.Load() {
				return nil // already flagged; the message loop closes
			}
			s.extendedError(err)
			return nil
		}
		conn, cerr := s.ensureConn()
		if cerr != nil {
			s.extendedError(newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+cerr.Error()))
			return nil
		}
		if _, xerr := conn.ExecContext(execCtx, ps.sql); xerr != nil {
			// COMMIT/ROLLBACK outside a block: PostgreSQL answers with a
			// warning notice and a CommandComplete (see executeStatement).
			pe := classifyEngineError(xerr)
			if (first == "COMMIT" || first == "END" || first == "ROLLBACK" || first == "ABORT") && !s.inTx {
				if pgErr, ok := pe.(*PGError); ok && pgErr.Code == stateNoActiveSQLTransaction {
					if nerr := s.writeNotice(newPGError(stateNoActiveSQLTransaction,
						"there is no transaction in progress")); nerr != nil {
						return nerr
					}
					tag := "COMMIT"
					if first == "ROLLBACK" || first == "ABORT" {
						tag = "ROLLBACK"
					}
					return s.writeCommandCompleteTag(tag)
				}
			}
			s.extendedError(pe)
			return nil
		}
		switch {
		case first == "BEGIN" || first == "START":
			s.inTx = true
			return s.writeCommandCompleteTag("BEGIN")
		case first == "COMMIT" || first == "END":
			s.inTx = false
			s.failed = false
			s.endTransactionBlock()
			return s.writeCommandCompleteTag("COMMIT")
		case first == "ROLLBACK" || first == "ABORT":
			s.inTx = false
			s.failed = false
			s.endTransactionBlock()
			return s.writeCommandCompleteTag("ROLLBACK")
		default:
			return s.writeCommandCompleteTag(first)
		}
	case ps.cls == classQuery:
		return s.executePortalRows(execCtx, p, limit, first, false)
	case ps.cls == classWrite:
		if dataMutationKeywords[first] {
			return s.executePortalMutation(execCtx, p, first, toks)
		}
		if ps.returns {
			return s.executePortalRows(execCtx, p, limit, first, true)
		}
		conn, cerr := s.ensureConn()
		if cerr != nil {
			s.extendedError(newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+cerr.Error()))
			return nil
		}
		result, xerr := conn.ExecContext(execCtx, ps.sql, p.args...)
		if xerr != nil {
			s.extendedError(classifyEngineError(xerr))
			return nil
		}
		// Temporary objects are engine CONNECTION state: the owning
		// connection is reset — or discarded — before it returns to the
		// pool (releaseConn; postgresql-access.md session isolation).
		if isTempDDL(pgStatement{stripped: ps.stripped}) {
			s.sessionDirty = true
		}
		affected, _ := result.RowsAffected()
		return s.writeCommandCompleteTag(refineTag(first, writeTag(first, affected), toks))
	default:
		s.extendedError(errProtocol("unclassifiable statement in portal %q", name))
		return nil
	}
}

// executeSessionPortal runs a gateway session-state portal (SET/RESET/
// SHOW) against the session's own state — never the engine.
func (s *session) executeSessionPortal(ps *preparedStmt) error {
	switch ps.session.kind {
	case "set":
		if perr := s.applySet(ps.session); perr != nil {
			s.extendedError(perr)
			return nil
		}
		return s.writeCommandCompleteTag("SET")
	case "reset":
		if perr := s.applyReset(ps.session); perr != nil {
			s.extendedError(perr)
			return nil
		}
		return s.writeCommandCompleteTag("RESET")
	case "show":
		cols, rows, perr := s.showResult(ps.session)
		if perr != nil {
			s.extendedError(perr)
			return nil
		}
		if err := s.writeRowDescription(cols); err != nil {
			return err
		}
		for _, row := range rows {
			if err := s.writeDataRow(row); err != nil {
				return err
			}
		}
		return s.writeCommandCompleteTag("SHOW")
	}
	s.extendedError(errProtocol("unknown session statement in portal"))
	return nil
}

// executePortalMutation runs a DML portal with atomic CDC capture: the
// plain form captures inside the transaction that commits the statement,
// the RETURNING form streams the returned rows on the same runner before
// the frame is appended (capture and data are atomic).
func (s *session) executePortalMutation(ctx context.Context, p *portal, first string, toks []token) error {
	ps := p.stmt
	if ps.returns {
		return s.executePortalMutationReturning(ctx, p, first, toks)
	}
	conn, cerr := s.ensureConn()
	if cerr != nil {
		s.extendedError(newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+cerr.Error()))
		return nil
	}
	captureOf := func(r engineRunner) application.ChangeCapture {
		return changeCaptureOf(ctx, r, first, toks, ps.sql)
	}
	out, xerr := s.runDataMutation(ctx, conn, captureOf, func(r engineRunner) (execResult, error) {
		result, err := r.ExecContext(ctx, ps.sql, p.args...)
		if err != nil {
			return execResult{}, classifyEngineError(err)
		}
		affected, aerr := result.RowsAffected()
		if aerr != nil {
			affected = 0
		}
		return execResult{tag: refineTag(first, writeTag(first, affected), toks)}, nil
	})
	if xerr != nil {
		s.extendedError(xerr)
		return nil
	}
	return s.writeCommandCompleteTag(out.tag)
}

// executePortalMutationReturning streams a DML-with-RETURNING portal and
// appends its CDC frame on the same transaction scope the rows were read
// in.
func (s *session) executePortalMutationReturning(ctx context.Context, p *portal, first string, toks []token) error {
	ps := p.stmt
	conn, cerr := s.ensureConn()
	if cerr != nil {
		s.extendedError(newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+cerr.Error()))
		return nil
	}
	if perr := s.resolveStatementShape(ps); perr != nil {
		s.extendedError(perr)
		return nil
	}
	captureOf := func(r engineRunner) application.ChangeCapture {
		return changeCaptureOf(ctx, r, first, toks, ps.sql)
	}
	out, xerr := s.runDataMutation(ctx, conn, captureOf, func(r engineRunner) (execResult, error) {
		rows, qerr := r.QueryContext(ctx, ps.sql, p.args...)
		if qerr != nil {
			return execResult{}, classifyEngineError(qerr)
		}
		defer rows.Close()
		// Mutation portals run without a row limit, so the stream cannot
		// suspend; the third return is asserted rather than acted on.
		count, _, suspended, serr := s.streamPreparedRows(rows, ps, 0, 0, nil)
		if serr != nil {
			return execResult{}, serr
		}
		if suspended {
			return execResult{}, errProtocol("mutation portal suspended")
		}
		return execResult{
			rowsReturned: count,
			tag:          refineTag(first, writeTag(first, int64(count)), toks),
		}, nil
	})
	if xerr != nil {
		s.extendedError(xerr)
		return nil
	}
	return s.writeCommandCompleteTag(out.tag)
}

// executePortalRows streams a rows-returning portal up to its row limit;
// a limit hit before completion suspends the portal (PortalSuspended) and
// the next Execute resumes the open row stream.
func (s *session) executePortalRows(ctx context.Context, p *portal, limit int, first string, isWrite bool) error {
	ps := p.stmt
	conn, cerr := s.ensureConn()
	if cerr != nil {
		s.extendedError(newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+cerr.Error()))
		return nil
	}
	if perr := s.resolveStatementShape(ps); perr != nil {
		s.extendedError(perr)
		return nil
	}
	var rows *sql.Rows
	var startCount int
	var buffered []any
	if p.suspended != nil {
		rows = p.suspended.rows
		startCount = p.suspended.count
		buffered = p.suspended.buffered
		p.suspended = nil
	} else {
		r, qerr := conn.QueryContext(ctx, ps.sql, p.args...)
		if qerr != nil {
			s.extendedError(classifyEngineError(qerr))
			return nil
		}
		rows = r
	}
	count, buffered, suspended, derr := s.streamPreparedRows(rows, ps, limit, startCount, buffered)
	if derr != nil {
		_ = rows.Close()
		s.extendedError(classifyEngineError(derr))
		return nil
	}
	if suspended {
		p.suspended = &suspendedPortal{rows: rows, count: count, buffered: buffered}
		s.writeMu.Lock()
		_, werr := writeMessage(s.w(), respPortalSuspended, nil)
		s.writeMu.Unlock()
		return werr
	}
	_ = rows.Close()
	tag := selectTag(first, count)
	if isWrite {
		tag = refineTag(first, writeTag(first, int64(count)), tokens(ps.stripped))
	}
	return s.writeCommandCompleteTag(tag)
}

// streamPreparedRows writes the RowDescription (from the statement's
// declared-type shape) and DataRows up to the row limit; when the limit
// stops the stream early it peeks one more row to distinguish "suspended"
// from "ended" and returns that peeked row (it is delivered first on
// resume). Returns the delivered count, the peeked row (nil when none),
// whether the portal suspended and an error.
func (s *session) streamPreparedRows(rows *sql.Rows, ps *preparedStmt, limit, startCount int, buffered []any) (int, []any, bool, error) {
	count := startCount
	cols, cerr := rows.Columns()
	if cerr != nil {
		return count, nil, false, cerr
	}
	oids := ps.oids
	if len(oids) != len(cols) {
		// Shape drift (DDL raced the prepared shape): degrade to TEXT.
		oids = make([]int32, len(cols))
		for i := range oids {
			oids[i] = oidText
		}
	}
	if buffered != nil {
		// The buffered row (from a previous suspension's peek) is
		// delivered first. RowDescription is re-sent for the resumed
		// Execute (every Execute carries its own description; the column
		// set is the same stream's).
		columns := make([]pgColumn, len(cols))
		for i, name := range cols {
			columns[i] = pgColumn{Name: name, OID: oids[i]}
		}
		if err := s.writeRowDescription(columns); err != nil {
			return count, nil, false, err
		}
		count++
		if err := s.writeDataRow(buffered); err != nil {
			return count, nil, false, err
		}
	} else {
		columns := make([]pgColumn, len(cols))
		for i, name := range cols {
			columns[i] = pgColumn{Name: name, OID: oids[i]}
		}
		if err := s.writeRowDescription(columns); err != nil {
			return count, nil, false, err
		}
	}
	vals := make([]any, len(cols))
	dests := make([]any, len(cols))
	delivered := count - startCount
	for {
		if limit > 0 && delivered >= limit {
			// Peek one more row to distinguish suspension from end.
			for i := range dests {
				vals[i] = nil
				dests[i] = &vals[i]
			}
			if rows.Next() {
				if serr := rows.Scan(dests...); serr != nil {
					return count, nil, false, serr
				}
				return count, append([]any(nil), vals...), true, nil
			}
			if rerr := rows.Err(); rerr != nil {
				return count, nil, false, rerr
			}
			return count, nil, false, nil
		}
		for i := range dests {
			vals[i] = nil
			dests[i] = &vals[i]
		}
		if !rows.Next() {
			break
		}
		if serr := rows.Scan(dests...); serr != nil {
			return count, nil, false, serr
		}
		count++
		delivered++
		if err := s.writeDataRow(vals); err != nil {
			return count, nil, false, err
		}
	}
	if rerr := rows.Err(); rerr != nil {
		return count, nil, false, rerr
	}
	return count, nil, false, nil
}

// handleClose processes Close ('S' statement / 'P' portal).
func (s *session) handleClose() error {
	payload, err := s.readExtendedPayload()
	if err != nil {
		return err
	}
	if len(payload) < 1 {
		return errProtocol("Close without a kind byte")
	}
	kind := payload[0]
	rest := payload[1:]
	name, err := readCStrBytes(&rest)
	if err != nil {
		return err
	}
	switch kind {
	case 'S':
		s.closeStatement(name)
	case 'P':
		s.closePortal(name)
	default:
		return errProtocol("invalid Close kind %q", string(kind))
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = writeMessage(s.w(), respCloseComplete, nil)
	return err
}

// writeReadyStatus writes ReadyForQuery with the current transaction
// status byte.
func (s *session) writeReadyStatus() error {
	status := byte('I')
	switch {
	case s.inTx && s.failed:
		status = 'E'
	case s.inTx:
		status = 'T'
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeReadyForQuery(s.w(), status)
}

func (s *session) writeCommandCompleteTag(tag string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := writeMessage(s.w(), respCommandComplete, append([]byte(tag), 0))
	return err
}
