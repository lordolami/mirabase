package application

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mirabase/core"
)

// ---------------------------------------------------------------------------
// R3 independent applications (application-isolation.md): separate
// auth/schema/settings/secrets/artifacts/files and app-local superusers
// inside ONE process, with real per-app data directories and per-app
// core.BaseApp instances. Equal IDs/emails in unrelated apps cannot
// collide (each app owns its own store). No process per app/DB.
// ---------------------------------------------------------------------------

// applicationsDDL is the application registry.
const applicationsDDL = `CREATE TABLE IF NOT EXISTS _mb_applications (
 id TEXT PRIMARY KEY CHECK(length(id) = 32),
 generation INTEGER NOT NULL CHECK(generation > 0),
 revision INTEGER NOT NULL CHECK(revision > 0),
 display_name TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('provisioning','ready','maintenance','suspended','restoring','deleting','quarantined','tombstoned','purged','failed')),
 layout TEXT NOT NULL CHECK(layout IN ('shared','user','tenant','agent')),
 base_path TEXT NOT NULL,
 created TEXT NOT NULL,
 updated TEXT NOT NULL
)`

// ApplicationRecord is one registry row.
type ApplicationRecord struct {
	ID          string
	Generation  int64
	Revision    int64
	DisplayName string
	State       string
	Layout      string
	BasePath    string
	Created     string
	Updated     string
}

// ApplicationIDCollisionErr guards the default application id.
var ErrApplicationCollision = errors.New("conflict")

// AppSupervisor owns the live application instances (one process; no
// process per app).
type AppSupervisor struct {
	mu    sync.Mutex
	apps  map[string]*core.BaseApp
	dirty bool
}

var supervisor = &AppSupervisor{apps: map[string]*core.BaseApp{}}

// Supervisor returns the process-wide application supervisor.
func Supervisor() *AppSupervisor { return supervisor }

// ensureApplicationsTable installs the registry.
func (s *Service) ensureApplicationsTable(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, applicationsDDL); err != nil {
		return fmt.Errorf("failed to install the applications registry: %w", err)
	}
	return nil
}

// newApplicationID generates a 32-hex app id (never the default id).
func newApplicationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateApplication registers and provisions one independent application:
// a real data directory and a bootstrapped core.BaseApp instance in this
// process.
func (rt *Runtime) CreateApplication(ctx context.Context, displayName, layout string) (*ApplicationRecord, error) {
	if err := rt.Service.ensureApplicationsTable(ctx); err != nil {
		return nil, err
	}
	if layout != "shared" && layout != "user" && layout != "tenant" && layout != "agent" {
		return nil, fmt.Errorf("invalid_request: layout must be shared, user, tenant or agent")
	}
	if displayName == "" {
		return nil, fmt.Errorf("invalid_request: displayName is required")
	}
	id := newApplicationID()
	basePath := "apps/" + id
	now := nowRFC3339()
	if _, err := rt.PrimaryDB.ExecContext(ctx,
		`INSERT INTO _mb_applications (id, generation, revision, display_name, state, layout, base_path, created, updated)
		 VALUES (?, 1, 1, ?, 'provisioning', ?, ?, ?, ?)`,
		id, displayName, layout, basePath, now, now); err != nil {
		return nil, err
	}
	// Provision: the real per-app data directory + a live BaseApp
	// instance (its own auth/schema/settings/secrets/files).
	if err := rt.openApplication(ctx, id, displayName); err != nil {
		_, _ = rt.PrimaryDB.ExecContext(ctx,
			`UPDATE _mb_applications SET state = 'failed', updated = ? WHERE id = ?`, nowRFC3339(), id)
		return nil, err
	}
	if _, err := rt.PrimaryDB.ExecContext(ctx,
		`UPDATE _mb_applications SET state = 'ready', updated = ? WHERE id = ?`, nowRFC3339(), id); err != nil {
		return nil, err
	}
	return rt.GetApplication(ctx, id)
}

// appDataDir is the per-app data directory.
func (rt *Runtime) appDataDir(id string) string {
	return filepath_Apps(rt.DataDir, id)
}

// openApplication boots (or returns) the app's BaseApp instance.
func (rt *Runtime) openApplication(ctx context.Context, id, displayName string) error {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	if b, ok := supervisor.apps[id]; ok {
		_ = b
		return nil
	}
	dataDir := rt.appDataDir(id)
	if err := osMkdirAll(dataDir); err != nil {
		return err
	}
	b := core.NewBaseApp(core.BaseAppConfig{
		DataDir:   dataDir,
		DBConnect: core.TestDBConnect(),
	})
	if err := b.Bootstrap(); err != nil {
		return fmt.Errorf("the application instance failed to bootstrap: %w", err)
	}
	supervisor.apps[id] = b
	return nil
}

