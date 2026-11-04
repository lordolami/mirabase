package dbexec_test

import (
	"bufio"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"mirabase/internal/dbexec"
	tursogo "turso.tech/database/tursogo"
)

// newTestConnector builds the provider connector for an engine database
// path (shared by the pool tests and the crash helper; production wiring
// in core.providerConnector additionally applies the pragma set).
func newTestConnector(path string) (driver.Connector, error) {
	dsn := path + "?_busy_timeout=10000&experimental=vacuum"
	return tursogo.NewConnector(dsn, tursogo.WithBusyTimeout(10000))
}

// TestMain doubles as the crash-helper entrypoint: re-executed with
// MB_CRASH_HELPER=1 it runs mutation transactions and ACKs each commit on
// stdout, so the parent can kill the process at an exact commit boundary
// and verify the acknowledged effects survived (AT-RECOVERY: "Kill at
// commit/WAL boundaries, reopen and compare acknowledged effects").
func TestMain(m *testing.M) {
	if os.Getenv("MB_CRASH_HELPER") != "" {
		runCrashHelper()
		return
	}
	os.Exit(m.Run())
}

func runCrashHelper() {
	dbPath := os.Getenv("MB_CRASH_DB")

	if err := mbengineEnsureForHelper(); err != nil {
		fmt.Fprintln(os.Stderr, "helper: engine ensure:", err)
		os.Exit(2)
	}
	connector, err := newTestConnector(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper: connector:", err)
		os.Exit(2)
	}
	pool, err := dbexec.NewWriterPool(dbexec.WriterPoolConfig{
		Profile:   dbexec.ProfileWAL,
		Writers:   1,
		Connector: connector,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "helper: pool:", err)
		os.Exit(2)
	}
	defer pool.Close()

	if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
		_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS c (id INTEGER PRIMARY KEY, note TEXT)`)
		return err
	}); err != nil {
		fmt.Fprintln(os.Stderr, "helper: create:", err)
		os.Exit(2)
	}
	fmt.Println("ACK table")
	for i := 0; i < 20; i++ {
		if err := pool.RunTx(context.Background(), true, func(tx *sql.Tx) error {
			if err := dbexec.PersistReceipt(tx, "crash", crashKey(i), crashHash(i), "committed", nil, 0); err != nil {
				return err
			}
			_, err := tx.Exec(`INSERT INTO c (id, note) VALUES (?, ?)`, i, "committed")
			return err
		}); err != nil {
			fmt.Fprintln(os.Stderr, "helper: tx", i, ":", err)
			os.Exit(3)
		}
		// The ACK goes out only after the commit returned success: an
		// acknowledged write must never be lost.
		fmt.Printf("ACK %d\n", i)
	}
	fmt.Println("DONE")
}

func crashKey(i int) string  { return fmt.Sprintf("key-%016d", i) }
func crashHash(i int) string { return dbexec.CanonicalRequestHash([]byte(fmt.Sprintf(`{"i":%d}`, i))) }

func TestCrashKillAtCommitBoundary(t *testing.T) {
	requireEngine(t)

	dir := t.TempDir()
	dbPath := dir + "\\crash.db"

	// The helper is this test binary re-executed with the helper
	// environment (the digest pin is compiled in by the parity harness).
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(),
		"MB_CRASH_HELPER=1",
		"MB_CRASH_DB="+dbPath,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr: %v", err)
	}
	go func() {
		b := bufio.NewScanner(stderr)
		for b.Scan() {
			t.Logf("helper: %s", b.Text())
		}
	}()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	acked := 0
	dataAcked := 0
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ACK ") {
			acked++
			if line != "ACK table" {
				// Data-transaction ACKs count separately: the helper
				// prints them only after the commit returned success.
				dataAcked++
			}
			// Kill hard right after a commit is acknowledged: the
			// acknowledged effect must survive the crash (kill at the
			// commit/WAL boundary).
			if acked >= 6 {
				_ = cmd.Process.Kill()
				break
			}
		}
		if line == "DONE" {
			t.Fatalf("helper finished before the kill")
		}
	}
	_ = cmd.Wait()

	// Reopen the database and verify the acknowledged effects: every ACK
	// printed after a successful commit must be durable (no accepted-write
	// loss), and the committed-mutation boundary must match the number of
	// committed coordinator transactions.
	raw, err := sql.Open("turso", dbPath+"?_busy_timeout=10000&experimental=vacuum")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer raw.Close()

	var rows int
	if err := raw.QueryRow(`SELECT count(*) FROM c`).Scan(&rows); err != nil {
		t.Fatalf("reopen count: %v", err)
	}
	if rows < dataAcked {
		t.Fatalf("accepted-write loss: data-acked=%d committed=%d", dataAcked, rows)
	}

	seq, err := dbexec.MutationBoundarySequence(context.Background(), raw)
	if err != nil {
		t.Fatalf("boundary: %v", err)
	}
	// +1 for the table-creation transaction.
	if seq != int64(rows+1) {
		t.Fatalf("boundary sequence %d does not match committed transactions %d (+1 create)", seq, rows)
	}
	t.Logf("crash verified: acked=%d (data %d) committed=%d boundary=%d (no accepted-write loss, WAL recovery clean)", acked, dataAcked, rows, seq)

	// Receipts for the acknowledged transactions reconcile; an in-flight
	// unacknowledged one is absent or committed — never a phantom.
	for i := 0; i < dataAcked; i++ {
		rec, err := dbexec.ReconcileReceipt(context.Background(), raw, "crash", crashKey(i), crashHash(i))
		if err != nil {
			t.Fatalf("receipt %d reconcile: %v", i, err)
		}
		if rec == nil {
			t.Fatalf("acknowledged receipt %d missing after crash", i)
		}
	}
}

var _ = errors.New
