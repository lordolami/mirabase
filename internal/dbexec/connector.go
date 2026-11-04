// Package dbexec owns native execution, pooling, transactions, policy
// guards and capture (build-start.md source ownership table). This file
// provides the transaction-mode seam for the provider connector: the
// inherited deferred BEGIN for framework callers and BEGIN CONCURRENT for
// the bounded MVCC writer profile (integration-contracts.md).
package dbexec

import (
	"context"
	"database/sql/driver"
	"fmt"
)

type ctxKey int

// mvccWriterKey marks a context as belonging to the bounded MVCC writer
// profile. Connections opened for such a context issue BEGIN CONCURRENT.
const mvccWriterKey ctxKey = 1

// WithMvccWriter returns a context that tags the connection use as an MVCC
// writer profile operation.
func WithMvccWriter(ctx context.Context) context.Context {
	return context.WithValue(ctx, mvccWriterKey, true)
}

// IsMvccWriter reports whether the context was tagged as an MVCC writer.
func IsMvccWriter(ctx context.Context) bool {
	v, _ := ctx.Value(mvccWriterKey).(bool)
	return v
}

// MvccConnector wraps a provider connector and implements driver.ConnBeginTx
// on the wrapped connection: the inherited deferred BEGIN semantics remain
// the default, while a context-tagged MVCC writer operation selects
// BEGIN CONCURRENT on that pinned connection. No manual BEGIN on a random
// pooled connection happens outside this path.
type MvccConnector struct {
	Inner driver.Connector
}

var _ driver.Connector = (*MvccConnector)(nil)

// Connect opens a connection and wraps it with the transaction-mode
// behavior.
func (c *MvccConnector) Connect(ctx context.Context) (driver.Conn, error) {
	inner, err := c.Inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &mvccConn{Conn: inner}, nil
}

// Driver delegates to the wrapped connector's driver.
func (c *MvccConnector) Driver() driver.Driver { return c.Inner.Driver() }

type mvccConn struct {
	// Conn is the provider connection (with pragmas already applied).
	driver.Conn
}

var (
	_ driver.ConnBeginTx = (*mvccConn)(nil)
)

// BeginTx starts a transaction on this pinned connection. The default
// profile preserves the inherited serialized semantics (deferred BEGIN via
// the driver's own ConnBeginTx when implemented); the MVCC writer profile
// issues BEGIN CONCURRENT on this connection (integration-contracts.md).
func (c *mvccConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if IsMvccWriter(ctx) {
		if err := execOnConn(ctx, c.Conn, "BEGIN CONCURRENT"); err != nil {
			return nil, fmt.Errorf("failed to begin the MVCC writer transaction: %w", err)
		}
		return &explicitTx{conn: c.Conn}, nil
	}
	if bt, ok := c.Conn.(driver.ConnBeginTx); ok {
		return bt.BeginTx(ctx, opts)
	}
	// The inherited fallback: plain deferred BEGIN.
	if err := execOnConn(ctx, c.Conn, "BEGIN"); err != nil {
		return nil, fmt.Errorf("failed to begin the transaction: %w", err)
	}
	return &explicitTx{conn: c.Conn}, nil
}

// explicitTx commits or rolls back a transaction begun by this wrapper.
type explicitTx struct {
	conn driver.Conn
}

func (t *explicitTx) Commit() error {
	return execOnConn(context.Background(), t.conn, "COMMIT")
}

func (t *explicitTx) Rollback() error {
	return execOnConn(context.Background(), t.conn, "ROLLBACK")
}

func execOnConn(ctx context.Context, conn driver.Conn, stmt string) error {
	if ec, ok := conn.(driver.ExecerContext); ok {
		_, err := ec.ExecContext(ctx, stmt, nil)
		return err
	}
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
