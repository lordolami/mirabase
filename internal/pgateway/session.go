package pgateway

import (
	"bufio"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mirabase/internal/application"
)

// startupHandshakeDeadline bounds the TLS negotiation and the startup
// packet: an idle pre-auth connection cannot hold a session slot.
const startupHandshakeDeadline = 10 * time.Second

// resolvedDatabase is the immutable DatabaseContext resolved from the
// startup packet BEFORE authentication (postgresql-access.md: "then pass
// an immutable authorized DatabaseContext to the native frontend" —
// authorization is verified inside the credential check; the context
// itself never changes for the session).
type resolvedDatabase struct {
	applicationID string
	databaseID    string // "primary" or the opaque database id
	generation    int64
	policyEpoch   int64
	profile       string
}

// session is one authenticated PG wire session.
type session struct {
	gw      *Server
	raw     net.Conn
	br      *bufio.Reader
	log     *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	writeMu sync.Mutex

	// Resolved context + credential (set during startup/auth).
	db         resolvedDatabase
	cred       *application.CredentialRecord
	writeScope bool // sql.write granted

	// Managed collection set of the resolved database (DDL enforcement).
	managed map[string]bool

	// Backend identity for cancellation.
	pid    int32
	secret int32

	// The running statement batch (CancelRequest aborts exactly it; the
	// session itself stays usable, PostgreSQL semantics).
	batchMu     sync.Mutex
	batchCancel context.CancelFunc

	// Engine connection state (dedicated while in a transaction block).
	conn   *sql.Conn
	inTx   bool
	failed bool

	// sessionDirty records that the session created temporary ENGINE
	// objects on its connection (CREATE TEMP ...): releaseConn drops them
	// — or discards the connection — so they cannot leak into the next
	// session that acquires it. Session SETTINGS are gateway state (see
	// sessionstate.go) and never touch the engine connection at all.
	sessionDirty bool

	// Gateway session settings (SET/RESET/SHOW): session scope and
	// transaction-block scope (SET LOCAL). Never on the engine connection.
	settings    map[string]string
	txSettings  map[string]string

	// Extended-protocol state (Parse/Bind/Describe/Execute/Sync/Close).
	stmts        map[string]*preparedStmt
	portals      map[string]*portal
	skipUntilSync bool

	terminated atomic.Bool
}

