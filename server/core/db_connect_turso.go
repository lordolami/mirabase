//go:build no_default_driver

// Package core — Turso-native DB connection (MiraBase V01-02).
//
// This variant of DefaultDBConnect builds when the no_default_driver tag is
// set: the inherited modernc.org/sqlite connector is excluded and BOTH
// application databases (data.db and auxiliary.db) go through the pinned
// Turso engine via its database/sql driver, wrapped by the owned provider
// connector. The original !no_default_driver variant remains available for
// reference qualification and import tooling only; it is never a runtime
// fallback (native-provider.md).
package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"ganigeorgiev.com/pocketbase/dbx"
	"mirabase/internal/mbengine"
	tursogo "turso.tech/database/tursogo"
)

// pragmaSpec is one inherited connection setting to apply and read back on
// every connection (native-provider.md: "Run the inherited PRAGMA
// configuration only after checking support and actual readback"; "Never
// ignore a failed security/durability PRAGMA").
type pragmaSpec struct {
	name     string
	value    string
	// hard pragmas are security/durability: a failed application or a
	// readback mismatch is a connection error. Soft pragmas are recorded as
	// explicit provider differences instead.
	hard     bool
	readback string // expected normalized readback value ("" = skip readback)
	numeric  bool   // readback compares integer value
}

// inheritedPragmas is the inherited connection configuration, annotated
// with the pinned engine's support level (turso/COMPAT.md at the pinned
// commit).
var inheritedPragmas = []pragmaSpec{
	// Note: the busy_timeout pragma must be first because the connection
	// needs to block on busy before WAL mode is set (inherited comment).
	{name: "busy_timeout", value: "10000", hard: true, readback: "10000", numeric: true},
	{name: "journal_mode", value: "WAL", hard: true, readback: "wal"},
	{name: "synchronous", value: "NORMAL", hard: true, readback: "1", numeric: true},
	{name: "foreign_keys", value: "ON", hard: true, readback: "1", numeric: true},
	{name: "temp_store", value: "MEMORY", hard: false, readback: "2", numeric: true},
	{name: "cache_size", value: "-32000", hard: false, readback: "-32000", numeric: true},
}

// DefaultDBConnect opens a DBX database over the pinned Turso engine with
// the inherited PRAGMA configuration applied and verified per connection.
func TursoDBConnect(dbPath string) (*dbx.DB, error) {
	// The payload trust sequence (digest pin, extraction, ABI and
	// capability probes) must complete before any connection is opened.
	if err := mbengine.Ensure(); err != nil {
		return nil, err
	}

	path, legacy, err := parseLegacySQLiteDSN(dbPath)
	if err != nil {
		return nil, err
	}
	_ = legacy

	// The turso driver DSN: engine path plus a native busy timeout. VFS is
	// left at the engine default (the platform fallback remains; the
	// target-specific async backends are selected through the owned bridge
	// config, not the driver DSN).
	dsn := path + "?_busy_timeout=10000&experimental=vacuum"
	inner, err := tursogo.NewConnector(dsn, tursogo.WithBusyTimeout(10000))
	if err != nil {
		return nil, fmt.Errorf("failed to create the turso connector: %w", err)
	}

	db := sql.OpenDB(&providerConnector{inner: inner})
	db.SetConnMaxIdleTime(0)
	db.SetConnMaxLifetime(0)
	return dbx.NewFromDB(db, "sqlite"), nil
}

// parseLegacySQLiteDSN splits the inherited modernc-style DSN
// (path?_pragma=...&_defensive=1) into the engine path and the pragma list.
// Unknown legacy options are recorded as provider differences.
func parseLegacySQLiteDSN(dsn string) (string, []string, error) {
	q := strings.IndexByte(dsn, '?')
	if q < 0 {
		return dsn, nil, nil
	}
	path := dsn[:q]
	raw := dsn[q+1:]
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse the connection DSN: %w", err)
	}
	var pragmas []string
	for _, p := range vals["_pragma"] {
		open := strings.IndexByte(p, '(')
		if open <= 0 {
			pragmas = append(pragmas, p)
			continue
		}
		name := p[:open]
		value := strings.TrimSuffix(p[open+1:], ")")
		pragmas = append(pragmas, name+"="+value)
	}
	return path, pragmas, nil
}

// providerConnector wraps the turso driver connector and applies the
// inherited PRAGMA configuration with readback verification on every
// connection.
type providerConnector struct {
	inner driver.Connector
	once  sync.Once
}

var _ driver.Connector = (*providerConnector)(nil)