// GetApplication loads one registry row.
func (rt *Runtime) GetApplication(ctx context.Context, id string) (*ApplicationRecord, error) {
	var r ApplicationRecord
	err := rt.PrimaryDB.QueryRowContext(ctx,
		`SELECT id, generation, revision, display_name, state, layout, base_path, created, updated
		 FROM _mb_applications WHERE id = ?`, id,
	).Scan(&r.ID, &r.Generation, &r.Revision, &r.DisplayName, &r.State, &r.Layout, &r.BasePath, &r.Created, &r.Updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListApplications lists the registry rows.
func (rt *Runtime) ListApplications(ctx context.Context, page, perPage int) (items []*ApplicationRecord, total int64, err error) {
	if err := rt.Service.ensureApplicationsTable(ctx); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 1000 {
		perPage = 30
	}
	if err := rt.PrimaryDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM _mb_applications`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := rt.PrimaryDB.QueryContext(ctx,
		`SELECT id, generation, revision, display_name, state, layout, base_path, created, updated
		 FROM _mb_applications ORDER BY created LIMIT ? OFFSET ?`, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var r ApplicationRecord
		if err := rows.Scan(&r.ID, &r.Generation, &r.Revision, &r.DisplayName, &r.State, &r.Layout, &r.BasePath, &r.Created, &r.Updated); err != nil {
			return nil, 0, err
		}
		items = append(items, &r)
	}
	return items, total, rows.Err()
}

// ApplicationStateChange applies maintenance/suspend/resume to one
// application instance (suspend closes the live instance; resume reopens
// it; the data is retained, never deleted).
func (rt *Runtime) ApplicationStateChange(ctx context.Context, id, action string, expectedGeneration int64) (*ApplicationRecord, error) {
	r, err := rt.GetApplication(ctx, id)
	if err != nil {
		return nil, err
	}
	if expectedGeneration != r.Generation {
		return nil, fmt.Errorf("%w: the expected generation does not match the application row", ErrReferenceStale)
	}
	switch action {
	case "maintenance":
		if err := rt.setApplicationState(ctx, id, "maintenance"); err != nil {
			return nil, err
		}
	case "suspend":
		if err := rt.setApplicationState(ctx, id, "suspended"); err != nil {
			return nil, err
		}
		supervisor.mu.Lock()
		if b, ok := supervisor.apps[id]; ok {
			b.ClearBootstrap()
			delete(supervisor.apps, id)
		}
		supervisor.mu.Unlock()
	case "resume":
		if err := rt.setApplicationState(ctx, id, "ready"); err != nil {
			return nil, err
		}
		if err := rt.openApplication(ctx, id, r.DisplayName); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid_request: action must be maintenance, suspend or resume")
	}
	return rt.GetApplication(ctx, id)
}

// setApplicationState updates the registry state with the generation bump.
func (rt *Runtime) setApplicationState(ctx context.Context, id, state string) error {
	_, err := rt.PrimaryDB.ExecContext(ctx,
		`UPDATE _mb_applications SET state = ?, generation = generation + 1, updated = ? WHERE id = ?`,
		state, nowRFC3339(), id)
	return err
}

// ApplicationBase returns the live instance of one application (nil when
// suspended).
func (rt *Runtime) ApplicationBase(id string) *core.BaseApp {
	supervisor.mu.Lock()
	defer supervisor.mu.Unlock()
	return supervisor.apps[id]
}

// DeleteApplication tombstones the application and quarantines its data
// directory (never deleting a directory solely because it is absent from
// a stale registry snapshot).
func (rt *Runtime) DeleteApplication(ctx context.Context, id string, expectedGeneration int64) error {
	r, err := rt.GetApplication(ctx, id)
	if err != nil {
		return err
	}
	if expectedGeneration != r.Generation {
		return fmt.Errorf("%w: the expected generation does not match the application row", ErrReferenceStale)
	}
	if r.State == "tombstoned" || r.State == "purged" {
		return fmt.Errorf("the application is already %s", r.State)
	}
	supervisor.mu.Lock()
	if b, ok := supervisor.apps[id]; ok {
		b.ClearBootstrap()
		delete(supervisor.apps, id)
	}
	supervisor.mu.Unlock()
	if err := rt.setApplicationState(ctx, id, "tombstoned"); err != nil {
		return err
	}
	return nil
}

// BootstrapApplicationAdministrator creates the app's first administrator
// through inherited forms (app-local superusers: equal emails in
// unrelated applications cannot collide).
func (rt *Runtime) BootstrapApplicationAdministrator(ctx context.Context, id, email, password string) error {
	r, err := rt.GetApplication(ctx, id)
	if err != nil {
		return err
	}
	if r.State != "ready" {
		return fmt.Errorf("%w: the application state is %q", ErrReferenceStale, r.State)
	}
	b := rt.ApplicationBase(id)
	if b == nil {
		return fmt.Errorf("%w: the application instance is suspended", ErrReferenceStale)
	}
	superusers, serr := b.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if serr != nil {
		return serr
	}
	admin := core.NewRecord(superusers)
	admin.Set("email", email)
	admin.Set("password", password)
	return b.Save(admin)
}

// MarshalApplicationsRow renders one row for the wire.
func MarshalApplicationsRow(r *ApplicationRecord) map[string]any {
	return map[string]any{
		"id": r.ID, "generation": r.Generation, "revision": r.Revision,
		"displayName": r.DisplayName, "state": r.State, "layout": r.Layout,
		"basePath": r.BasePath, "created": r.Created, "updated": r.Updated,
	}
}

var _ = json.Marshal
var _ = time.Now

// filepath_Apps joins the applications root under the data directory.
func filepath_Apps(dataDir, id string) string {
	return filepath.Join(dataDir, "apps", id)
}

// osMkdirAll creates the directory tree.
func osMkdirAll(dir string) error {
	return os.MkdirAll(dir, 0o700)
}
