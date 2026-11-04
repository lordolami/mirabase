package core_test

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mirabase/core"
	"mirabase/tests"
	tursogo "turso.tech/database/tursogo"
)

// Exercise real native export while another connection commits writes. Every
// extracted archive must independently open and retain the two-row invariant.
func TestCreateBackupConcurrentWritesPreserveSnapshot(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	if os.Getenv("MIRABASE_NATIVE_TRACE") == "1" {
		if err := tursogo.Setup(tursogo.TursoConfig{LogLevel: "debug", Logger: func(event tursogo.TursoLog) {
			if strings.Contains(event.Message, "reprepare") || strings.Contains(event.Message, "VACUUM") || strings.Contains(event.Message, "Busy") || strings.Contains(event.Message, "Schema") {
				t.Logf("native trace: %s", event.Message)
			}
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.DB().NewQuery(`CREATE TABLE backup_pair (id INTEGER PRIMARY KEY, value INTEGER NOT NULL)`).Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB().NewQuery(`INSERT INTO backup_pair VALUES(1,0),(2,0)`).Execute(); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				done <- nil
				return
			case <-ticker.C:
				if _, err := app.NonconcurrentDB().NewQuery(`UPDATE backup_pair SET value=value+1`).Execute(); err != nil {
					done <- err
					return
				}
			}
		}
	}()
	defer func() {
		close(stop)
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("concurrent writer: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("concurrent writer did not stop")
		}
	}()
	for i := range 8 {
		name := fmt.Sprintf("concurrent-%02d.zip", i)
		if err := app.CreateBackup(context.Background(), name); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
		z, err := zip.OpenReader(filepath.Join(app.DataDir(), core.LocalBackupsDirName, name))
		if err != nil {
			t.Fatal(err)
		}
		reader, err := z.Open("data.db")
		if err != nil {
			z.Close()
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "snapshot.db")
		file, err := os.Create(path)
		if err != nil {
			reader.Close()
			z.Close()
			t.Fatal(err)
		}
		_, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		reader.Close()
		z.Close()
		if copyErr != nil || closeErr != nil {
			t.Fatalf("copy archive: %v %v", copyErr, closeErr)
		}
		db, err := core.TestDBConnect()(path)
		if err != nil {
			t.Fatal(err)
		}
		var count, minimum, maximum int
		err = db.NewQuery(`SELECT count(*),min(value),max(value) FROM backup_pair`).Row(&count, &minimum, &maximum)
		db.Close()
		if err != nil || count != 2 || minimum != maximum {
			t.Fatalf("backup %d is not a coherent snapshot: count=%d min=%d max=%d err=%v", i, count, minimum, maximum, err)
		}
	}
}
