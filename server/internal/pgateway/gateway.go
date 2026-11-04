package pgateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"mirabase/internal/application"
)

// Defaults for the gateway configuration (postgresql-access.md: "Disabled
// by default. Default enabled bind is 127.0.0.1:5433").
const (
	DefaultAddr         = "127.0.0.1:5433"
	DefaultMaxSessions  = 32
	// SessionCheckInterval is the documented bounded revocation check:
	// live sessions re-verify their credential at least this often, so a
	// rotation or revocation terminates sessions within the bounded
	// window (postgresql-access.md: "Rotation/revocation terminates
	// sessions within the documented bounded check"). Transaction
	// boundaries additionally check immediately.
	SessionCheckInterval = 15 * time.Second
)

// Config is the gateway listener configuration.
type Config struct {
	// Addr is the bind address (default 127.0.0.1:5433).
	Addr string
	// TLS is REQUIRED: the gateway never serves a plaintext session (the
	// upstream trust-only/no-TLS server cannot be exposed; trust fallback
	// is rejected on every path).
	TLS *tls.Config
	// MaxSessions bounds concurrent authenticated sessions
	// (connection exhaustion fails safely with 53300).
	MaxSessions int
}

// Server is the PostgreSQL wire gateway: the TLS listener, the session
// admission bound, the cancellation registry and the bounded credential
// revocation check.
type Server struct {
	rt  *application.Runtime
	cfg Config
	log *slog.Logger

	mu       sync.Mutex
	sessions map[int32]*session // by backend pid (cancellation registry)
	live     int
	nextPid  int32

	ctx    context.Context
	cancel context.CancelFunc
}

// New builds the gateway server over the partitioning runtime. The
// runtime must already be bound (application.BindRuntime).
func New(rt *application.Runtime, cfg Config, log *slog.Logger) *Server {
	if cfg.Addr == "" {
		cfg.Addr = DefaultAddr
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = DefaultMaxSessions
	}
	if log == nil {
		log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		rt:       rt,
		cfg:      cfg,
		log:      log,
		sessions: map[int32]*session{},
		nextPid:  1000,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Listen binds the listener, starts the accept loop and the bounded
// credential revocation checker, and returns the listener (the caller
// owns its lifetime; Close stops the accept loop and in-flight sessions).
// A nil TLS configuration is refused: the gateway is TLS-only.
func (g *Server) Listen() (net.Listener, error) {
	if g.cfg.TLS == nil {
		return nil, errors.New("the PostgreSQL gateway requires a TLS configuration (TLS-only; no plaintext service)")
	}
	ln, err := net.Listen("tcp", g.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind the PostgreSQL gateway on %s: %w", g.cfg.Addr, err)
	}
	go g.acceptLoop(ln)
	go g.revocationLoop()
	return ln, nil
}

// Close stops the accept loop and all live sessions.
func (g *Server) Close() {
	g.cancel()
	g.mu.Lock()
	sessions := make([]*session, 0, len(g.sessions))
	for _, s := range g.sessions {
		sessions = append(sessions, s)
	}
	g.sessions = map[int32]*session{}
	g.live = 0
	g.mu.Unlock()
	for _, s := range sessions {
		_ = s.raw.Close()
	}
}

// acceptLoop accepts plaintext connections and performs the first-byte
// negotiation: SSLRequest upgrades to TLS, a direct plaintext startup is
// refused (TLS-only, no trust fallback), CancelRequest is honored on any
// transport.
func (g *Server) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-g.ctx.Done():
				return
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			g.log.Warn("pgateway accept failed", "error", err)
			return
		}
		go g.handleConn(conn)
	}
}

// handleConn performs the negotiation for one accepted connection.
func (g *Server) handleConn(raw net.Conn) {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(startupHandshakeDeadline))
	br := bufio.NewReader(raw)
	for {
		n, err := readMessageLength(br, maxStartupMessageLen)
		if err != nil {
			return
		}
		if n == 8 {
			var codeBuf [4]byte
			if _, err := io.ReadFull(br, codeBuf[:]); err != nil {
				return
			}
			switch binary.BigEndian.Uint32(codeBuf[:]) {
			case sslRequestCode:
				// TLS upgrade: reply 'S' and handshake. TLS is the only
				// supported transport; the handshake failure closes.
				var leftover []byte
				if bn := br.Buffered(); bn > 0 {
					leftover = make([]byte, bn)
					if _, err := io.ReadFull(br, leftover); err != nil {
						return
					}
				}
				if _, err := raw.Write([]byte{'S'}); err != nil {
					return
				}
				tlsConn := tls.Server(raw, g.cfg.TLS)
				if err := tlsConn.HandshakeContext(g.ctx); err != nil {
					g.log.Debug("pgateway TLS handshake failed", "error", err)
					return
				}
				reader := bufio.NewReader(io.MultiReader(bytes.NewReader(leftover), tlsConn))
				g.serveSession(tlsConn, reader)
				return
			case gssEncRequest:
				// GSSAPI encryption is not supported; the client may retry
				// with TLS (a plaintext startup is then refused below).
				if _, err := raw.Write([]byte{'N'}); err != nil {
					return
				}
				continue
			case cancelRequest:
				// CancelRequest (16 bytes): pid + secret key.
				var body [8]byte
				if _, err := io.ReadFull(br, body[:]); err != nil {
					return
				}
				pid := int32(binary.BigEndian.Uint32(body[0:4]))
				secret := int32(binary.BigEndian.Uint32(body[4:8]))
				g.cancelSession(pid, secret)
				return
			default:
				return
			}
		}
		// A non-8 length at this point is the StartupMessage in
		// plaintext: refuse it (TLS-only service, no trust fallback).
		payload := make([]byte, n-4)
		if _, err := io.ReadFull(br, payload); err != nil {
			return
		}
		params, perr := parseStartupMessage(payload)
		_ = params
		if perr != nil {
			_ = writeError(raw, newPGError(stateProtocolViolation, "invalid startup packet"))
			return
		}
		_ = writeError(raw, newPGError(stateInvalidAuthSpecification,
			"no PostgreSQL connection started with TLS: the PostgreSQL gateway requires TLS"))
		return
	}
}

