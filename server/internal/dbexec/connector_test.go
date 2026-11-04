package dbexec

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"testing"
)

// fakeConn records executed statements.
type fakeConn struct {
	executed []string
	begun    []string // statements issued via BeginTx
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 0, nil }

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}
func (c *fakeConn) Close() error              { return nil }
func (c *fakeConn) Begin() (driver.Tx, error) { return nil, errors.New("not implemented") }

func (c *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.executed = append(c.executed, query)
	return fakeResult{}, nil
}

// fakeConnBeginTx additionally implements driver.ConnBeginTx.
type fakeConnBeginTx struct{ fakeConn }

type fakeTx struct{ c *fakeConnBeginTx }

func (t *fakeTx) Commit() error   { return nil }
func (t *fakeTx) Rollback() error { return nil }

func (c *fakeConnBeginTx) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.begun = append(c.begun, "driver-internal-begin")
	return &fakeTx{c: c}, nil
}

type fakeConnector struct{ conn driver.Conn }

func (f *fakeConnector) Connect(ctx context.Context) (driver.Conn, error) { return f.conn, nil }
func (f *fakeConnector) Driver() driver.Driver {
	return fakeDriver{}
}

type fakeDriver struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) { return nil, errors.New("unused") }

func TestMvccConnectorSelectsBeginConcurrentForTaggedContext(t *testing.T) {
	inner := &fakeConnBeginTx{}
	c := &MvccConnector{Inner: &fakeConnector{conn: inner}}

	conn, err := c.Connect(WithMvccWriter(context.Background()))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	bt, ok := conn.(driver.ConnBeginTx)
	if !ok {
		t.Fatalf("wrapped conn must implement driver.ConnBeginTx")
	}
	tx, err := bt.BeginTx(WithMvccWriter(context.Background()), driver.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if !reflect.DeepEqual(inner.executed, []string{"BEGIN CONCURRENT"}) {
		t.Fatalf("executed = %v, want [BEGIN CONCURRENT]", inner.executed)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !reflect.DeepEqual(inner.executed, []string{"BEGIN CONCURRENT", "COMMIT"}) {
		t.Fatalf("executed after commit = %v", inner.executed)
	}

	// Rollback path.
	tx2, err := bt.BeginTx(WithMvccWriter(context.Background()), driver.TxOptions{})
	if err != nil {
		t.Fatalf("begin 2: %v", err)
	}
	if err := tx2.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !reflect.DeepEqual(inner.executed, []string{"BEGIN CONCURRENT", "COMMIT", "BEGIN CONCURRENT", "ROLLBACK"}) {
		t.Fatalf("executed after rollback = %v", inner.executed)
	}
	_ = io.Discard
}

func TestMvccConnectorDefaultPreservesDriverBeginTx(t *testing.T) {
	inner := &fakeConnBeginTx{}
	c := &MvccConnector{Inner: &fakeConnector{conn: inner}}
	conn, _ := c.Connect(context.Background())
	bt := conn.(driver.ConnBeginTx)
	if _, err := bt.BeginTx(context.Background(), driver.TxOptions{}); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if len(inner.begun) != 1 || inner.begun[0] != "driver-internal-begin" {
		t.Fatalf("default must delegate to the driver's own BeginTx, got %v", inner.begun)
	}
	if len(inner.executed) != 0 {
		t.Fatalf("default must not inject statements, got %v", inner.executed)
	}
}

func TestMvccConnectorFallbackIssuesDeferredBegin(t *testing.T) {
	inner := &fakeConn{} // no ConnBeginTx
	c := &MvccConnector{Inner: &fakeConnector{conn: inner}}
	conn, _ := c.Connect(context.Background())
	bt := conn.(driver.ConnBeginTx)
	if _, err := bt.BeginTx(context.Background(), driver.TxOptions{}); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if !reflect.DeepEqual(inner.executed, []string{"BEGIN"}) {
		t.Fatalf("fallback executed = %v, want [BEGIN]", inner.executed)
	}
}
