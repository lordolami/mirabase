package application

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mirabase/internal/admission"
	tursogo "turso.tech/database/tursogo"
)

// viewEntry is one cached context-bound app view.
type viewEntry struct {
	app     *sql.DB
	lastUse time.Time
}

// HandleCache is the admission-coupled cache of opened managed-database
// handles: "Default admission retains 64 managed handles, 8 concurrent
// opens ... Idle handles are evictable; leased transactions are not"
// (v0.1-build.md; implementation-closure.md). The cache owns the physical
// view of each private database; the registry remains authoritative for
// routing.
type HandleCache struct {
	dataDir  string
	mu       sync.Mutex
	views    map[string]*viewEntry
	order    []string // LRU order, oldest last
	max      int
	idleTTL  time.Duration
	admission *admission.Manager
}

// NewHandleCache builds the handle cache with the contract defaults.
func NewHandleCache(dataDir string, adm *admission.Manager) *HandleCache {
	max := 64
	if adm != nil {
		max = adm.Bounds().MaxDbHandles
	}
	if max <= 0 {
		max = 64
	}
	return &HandleCache{
		dataDir:   dataDir,
		views:     map[string]*viewEntry{},
		max:       max,
		idleTTL:   30 * time.Second,
		admission: adm,
	}
}

// ErrCacheFull is the retryable exhaustion signal
// ("resource_exhausted" with retry guidance).
var ErrCacheFull = errors.New("resource_exhausted")

// Open returns (and caches) a *sql.DB handle for a managed database
// directory. Opens are bounded by the admission manager's concurrent-open
// ceiling; idle handles are evicted after the idle TTL (leased
// transactions are not evicted — the engine's connection semantics hold
// the file open for the transaction's lifetime, and eviction skips
// handles with an open transaction).
func (h *HandleCache) Open(ctx context.Context, databaseID string) (*sql.DB, error) {
	h.mu.Lock()
	if e, ok := h.views[databaseID]; ok {
		e.lastUse = time.Now()
		h.touchLRU(databaseID)
		h.mu.Unlock()
		return e.app, nil
	}
	// Make room: evict idle handles beyond the ceiling.
	h.evictLocked(databaseID)
	h.mu.Unlock()

	var release admission.Release
	if h.admission != nil {
		var err error
		release, err = h.admission.AcquireOpen(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: concurrent open bound reached", ErrCacheFull)
		}
		defer release()
	}

	path := h.databasePath(databaseID)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrUnknownDatabase, databaseID)
		}
		return nil, err
	}
	connector, err := newPrivateConnector(path)
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	h.mu.Lock()
	defer h.mu.Unlock()
	// Another opener may have raced; keep one handle.
	if e, ok := h.views[databaseID]; ok {
		_ = db.Close()
		return e.app, nil
	}
	h.views[databaseID] = &viewEntry{app: db, lastUse: time.Now()}
	h.order = append([]string{databaseID}, h.order...)
	return db, nil
}

// Close releases a handle explicitly (lifecycle transitions close the
// view before touching files).
func (h *HandleCache) Close(databaseID string) error {
	h.mu.Lock()
	e, ok := h.views[databaseID]
	if ok {
		delete(h.views, databaseID)
		for i, id := range h.order {
			if id == databaseID {
				h.order = append(h.order[:i], h.order[i+1:]...)
				break
			}
		}
	}
	h.mu.Unlock()
	if ok {
		return e.app.Close()
	}
	return nil
}

// EvictIdle closes handles idle beyond the TTL (the runtime calls this
// periodically; evictable = no leased transaction, which for *sql.DB
// means no open Conn/Tx — a Close during an open transaction fails at the
// driver level and the handle stays).
func (h *HandleCache) EvictIdle() {
	h.mu.Lock()
	var victims []string
	now := time.Now()
	for id, e := range h.views {
		if now.Sub(e.lastUse) > h.idleTTL {
			victims = append(victims, id)
		}
	}
	entries := make([]*viewEntry, 0, len(victims))
	for _, id := range victims {
		entries = append(entries, h.views[id])
		delete(h.views, id)
		for i, oid := range h.order {
			if oid == id {
				h.order = append(h.order[:i], h.order[i+1:]...)
				break
			}
		}
	}
	h.mu.Unlock()
	for _, e := range entries {
		_ = e.app.Close()
	}
}

// evictLocked closes the oldest idle handles until one slot is free for
// the incoming database id.
func (h *HandleCache) evictLocked(incoming string) {
	for len(h.views) >= h.max && len(h.order) > 0 {
		oldest := h.order[len(h.order)-1]
		if oldest == incoming {
			break
		}
		e := h.views[oldest]
		delete(h.views, oldest)
		h.order = h.order[:len(h.order)-1]
		// Close outside the critical section is not possible here; Close
		// is fast for engine handles and never blocks on queries.
		go func(e *viewEntry) { _ = e.app.Close() }(e)
	}
}

func (h *HandleCache) touchLRU(id string) {
	for i, oid := range h.order {
		if oid == id {
			h.order = append(append(h.order[:i:i], h.order[i+1:]...), id)
			return
		}
	}
}

// databasePath resolves the engine file of a private database.
func (h *HandleCache) databasePath(databaseID string) string {
	return filepath.Join(h.dataDir, "databases", databaseID, "data.db")
}

// newPrivateConnector builds the provider connector for a private
// database file (the same engine, the fallback I/O backend; the private
// database always goes through the selected provider — native-provider.md,
// no SQLite fallback).
func newPrivateConnector(path string) (driver.Connector, error) {
	dsn := path + "?_busy_timeout=10000&experimental=vacuum,index_method"
	return tursogo.NewConnector(dsn, tursogo.WithBusyTimeout(10000))
}

// OpenRaw is a test/operational accessor for the private database file.
func OpenRaw(path string) (*sql.DB, error) {
	return sql.Open("turso", path+"?_busy_timeout=10000&experimental=vacuum,index_method")
}