// serveSession runs one session: admission, startup/auth, message loop.
func (g *Server) serveSession(conn net.Conn, br *bufio.Reader) {
	sctx, cancel := context.WithCancel(g.ctx)
	s := &session{
		gw:     g,
		raw:    conn,
		br:     br,
		log:    g.log,
		ctx:    sctx,
		cancel: cancel,
	}
	// Admission: an authenticated session over the bound is refused
	// (connection exhaustion fails safely with 53300).
	if !g.register(s) {
		_ = writeError(conn, newPGError(stateTooManyConnections,
			"the PostgreSQL gateway session bound is exhausted; retry later"))
		cancel()
		return
	}
	defer s.cleanup()

	// The pre-auth deadline survives the TLS handshake: an idle
	// pre-authentication connection cannot hold a session slot. It is
	// cleared only after a successful authentication.
	_ = conn.SetDeadline(time.Now().Add(startupHandshakeDeadline))

	// Startup packet: length + protocol version + parameters.
	n, err := readMessageLength(br, maxStartupMessageLen)
	if err != nil {
		return
	}
	payload := make([]byte, n-4)
	if _, err := io.ReadFull(br, payload); err != nil {
		return
	}
	params, perr := parseStartupMessage(payload)
	if perr != nil {
		if pe, ok := perr.(*PGError); ok {
			_ = s.authFailure(pe)
		} else {
			_ = s.authFailure(errProtocol("invalid startup packet"))
		}
		return
	}
	if err := s.serveStartup(params); err != nil {
		g.log.Info("pgateway startup refused", "error", err.Error())
		return
	}
	_ = conn.SetDeadline(time.Time{}) // authenticated sessions are long-lived
	if err := s.serve(); err != nil && !errors.Is(err, io.EOF) {
		g.log.Debug("pgateway session ended", "session", s.String(), "error", err.Error())
	}
}

// register admits a session (bounded). The session is registered only
// after the startup resolution; cancel requests target authenticated
// sessions.
func (g *Server) register(s *session) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.live >= g.cfg.MaxSessions {
		return false
	}
	g.live++
	return true
}

// bind assigns the backend identity and registers the session for
// cancellation (called once, after the credential binding succeeded).
func (g *Server) bind(s *session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nextPid++
	s.pid = g.nextPid
	var b [4]byte
	_, _ = rand.Read(b[:])
	s.secret = int32(binary.BigEndian.Uint32(b[:]))
	g.sessions[s.pid] = s
}

// deregister removes the session from the cancellation registry.
func (g *Server) deregister(s *session) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if s.pid != 0 {
		delete(g.sessions, s.pid)
	}
	g.live--
	if g.live < 0 {
		g.live = 0
	}
}

// cancelSession honors a CancelRequest: the backend key must match. The
// running statement batch is aborted; the session itself stays usable.
func (g *Server) cancelSession(pid, secret int32) {
	g.mu.Lock()
	s := g.sessions[pid]
	g.mu.Unlock()
	if s != nil && s.secret == secret {
		s.cancelBatch()
	}
}

// revocationLoop is the bounded revocation check: live sessions re-verify
// their credential (revocation/expiry, database binding, generation and
// policy epoch) at least every SessionCheckInterval; a failed check
// terminates the session.
func (g *Server) revocationLoop() {
	ticker := time.NewTicker(SessionCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
		}
		g.mu.Lock()
		sessions := make([]*session, 0, len(g.sessions))
		for _, s := range g.sessions {
			sessions = append(sessions, s)
		}
		g.mu.Unlock()
		for _, s := range sessions {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 5*time.Second)
			err := s.credentialBoundary(ctx)
			cancel()
			if err != nil {
				// credentialBoundary already sent the ErrorResponse and
				// flagged the session; closing the socket ends it.
				_ = s.raw.Close()
			}
		}
	}
}