// cancelBatch aborts the statement batch currently executing on the
// session (no-op when idle).
func (s *session) cancelBatch() {
	s.batchMu.Lock()
	cancel := s.batchCancel
	s.batchMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ---------------------------------------------------------------------------
// Startup and authentication
// ---------------------------------------------------------------------------

// serveStartup runs the TLS-negotiated startup: resolve the application
// and database BEFORE authentication (never trusting a forgeable startup
// parameter for authorization), then authenticate with SCRAM-SHA-256
// against the issued credential, then enforce the credential binding.
// Every failure path answers with an ErrorResponse and closes the
// connection (no trust fallback in either direction).
func (s *session) serveStartup(p *startupParams) error {
	// 1. Unsupported startup parameters are refused (they cannot be
	// silently carried into session state — no leakage, no smuggling).
	if v := p.get("options"); v != "" {
		return s.authFailure(newPGError(stateFeatureNotSupported,
			"the options startup parameter is not supported by this gateway"))
	}
	if v := p.get("replication"); v != "" && v != "off" && v != "false" {
		return s.authFailure(newPGError(stateFeatureNotSupported,
			"replication is not available through the PostgreSQL gateway"))
	}
	if v := p.get("client_encoding"); v != "" && !strings.EqualFold(v, "UTF8") && !strings.EqualFold(v, "UNICODE") {
		return s.authFailure(errAuth(stateInvalidAuthSpecification,
			"client_encoding %q is not supported; the gateway speaks UTF8", v))
	}
	username := p.get("user")
	if username == "" {
		return s.authFailure(errAuth(stateInvalidAuthSpecification,
			"no user specified in the startup packet"))
	}

	// 2. Resolve the application context: v0.1 has exactly one
	// application (application.ApplicationID); nothing on the wire can
	// select another one.
	appID := application.ApplicationID

	// 3. Resolve the database startup name (opaque database id or the
	// reserved primary alias) BEFORE authentication, against the live
	// registry. Unknown and not-ready databases fail here.
	dbName := p.get("database")
	if dbName == "" {
		dbName = application.PrimaryDatabaseID
	}
	rdb, rerr := s.resolveDatabase(appID, dbName)
	if rerr != nil {
		return s.authFailure(rerr)
	}
	s.db = *rdb
	s.log = s.log.With("database", s.db.databaseID, "user", username)

	// 4. Authenticate: SCRAM-SHA-256 over the TLS channel against the
	// stored verifier material of the credential named by the username.
	cred, aerr := s.authenticate(username)
	if aerr != nil {
		return s.authFailure(aerr)
	}

	// 5. Binding: the credential must carry purpose "pg", be scoped to
	// exactly the resolved database and match the live generation/policy
	// epoch. A credential scoped to another database is refused here.
	// The PRIMARY alias has no registry row: its binding state is the
	// credential's own recorded generation/policy epoch, so the resolved
	// context takes those values here and every later boundary compares
	// against them (postgresql-access.md: "Credential scope, expiry and
	// policy epoch are checked at connection and transaction boundaries").
	if s.db.databaseID == application.PrimaryDatabaseID {
		s.db.generation = cred.Generation
		s.db.policyEpoch = cred.PolicyEpoch
	}
	if berr := s.checkBinding(cred, username); berr != nil {
		return s.authFailure(berr)
	}
	s.cred = cred
	s.writeScope = false
	for _, sc := range cred.Scopes {
		if sc == "sql.write" {
			s.writeScope = true
		}
	}
	if !s.writeScope {
		hasRead := false
		for _, sc := range cred.Scopes {
			if sc == "sql.read" {
				hasRead = true
			}
		}
		if !hasRead {
			return s.authFailure(errAuth(stateInvalidAuthSpecification,
				"credential %q carries no SQL scope", username))
		}
	}

	// Managed collection set for DDL enforcement (best-effort; an error
	// here means the registry is unreadable and the session must fail).
	managed, merr := s.gw.rt.Service.ManagedCollectionNames(s.ctx, s.db.databaseID)
	if merr != nil {
		return s.authFailure(newPGError(stateCannotConnectNow,
			"failed to resolve the managed collections of the database"))
	}
	s.managed = managed

	// 6. Accept: AuthenticationOk, parameter statuses, backend key data,
	// ReadyForQuery. bind() assigns the backend identity and registers
	// the session in the cancellation registry (CancelRequest reaches it
	// from here on). The static statuses are the same defaults SHOW
	// reports (sessionstate.go).
	if err := writeAuthRequest(s.w(), authOK, nil); err != nil {
		return err
	}
	for _, kv := range baseParameterStatuses {
		if err := writeParameterStatus(s.w(), kv[0], kv[1]); err != nil {
			return err
		}
	}
	for _, kv := range [][2]string{
		{"session_authorization", username},
		{"application_name", p.get("application_name")},
		{"mirabase_application", appID},
		{"mirabase_database", s.db.databaseID},
	} {
		if err := writeParameterStatus(s.w(), kv[0], kv[1]); err != nil {
			return err
		}
	}
	s.gw.bind(s)
	keyPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(keyPayload[0:4], uint32(s.pid))
	binary.BigEndian.PutUint32(keyPayload[4:8], uint32(s.secret))
	if _, err := writeMessage(s.w(), respBackendKeyData, keyPayload); err != nil {
		return err
	}
	if err := writeReadyForQuery(s.w(), 'I'); err != nil {
		return err
	}
	s.log.Info("pgateway session authenticated",
		"credential", cred.ID, "scopes", strings.Join(cred.Scopes, ","))
	return nil
}

// resolveDatabase resolves the startup database name against the live
// registry: the reserved primary alias or an opaque database id. Unknown
// and not-ready databases are refused (never silently falling back to
// primary).
func (s *session) resolveDatabase(appID, name string) (*resolvedDatabase, error) {
	if name == application.PrimaryDatabaseID {
		// The primary alias: the reserved file-local reference to this
		// application's primary database. Only a primary-scoped
		// ("superuser") credential may use it — enforced by checkBinding.
		// The primary has no registry row: its generation/policy epoch
		// binding is the credential's own recorded state, filled after the
		// exchange succeeds (serveStartup) and compared at every boundary
		// (credentialBoundary) — never a hardcoded pass.
		return &resolvedDatabase{
			applicationID: appID,
			databaseID:    application.PrimaryDatabaseID,
			generation:    0,
			policyEpoch:   0,
			profile:       "primary",
		}, nil
	}
	d, err := s.gw.rt.Service.GetDatabase(s.ctx, name)
	if errors.Is(err, application.ErrNotFound) {
		return nil, newPGError(stateInvalidCatalogName,
			fmt.Sprintf("database %q does not exist", name))
	}
	if err != nil {
		return nil, err
	}
	if d.State != application.StateReady {
		return nil, newPGError(stateCannotConnectNow,
			fmt.Sprintf("database %q is %s and cannot accept connections", name, d.State))
	}
	return &resolvedDatabase{
		applicationID: appID,
		databaseID:    d.ID,
		generation:    d.Generation,
		policyEpoch:   d.PolicyEpoch,
		profile:       d.Profile,
	}, nil
}

// authenticate runs the SCRAM-SHA-256 exchange: AuthenticationSASL with
// the offered mechanism, the client SASLInitialResponse, the server-first
// message, the client SASLResponse, proof verification and the
// server-final signature. The plaintext secret is never transported.
func (s *session) authenticate(username string) (*application.CredentialRecord, error) {
	material, err := s.gw.rt.GetPGSCRAM(s.ctx, username)
	if errors.Is(err, application.ErrNotFound) {
		// Unknown credential: same exchange up front, uniform failure
		// (no credential-existence oracle on the error message).
		return nil, errAuth(stateInvalidPassword,
			"password authentication failed for user %q", username)
	}
	if err != nil {
		return nil, err
	}
	stored := &scramStoredKeys{
		Salt:       material.Salt,
		Iterations: material.Iterations,
		StoredKey:  material.StoredKey,
		ServerKey:  material.ServerKey,
	}

	// The mechanism list is a list of cstrs terminated by an empty cstr
	// (protocol requirement for libpq-compatible clients).
	mechList := scramMechanism + "\x00\x00"
	if err := writeAuthRequest(s.w(), authSASL, []byte(mechList)); err != nil {
		return nil, err
	}

	// SASLInitialResponse: mechanism cstr + int32 len + initial bytes.
	mtype, payload, err := s.readAuthMessage()
	if err != nil {
		return nil, err
	}
	if mtype != msgSASLInitial {
		return nil, errProtocol("expected SASLInitialResponse")
	}
	rest := payload
	mechanism, err := readCStrBytes(&rest)
	if err != nil {
		return nil, err
	}
	if mechanism != scramMechanism {
		return nil, errAuth(stateInvalidAuthSpecification,
			"unsupported SASL mechanism %q", mechanism)
	}
	if len(rest) < 4 {
		return nil, errProtocol("SASLInitialResponse too short")
	}
	initLen := int(int32(binary.BigEndian.Uint32(rest[:4])))
	rest = rest[4:]
	if initLen < 0 {
		return nil, errProtocol("SASL initial response carries no data")
	}
	if initLen > len(rest) {
		return nil, errProtocol("SASL initial response length mismatch")
	}
	sc, serverFirst, err := beginSASL(username, rest[:initLen], stored)
	if err != nil {
		return nil, err
	}
	if err := writeAuthRequest(s.w(), authSASLContinue, []byte(serverFirst)); err != nil {
		return nil, err
	}

	// SASLResponse: the client-final-message.
	mtype, final, err := s.readAuthMessage()
	if err != nil {
		return nil, err
	}
	if mtype != msgSASLResponse {
		return nil, errProtocol("expected SASLResponse")
	}
	serverFinal, err := sc.finishSASL(final)
	if err != nil {
		return nil, err
	}
	if err := writeAuthRequest(s.w(), authSASLFinal, []byte(serverFinal)); err != nil {
		return nil, err
	}

	// The exchange proves possession of the secret; the durable state of
	// the credential (revocation/expiry) is checked here and at every
	// transaction boundary afterwards.
	cred, err := s.gw.rt.CheckCredentialUsable(s.ctx, username)
	if errors.Is(err, application.ErrNotFound) {
		return nil, errAuth(stateInvalidPassword,
			"password authentication failed for user %q", username)
	}
	if errors.Is(err, application.ErrCredentialRevoked) {
		return nil, errAuth(stateInvalidAuthSpecification, "credential is not usable (revoked or expired)")
	}
	if err != nil {
		return nil, err
	}
	return cred, nil
}

// checkBinding enforces the credential binding after the exchange:
// purpose "pg", username agreement, database scope (a credential scoped to
// another database is refused) and the live generation/policy epoch.
func (s *session) checkBinding(cred *application.CredentialRecord, username string) error {
	if cred.Purpose != "pg" {
		return errAuth(stateInvalidAuthSpecification,
			"credential purpose %q is not accepted by the PostgreSQL gateway", cred.Purpose)
	}
	if cred.ID != username {
		return errAuth(stateInvalidAuthSpecification,
			"credential identity does not match the startup user")
	}
	credDB := cred.DatabaseID
	if credDB == "" {
		credDB = application.PrimaryDatabaseID
	}
	if credDB != s.db.databaseID {
		return errAuth(stateInvalidAuthSpecification,
			"the credential is not authorized for database %q", s.db.databaseID)
	}
	if credDB == application.PrimaryDatabaseID {
		// Primary-scoped credentials are bound to their own recorded
		// generation/policy epoch (no registry row exists); the session
		// context took those values at the binding step above.
		if cred.Generation != s.db.generation || cred.PolicyEpoch != s.db.policyEpoch {
			return errAuth(stateInvalidAuthSpecification,
				"the credential no longer matches the database generation or policy epoch")
		}
		return nil
	}
	d, err := s.gw.rt.Service.GetDatabase(s.ctx, s.db.databaseID)
	if err != nil {
		return errAuth(stateInvalidAuthSpecification,
			"the database is no longer available")
	}
	if d.Generation != cred.Generation || d.PolicyEpoch != cred.PolicyEpoch {
		return errAuth(stateInvalidAuthSpecification,
			"the credential no longer matches the database generation or policy epoch")
	}
	return nil
}

// readAuthMessage reads one client message during the authentication
// phase (bounded sizes; anything else is a protocol violation).
func (s *session) readAuthMessage() (byte, []byte, error) {
	mtype, err := s.br.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	n, err := readMessageLength(s.br, maxAuthMessageLen+8)
	if err != nil {
		return 0, nil, err
	}
	payload := make([]byte, n-4)
	if _, err := io.ReadFull(s.br, payload); err != nil {
		return 0, nil, err
	}
	return mtype, payload, nil
}

// authFailure writes the authentication error and reports the connection
// as closed (the caller closes the socket).
func (s *session) authFailure(err error) error {
	var pe *PGError
	if !errors.As(err, &pe) {
		pe = newPGError(stateInternalError, err.Error())
	}
	_ = writeError(s.w(), pe)
	return err
}

// ---------------------------------------------------------------------------
// Wire output (all writes go through the mutex)
// ---------------------------------------------------------------------------

// w returns the framed writer (the raw connection).
func (s *session) w() io.Writer { return s.raw }

// writeErrorLocked wraps writeError under the write mutex.
func (s *session) writeError(e *PGError) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeError(s.w(), e)
}