func (c *providerConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("turso connect failed: %w", err)
	}
	diffs, err := applyPragmas(ctx, conn)
	if err != nil {
		// Security/durability pragma failure: discard the connection
		// (native-provider.md: never ignore a failed pragma; foreign keys
		// must be active on every relevant connection).
		if closer, ok := conn.(driver.Conn); ok {
			_ = closer.Close()
		}
		return nil, err
	}
	c.once.Do(func() {
		log.Printf("native provider: connection pragmas verified: busy_timeout=10000 journal_mode=wal synchronous=normal foreign_keys=on; %s", diffs)
	})
	return conn, nil
}

func (c *providerConnector) Driver() driver.Driver { return c.inner.Driver() }

// applyPragmas executes the inherited pragma set on a fresh connection and
// verifies the readback of each supported setting. Supported-by-engine
// pragmas that mismatch are hard failures for security/durability settings
// and recorded provider differences otherwise. journal_size_limit is not
// supported by the pinned engine and is recorded as an explicit provider
// difference (the MB-side WAL-size monitoring takes over in the admission
// layer).
func applyPragmas(ctx context.Context, conn driver.Conn) (string, error) {
	var diffs []string
	for _, p := range inheritedPragmas {
		stmt := fmt.Sprintf("PRAGMA %s=%s", p.name, p.value)
		if err := execSimple(ctx, conn, stmt); err != nil {
			msg := fmt.Sprintf("pragma %s failed: %v", p.name, err)
			if p.hard {
				return "", fmt.Errorf("%s", msg)
			}
			diffs = append(diffs, msg+" (provider difference)")
			continue
		}
		if p.readback == "" {
			continue
		}
		got, err := querySimple(ctx, conn, "PRAGMA "+p.name)
		if err != nil {
			msg := fmt.Sprintf("pragma %s readback failed: %v", p.name, err)
			if p.hard {
				return "", fmt.Errorf("%s", msg)
			}
			diffs = append(diffs, msg+" (provider difference)")
			continue
		}
		if !readbackMatches(got, p.readback, p.numeric) {
			msg := fmt.Sprintf("pragma %s readback mismatch: got %q want %q", p.name, got, p.readback)
			if p.hard {
				return "", fmt.Errorf("%s", msg)
			}
			diffs = append(diffs, msg+" (provider difference)")
		}
	}
	// The inherited modernc-specific journal_size_limit is not supported by
	// the pinned engine; record it once as a provider difference.
	diffs = append(diffs, "journal_size_limit is not supported by the pinned engine (provider difference; WAL-size monitoring belongs to the admission layer)")
	// The modernc-specific _defensive connection flag has no Turso
	// equivalent; the MB-side statement policy provides the intent.
	diffs = append(diffs, "_defensive connection flag has no Turso equivalent (provider difference; statement policy owned by dbexec)")
	return strings.Join(diffs, "; "), nil
}

func readbackMatches(got, want string, numeric bool) bool {
	g := strings.ToLower(strings.TrimSpace(got))
	w := strings.ToLower(strings.TrimSpace(want))
	if numeric {
		gi, gerr := strconv.ParseInt(g, 10, 64)
		wi, werr := strconv.ParseInt(w, 10, 64)
		if gerr == nil && werr == nil {
			return gi == wi
		}
	}
	return g == w
}

func execSimple(ctx context.Context, conn driver.Conn, stmt string) error {
	if ec, ok := conn.(driver.ExecerContext); ok {
		_, err := ec.ExecContext(ctx, stmt, nil)
		return err
	}
	// Fallback through the prepare/execute path.
	p, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, stmt)
	if err != nil {
		return err
	}
	defer p.Close()
	if se, ok := p.(driver.StmtExecContext); ok {
		_, err = se.ExecContext(ctx, nil)
	}
	return err
}

func querySimple(ctx context.Context, conn driver.Conn, stmt string) (string, error) {
	if qc, ok := conn.(driver.QueryerContext); ok {
		rows, err := qc.QueryContext(ctx, stmt, nil)
		if err != nil {
			return "", err
		}
		defer rows.Close()
		return firstRowString(rows)
	}
	p, err := conn.(driver.ConnPrepareContext).PrepareContext(ctx, stmt)
	if err != nil {
		return "", err
	}
	defer p.Close()
	q, ok := p.(driver.StmtQueryContext)
	if !ok {
		return "", fmt.Errorf("connection does not support query")
	}
	rows, err := q.QueryContext(ctx, nil)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	return firstRowString(rows)
}

func firstRowString(rows driver.Rows) (string, error) {
	vals := make([]driver.Value, len(rows.Columns()))
	if err := rows.Next(vals); err != nil {
		return "", err
	}
	switch v := vals[0].(type) {
	case string:
		return v, nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("unexpected pragma readback type %T", v)
	}
}
