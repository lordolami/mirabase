package apis

import (
	"crypto/tls"
	"log/slog"
	"net"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/pgateway"
)

// StartPGGateway brings the PostgreSQL wire gateway listener up
// (postgresql-access.md). The gateway is DISABLED BY DEFAULT: the wiring
// step calls this only when the PostgreSQL access feature is enabled. The
// default bind is 127.0.0.1:5433; TLS is required (a nil tlsConfig is
// refused — the upstream trust-only/no-TLS server cannot be exposed), and
// authentication is SCRAM-SHA-256 against the server-issued pg-purpose
// scoped credentials, with the application/database context resolved from
// the startup packet before the credential is verified.
//
// The returned listener is owned by the caller: Close() stops the accept
// loop, the bounded revocation checker and all live sessions.
//
// The application superuser is never consulted by this surface; an
// application superuser cannot silently become an instance operator
// through it — whole-database SQL authority comes exclusively from the
// credential's sql.read/sql.write scopes.
func StartPGGateway(app core.App, addr string, tlsConfig *tls.Config, log *slog.Logger) (net.Listener, error) {
	rt, err := application.BindRuntime(app)
	if err != nil {
		return nil, err
	}
	cfg := pgateway.Config{
		Addr: addr,
		TLS:  tlsConfig,
	}
	server := pgateway.New(rt, cfg, log)
	return server.Listen()
}