// writeNotice wraps writeNotice under the write mutex (NoticeResponse;
// errors.go).
func (s *session) writeNotice(e *PGError) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return writeNotice(s.w(), e)
}

func (s *session) writeCommandComplete(tag string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := writeMessage(s.w(), respCommandComplete, append([]byte(tag), 0))
	return err
}

func (s *session) writeRowDescription(columns []pgColumn) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := writeMessage(s.w(), respRowDescription, encodeRowDescription(columns))
	return err
}

func (s *session) writeDataRow(values []any) error {
	payload, err := encodeDataRow(values)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err = writeMessage(s.w(), respDataRow, payload)
	return err
}

// ---------------------------------------------------------------------------
// Credential checks at transaction boundaries
// ---------------------------------------------------------------------------

// credentialBoundary re-validates the credential's durable state and the
// live database binding. A revoked/expired credential or a changed
// generation/policy epoch terminates the session (postgresql-access.md:
// "Rotation/revocation terminates sessions within the documented bounded
// check").
func (s *session) credentialBoundary(ctx context.Context) error {
	cred, err := s.gw.rt.CheckCredentialUsable(ctx, s.cred.ID)
	if errors.Is(err, application.ErrNotFound) || errors.Is(err, application.ErrCredentialRevoked) {
		s.terminateWith(newPGError(stateInvalidAuthSpecification,
			"the credential is no longer valid; the session is terminated"))
		return newPGError(stateInvalidAuthSpecification,
			"the credential is no longer valid; the session is terminated")
	}
	if err != nil {
		return err
	}
	s.cred = cred
	if credDB := credDatabaseID(cred); credDB != s.db.databaseID {
		s.terminateWith(newPGError(stateInvalidAuthSpecification,
			"the credential is no longer valid; the session is terminated"))
		return newPGError(stateInvalidAuthSpecification,
			"the credential is no longer valid; the session is terminated")
	}
	// Generation/policy epoch: primary-scoped credentials are compared
	// against their freshly read credential row (the binding state the
	// session recorded at connect — a re-issued credential row with a
	// bumped generation/epoch terminates live sessions), and database
	// credentials are additionally compared against the LIVE database row
	// (a restore/rotation bumps the database generation).
	if cred.Generation != s.db.generation || cred.PolicyEpoch != s.db.policyEpoch {
		s.terminateWith(newPGError(stateInvalidAuthSpecification,
			"the credential no longer matches the database generation or policy epoch; the session is terminated"))
		return newPGError(stateInvalidAuthSpecification,
			"the credential no longer matches the database generation or policy epoch; the session is terminated")
	}
	if s.db.databaseID != application.PrimaryDatabaseID {
		d, derr := s.gw.rt.Service.GetDatabase(ctx, s.db.databaseID)
		if derr != nil {
			s.terminateWith(newPGError(stateInvalidAuthSpecification,
				"the database is no longer available; the session is terminated"))
			return newPGError(stateInvalidAuthSpecification,
				"the database is no longer available; the session is terminated")
		}
		if d.Generation != cred.Generation || d.PolicyEpoch != cred.PolicyEpoch {
			s.terminateWith(newPGError(stateInvalidAuthSpecification,
				"the credential no longer matches the database generation or policy epoch; the session is terminated"))
			return newPGError(stateInvalidAuthSpecification,
				"the credential no longer matches the database generation or policy epoch; the session is terminated")
		}
	}
	return nil
}

