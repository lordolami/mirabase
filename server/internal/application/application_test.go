package application_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mirabase/core"
	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/tests"
)

// requireEngine skips when the pinned payload is not compiled in
// (parity_turso.sh passes the digest pin).
func requireEngine(t *testing.T) {
	t.Helper()
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure failed: %v", err)
	}
}

// openPrimary opens the primary registry database on the engine.
func openPrimary(t *testing.T, dir string) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(dir, "primary.db")
	raw, err := sql.Open("turso", path+"?_busy_timeout=10000&experimental=vacuum")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	if _, err := raw.Conn(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return raw, path
}

func newService(t *testing.T, dir string) (*application.Service, *sql.DB) {
	t.Helper()
	db, _ := openPrimary(t, dir)
	s, err := application.NewService(db)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return s, db
}

func publishProvisionFixture(t *testing.T, s *application.Service, id string) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	coll := core.NewBaseCollection("provision_notes")
	coll.Fields.Add(&core.TextField{Name: "body"})
	if err := app.Save(coll); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PublishTemplate(context.Background(), app, id, []string{coll.Name}); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryDDLAndDatabaseKinds(t *testing.T) {
	requireEngine(t)
	s, db := newService(t, t.TempDir())

	// The primary registry row itself (kind=primary, reserved id).
	if _, err := db.Exec(`INSERT INTO _mb_databases (id, owner_collection_id, owner_record_id, kind, state, generation, profile, template_id, schema_revision, policy_epoch, display_name, created, updated)
		VALUES ('primary', NULL, NULL, 'primary', 'ready', 1, 'wal', NULL, 1, 1, 'Primary', ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("primary row: %v", err)
	}
	d, err := s.GetDatabase(context.Background(), "primary")
	if err != nil || d.Kind != "primary" || d.State != "ready" {
		t.Fatalf("primary row: %+v %v", d, err)
	}

	// State CHECK enforcement: an invalid state must be rejected by the DDL.
	if _, err := db.Exec(`UPDATE _mb_databases SET state = 'shredded' WHERE id = 'primary'`); err == nil {
		t.Fatalf("the state CHECK constraint did not fire")
	}

	// Owner-null pair constraint: owner fields must be null together.
	if _, err := db.Exec(`UPDATE _mb_databases SET owner_collection_id = 'c1' WHERE id = 'primary'`); err == nil {
		t.Fatalf("the owner-pair CHECK constraint did not fire")
	}
}

func TestProvisionConvergesConcurrentRequestsOnOneBinding(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	s, _ := newService(t, dir)
	publishProvisionFixture(t, s, "tpl-basic")

	// The gate-4 acceptance scenario shape: two auth collections with
	// COLLIDING record IDs — the same record id in two collections must
	// not collide.
	const collidingID = "pbc_10000000000001"

	var results []*application.Database
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d, created, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
				Kind:              "user",
				TemplateID:        "tpl-basic",
				OwnerCollectionID: "users_a",
				OwnerRecordID:     collidingID,
				DisplayName:       "alice",
			})
			mu.Lock()
			defer mu.Unlock()
			if d != nil {
				results = append(results, d)
			}
			errs = append(errs, err)
			_ = created
		}()
	}
	wg.Wait()

	ids := map[string]bool{}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent provision %d: %v", i, err)
		}
	}
	for _, d := range results {
		ids[d.ID] = true
	}
	if len(ids) != 1 {
		t.Fatalf("concurrent first requests produced %d distinct bindings, want exactly 1 (no duplicate files): %v", len(ids), ids)
	}

	// The colliding ID in the second auth collection is a distinct binding.
	d2, created, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind:              "user",
		TemplateID:        "tpl-basic",
		OwnerCollectionID: "users_b",
		OwnerRecordID:     collidingID,
		DisplayName:       "bob",
	})
	if err != nil || d2 == nil {
		t.Fatalf("second collection provision: %v %v", d2, err)
	}
	if !created {
		t.Fatalf("second collection should provision its own database")
	}
	if d2.ID == results[0].ID {
		t.Fatalf("colliding record IDs across auth collections collided into one database")
	}

	// The private file layout exists for both (mb_data/databases/<id>/).
	for _, id := range []string{results[0].ID, d2.ID} {
		m, err := s.ReadManifest(dir, id)
		if err != nil {
			t.Fatalf("manifest %s: %v", id, err)
		}
		if m.FormatVersion != 1 || m.DatabaseID != id || m.Generation != "1" {
			t.Fatalf("manifest mismatch: %+v", m)
		}
	}
}

func TestLifecycleQuarantineRestoreTombstonePurge(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	s, _ := newService(t, dir)
	publishProvisionFixture(t, s, "tpl-agent")

	d, _, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind: "agent", TemplateID: "tpl-agent",
		OwnerCollectionID: "agents", OwnerRecordID: "agent1", DisplayName: "agent db",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Quarantine: files and generation intact.
	if err := s.Quarantine(context.Background(), d.ID); err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	q, _ := s.GetDatabase(context.Background(), d.ID)
	if q.State != "quarantined" || q.Generation != 1 {
		t.Fatalf("quarantined state/gen: %+v", q)
	}
	if _, err := s.ReadManifest(dir, d.ID); err != nil {
		t.Fatalf("quarantine must keep the files: %v", err)
	}

	// Restore: new generation (generation changes revoke old leases).
	if err := s.Restore(context.Background(), d.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	r, _ := s.GetDatabase(context.Background(), d.ID)
	if r.State != "ready" || r.Generation != 2 {
		t.Fatalf("restored state/gen: %+v", r)
	}

	// Tombstone + purge before retention: the purge must skip it.
	if err := s.Tombstone(context.Background(), d.ID); err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	purged, err := s.PurgeTombstoned(context.Background(), dir, 24*time.Hour, time.Now().UTC())
	if err != nil {
		t.Fatalf("purge (within retention): %v", err)
	}
	if len(purged) != 0 {
		t.Fatalf("purged inside the retention window: %v", purged)
	}
	if _, err := os.Stat(filepath.Join(dir, "databases", d.ID)); err != nil {
		t.Fatalf("retention must keep the trash: %v", err)
	}

	// After retention: purge removes the files and marks purged.
	purged, err = s.PurgeTombstoned(context.Background(), dir, 24*time.Hour, time.Now().UTC().Add(25*time.Hour))
	if err != nil || len(purged) != 1 {
		t.Fatalf("purge after retention: %v %v", purged, err)
	}
	p, _ := s.GetDatabase(context.Background(), d.ID)
	if p.State != "purged" {
		t.Fatalf("state after purge: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "databases", d.ID)); !os.IsNotExist(err) {
		t.Fatalf("purged files remain: %v", err)
	}
}

func TestGrantsAuthorizeAndRevokeAdvancesEpoch(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	s, _ := newService(t, dir)

	d, _, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind: "user", OwnerCollectionID: "users", OwnerRecordID: "alice", DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Owner is authorized by the binding; a third party is not. The
	// selector header is required for the context path to be exercised.
	res := httptest.NewRequest("GET", "/", nil)
	res.Header.Set(application.HeaderDatabase, d.ID)
	r := application.NewResolver(s)
	if _, err := r.ResolveHTTP(res.WithContext(context.Background()), "users", "alice"); err != nil {
		t.Fatalf("owner resolve: %v", err)
	}
	if _, err := r.ResolveHTTP(res.WithContext(context.Background()), "users", "bob"); !errors.Is(err, application.ErrUnauthorizedDatabase) {
		t.Fatalf("unauthorized resolve must fail without fallback: %v", err)
	}

	// A grant authorizes the subject.
	g, err := s.SetGrant(context.Background(), d.ID, "users", "bob",
		[]string{application.ScopeRecordRead, application.ScopeDatabaseInspect}, nil)
	if err != nil {
		t.Fatalf("set grant: %v", err)
	}
	if len(g.Scopes) != 2 || g.Epoch != 1 {
		t.Fatalf("grant shape: %+v", g)
	}
	// Scopes are sorted.
	if g.Scopes[0] != application.ScopeDatabaseInspect {
		t.Fatalf("scopes must be sorted: %v", g.Scopes)
	}
	authedReq := httptest.NewRequest("GET", "/", nil)
	authedReq.Header.Set(application.HeaderDatabase, d.ID)
	if _, err := r.ResolveHTTP(authedReq.WithContext(context.Background()), "users", "bob"); err != nil {
		t.Fatalf("granted resolve: %v", err)
	}

	// The v0.1 scope narrowing: changes.read is not grantable.
	if _, err := s.SetGrant(context.Background(), d.ID, "users", "carol",
		[]string{"changes.read"}, nil); err == nil {
		t.Fatalf("changes.read must be rejected in v0.1")
	}

	// Revocation advances the epoch and re-checks fail.
	if err := s.RevokeGrant(context.Background(), d.ID, "users", "bob"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked, _ := s.GetGrant(context.Background(), d.ID, "users", "bob")
	if revoked.RevokedAt == nil || revoked.Epoch != 2 {
		t.Fatalf("revocation must advance the epoch: %+v", revoked)
	}
	if revoked.HasScope(application.ScopeRecordRead) {
		t.Fatalf("revoked grant still carries scopes")
	}
	rejectReq := httptest.NewRequest("GET", "/", nil)
	rejectReq.Header.Set(application.HeaderDatabase, d.ID)
	if _, err := r.ResolveHTTP(rejectReq.WithContext(context.Background()), "users", "bob"); !errors.Is(err, application.ErrUnauthorizedDatabase) {
		t.Fatalf("revoked subject must fail again: %v", err)
	}
}

func TestAgentsBindRevokeAndOwnerIsolation(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	s, _ := newService(t, dir)

	// Alice owns a database and registers two agents.
	d, _, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind: "user", OwnerCollectionID: "users", OwnerRecordID: "alice", DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	id1, err := s.BindAgent(context.Background(), "agents", "agent1", "users", "alice")
	if err != nil || id1 == "" {
		t.Fatalf("bind agent1: %v %v", id1, err)
	}
	if _, err := s.BindAgent(context.Background(), "agents", "agent2", "users", "alice"); err != nil {
		t.Fatalf("bind agent2: %v", err)
	}

	a1, err := s.GetAgent(context.Background(), "agents", "agent1")
	if err != nil || a1.RegistrationState != "active" || a1.OwnerRecordID != "alice" {
		t.Fatalf("agent1 binding: %+v %v", a1, err)
	}

	// Agents get their own databases; the agent database is bound to the
	// agent identity, NOT the owner's database.
	ad, _, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind: "agent", OwnerCollectionID: "agents", OwnerRecordID: "agent1", DisplayName: "agent1 db",
	})
	if err != nil || ad.ID == d.ID {
		t.Fatalf("agent database must be separate from the owner's: %s vs %s", ad.ID, d.ID)
	}

	// Unique agent binding: the same agent identity is one row.
	if _, err := s.BindAgent(context.Background(), "agents", "agent1", "users", "someone-else"); err != nil {
		t.Fatalf("rebind: %v", err)
	}
	a1b, _ := s.GetAgent(context.Background(), "agents", "agent1")
	if a1b.OwnerRecordID != "someone-else" {
		t.Fatalf("rebind did not take: %+v", a1b)
	}

	// Revocation: the agent binding is revoked; the owner's database is
	// untouched ("Deleting an agent affects only its grants/bindings").
	if err := s.RevokeAgent(context.Background(), "agents", "agent1"); err != nil {
		t.Fatalf("revoke agent: %v", err)
	}
	a1c, _ := s.GetAgent(context.Background(), "agents", "agent1")
	if a1c.RegistrationState != "revoked" {
		t.Fatalf("agent state: %+v", a1c)
	}
	owner, _ := s.GetDatabase(context.Background(), d.ID)
	if owner.State != "ready" {
		t.Fatalf("owner database must be untouched: %+v", owner)
	}
}

func TestOwnerTransferPhases(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	s, _ := newService(t, dir)

	d, _, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind: "user", OwnerCollectionID: "users", OwnerRecordID: "alice", DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	// Intent with generation guard.
	id, err := s.StartOwnerTransfer(context.Background(), d.ID, d.Generation,
		map[string]any{"collectionId": "users", "recordId": "bob"}, 1, "revoke")
	if err != nil {
		t.Fatalf("start transfer: %v", err)
	}
	// Phase progression: intent → fenced → revoked → bound → complete.
	for _, phase := range []string{"fenced", "revoked", "bound", "complete"} {
		if err := s.AdvanceOwnerTransfer(context.Background(), id, phase); err != nil {
			t.Fatalf("phase %s: %v", phase, err)
		}
	}

	// Invalid agent policy is rejected at intent time.
	if _, err := s.StartOwnerTransfer(context.Background(), d.ID, d.Generation,
		map[string]any{}, 1, "wipe-everything"); err == nil {
		t.Fatalf("invalid agent policy accepted")
	}
}

func TestContextResolutionRules(t *testing.T) {
	requireEngine(t)
	dir := t.TempDir()
	s, _ := newService(t, dir)
	r := application.NewResolver(s)

	// No selectors → immutable default context.
	req := httptest.NewRequest("GET", "/", nil)
	c, err := r.ResolveHTTP(req, "users", "alice")
	if err != nil || !c.IsPrimary() || c.ApplicationID != "default" {
		t.Fatalf("default context: %+v %v", c, err)
	}

	// Unknown database selector never falls back to primary.
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set(application.HeaderDatabase, "deadbeefdeadbeefdeadbeefdeadbeef")
	if _, err := r.ResolveHTTP(req, "users", "alice"); !errors.Is(err, application.ErrUnknownDatabase) {
		t.Fatalf("unknown selector: %v", err)
	}

	// Both selectors at once is rejected.
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set(application.HeaderDatabase, "deadbeefdeadbeefdeadbeefdeadbeef")
	req.Header.Set(application.HeaderTenant, "school/a")
	if _, err := r.ResolveHTTP(req, "users", "alice"); err == nil {
		t.Fatalf("both selectors accepted")
	}

	// A granted subject resolves a real database context.
	d, _, err := s.Provision(context.Background(), dir, application.ProvisionRequest{
		Kind: "user", OwnerCollectionID: "users", OwnerRecordID: "alice", DisplayName: "alice",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set(application.HeaderDatabase, d.ID)
	c, err = r.ResolveHTTP(req, "users", "alice")
	if err != nil || c.IsPrimary() || c.TargetDatabaseID() != d.ID {
		t.Fatalf("database context: %+v %v", c, err)
	}

	// The context travels through the request context.
	attached := application.FromContext(application.WithContext(context.Background(), c))
	if attached.TargetDatabaseID() != d.ID {
		t.Fatalf("context propagation failed: %+v", attached)
	}
	// A missing value is the default.
	if got := application.FromContext(context.Background()); !got.IsPrimary() {
		t.Fatalf("missing context is not the default: %+v", got)
	}
}

var _ = fmt.Sprintf
var _ = sql.ErrNoRows