func credDatabaseID(cred *application.CredentialRecord) string {
	if cred.DatabaseID == "" {
		return application.PrimaryDatabaseID
	}
	return cred.DatabaseID
}

// terminateWith flags the session for termination after sending the error
// (the message loop closes the connection on the next iteration).
func (s *session) terminateWith(e *PGError) {
	_ = s.writeError(e)
	s.terminated.Store(true)
}

// ---------------------------------------------------------------------------
// Engine connection lifecycle
// ---------------------------------------------------------------------------

// targetHandle opens the engine handle of the resolved database: the
// primary registry connection or the private target through the shared,
// admission-coupled handle cache.
func (s *session) targetHandle() (*sql.DB, error) {
	if s.db.databaseID == application.PrimaryDatabaseID {
		return s.gw.rt.PrimaryDB, nil
	}
	return s.gw.rt.Handles.Open(s.ctx, s.db.databaseID)
}

// ensureConn acquires the dedicated engine connection (held only while a
// statement batch runs or a transaction block is open; released otherwise
// so an idle session never pins pool capacity).
func (s *session) ensureConn() (*sql.Conn, error) {
	if s.conn != nil {
		return s.conn, nil
	}
	handle, err := s.targetHandle()
	if err != nil {
		return nil, err
	}
	conn, err := handle.Conn(s.ctx)
	if err != nil {
		return nil, err
	}
	s.conn = conn
	return conn, nil
}

// releaseConn returns the dedicated connection when no transaction block
// is open and no portal streams rows. A connection carrying temporary
// engine objects STAYS with the session until cleanup (temporary tables
// are session-scoped in PostgreSQL; dropping them at every batch release
// would break the session's own later statements) — at cleanup the temp
// objects are dropped, or the connection is discarded
// (integration-contracts.md: "Implement ResetSession and discard a
// connection after failed cleanup"), so no temporary state can leak across
// sessions (postgresql-access.md session isolation). Session SETTINGS are
// gateway state and never touch the engine connection at all
// (sessionstate.go).
func (s *session) releaseConn() {
	if s.inTx {
		return
	}
	if s.portalRowsOpen() {
		return
	}
	if s.sessionDirty {
		// The session's temporary objects live on this connection: keep
		// it until cleanup resets (or discards) it.
		return
	}
	if c := s.conn; c != nil {
		s.conn = nil
		_ = c.Close()
	}
}

// resetEngineTempObjects drops every temporary object a session created on
// its engine connection before the connection returns to the pool. The
// temp catalog (sqlite_temp_master) is read on the connection itself and
// each table/view/index/trigger is dropped by its quoted name; internal
// sqlite_* objects are left alone. Any failure reports false — the caller
// discards the connection instead of pooling it (fail-safe: an unrestored
// connection is never reused).
func resetEngineTempObjects(c *sql.Conn, parent context.Context) bool {
	rctx, rcancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer rcancel()
	rows, err := c.QueryContext(rctx, `SELECT name, type FROM sqlite_temp_master`)
	if err != nil {
		return false
	}
	type tempObject struct {
		name, kind string
	}
	var objects []tempObject
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			rows.Close()
			return false
		}
		lower := strings.ToLower(name)
		if lower == "" || strings.HasPrefix(lower, "sqlite_") {
			continue // internal catalog objects are not session temp state
		}
		objects = append(objects, tempObject{name: name, kind: kind})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false
	}
	rows.Close()
	for _, o := range objects {
		stmt := "DROP TABLE " + quoteEngineIdentifier(o.name)
		switch strings.ToUpper(o.kind) {
		case "VIEW":
			stmt = "DROP VIEW " + quoteEngineIdentifier(o.name)
		case "INDEX":
			stmt = "DROP INDEX " + quoteEngineIdentifier(o.name)
		case "TRIGGER":
			stmt = "DROP TRIGGER " + quoteEngineIdentifier(o.name)
		}
		if _, err := c.ExecContext(rctx, stmt); err != nil {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Message loop
// ---------------------------------------------------------------------------

// serve runs the post-authentication message loop until Terminate, EOF,
// error or termination.
func (s *session) serve() error {
	s.extendedInit()
	for {
		if s.terminated.Load() {
			return io.EOF
		}
		mtype, err := s.br.ReadByte()
		if err != nil {
			return err
		}
		switch mtype {
		case msgQuery:
			if err := s.handleQuery(); err != nil {
				return err
			}
		case msgTerminate:
			return io.EOF
		case msgPassword, msgCancelSASL:
			// Authentication messages outside the auth phase are a
			// protocol violation (msgPassword, msgSASLInitial,
			// msgSASLResponse, msgGSSEncResponse and the SASL abort all
			// share the 'p'/'f' type bytes).
			s.writeError(errProtocol("unexpected authentication message outside the startup phase"))
			return errProtocol("unexpected authentication message outside the startup phase")
		case msgParse, msgBind, msgDescribe, msgExecute, msgFlush, msgSync, msgCloseStmt:
			// The Extended Query protocol (Parse/Bind/Describe/Execute/
			// Flush/Sync/Close): a bounded protocol-v3 state machine with
			// skip-until-Sync error recovery (postgresql-access.md).
			if err := s.handleExtended(mtype); err != nil {
				return err
			}
		case 'd', 'c':
			// COPY-data/Done without a COPY exchange (COPY is refused at
			// classification; the SASL abort 'f' is handled above): fail
			// safely, never hang.
			if err := s.handleExtended(mtype); err != nil {
				return err
			}
		default:
			s.writeError(errProtocol("unknown message type %q", string(rune(mtype))))
			return errProtocol("unknown message type %q", string(rune(mtype)))
		}
	}
}

// handleQuery executes one simple Query message: split, per-statement
// parsed enforcement and execution over the held connection, then
// ReadyForQuery with the transaction status.
func (s *session) handleQuery() error {
	n, err := readMessageLength(s.br, maxRegularMessageLen)
	if err != nil {
		return err
	}
	payload := make([]byte, n-4)
	if _, err := io.ReadFull(s.br, payload); err != nil {
		return err
	}
	query, err := readCStringBytes(payload)
	if err != nil {
		return err
	}

	stmts, serr := splitStatements(query)
	if serr != nil {
		if pe, ok := serr.(*PGError); ok {
			s.writeError(pe)
		} else {
			s.writeError(newPGError(stateSyntaxError, serr.Error()))
		}
		return s.finishQueryBatch()
	}

	conn, cerr := s.ensureConn()
	if cerr != nil {
		s.writeError(newPGError(stateCannotConnectNow, "failed to acquire a database connection: "+cerr.Error()))
		return s.finishQueryBatch()
	}

	// The batch runs under its own cancelable context: a CancelRequest
	// aborts exactly the running statement, the session stays usable.
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

	res, berr := s.executeBatch(batchCtx, conn, stmts)
	// Transaction bookkeeping from the batch (applied before the error
	// path: a partial batch may have opened or ended a block). A block
	// that ended discards its transaction-scoped settings (SET LOCAL).
	if res.started {
		s.inTx = true
	}
	if res.ended {
		s.inTx = false
		s.failed = false
		s.endTransactionBlock()
	}
	if berr != nil {
		if pe, ok := berr.(*PGError); ok {
			s.writeError(pe)
		} else {
			s.writeError(newPGError(stateInternalError, berr.Error()))
		}
		// A failed statement inside a transaction block leaves the block
		// in the failed state (25P02 for later statements).
		if s.inTx {
			s.failed = true
		}
	} else if res.empty {
		s.writeMu.Lock()
		_, _ = writeMessage(s.w(), respEmptyQuery, nil)
		s.writeMu.Unlock()
	}
	return s.finishQueryBatch()
}

// finishQueryBatch releases the engine connection outside a transaction
// block and writes ReadyForQuery with the current status byte
// ('I' idle, 'T' in a transaction block, 'E' in a failed one).
func (s *session) finishQueryBatch() error {
	s.releaseConn()
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

// readCStringBytes reads one NUL-terminated string from a byte slice
// (the Query payload).
func readCStringBytes(b []byte) (string, error) {
	end := -1
	for i, c := range b {
		if c == 0 {
			end = i
			break
		}
	}
	if end < 0 {
		return "", errProtocol("unterminated query string")
	}
	return string(b[:end]), nil
}

// cleanup releases the session's resources (rollback an open transaction
// block best-effort, close suspended portals, return the engine
// connection, deregister).
func (s *session) cleanup() {
	s.cancel()
	for name := range s.portals {
		s.closePortal(name)
	}
	if c := s.conn; c != nil {
		if s.inTx {
			_, _ = c.ExecContext(context.WithoutCancel(s.ctx), "ROLLBACK")
		}
		if s.sessionDirty {
			// A connection with temporary engine objects must not return
			// to the pool with its state: reset best-effort, discard on
			// failure.
			s.sessionDirty = false
			if !resetEngineTempObjects(c, s.ctx) {
				_ = c.Raw(func(any) error { return driver.ErrBadConn })
			}
		}
		_ = c.Close()
		s.conn = nil
	}
	_ = s.raw.Close()
	s.gw.deregister(s)
}

// fmtSession renders the session for logs (never secrets).
func (s *session) String() string {
	if s.cred != nil {
		return fmt.Sprintf("pid=%d db=%s cred=%s", s.pid, s.db.databaseID, s.cred.ID)
	}
	return "pid=" + strconv.Itoa(int(s.pid))
}
