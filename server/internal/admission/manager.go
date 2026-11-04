// Package admission owns the dynamic process and application resource
// budgets (implementation-closure.md "Resources and fair admission").
//
// v0.1 hard defaults (single application; per-application fairness within
// it is required, multi-application fairness is roadmap):
//
//	64 managed DB handles, 16 active application metadata leases,
//	8 concurrent opens, 256 actual SQL connections with 16 reserved for
//	auth/instance/recovery work, 128 active foreground queries (16 per
//	application), 2 background workers, 1024 queued jobs (128 per
//	application), five-second queue wait returning a retryable
//	resource_exhausted, 256 MiB aggregate lazy native cache budget,
//	1 GiB disk reserve, 256 MiB WAL soft warning per database.
//
// Pool maximums are ceilings, not preallocated connection promises: every
// real connection acquires an aggregate token before opening. Background
// work uses separate slots and cannot consume the recovery reservation.
package admission

import (
	"context"
	"errors"
	"time"
)

// ErrResourceExhausted is returned after the configured queue wait expires.
// It is retryable (implementation-closure.md).
var ErrResourceExhausted = errors.New("resource_exhausted")

// Default bounds (implementation-closure.md hard defaults).
const (
	DefaultMaxDbHandles         = 64
	DefaultMaxAppLeases         = 16
	DefaultMaxConcurrentOpens   = 8
	DefaultMaxSQLConnections    = 256
	DefaultReservedConnections  = 16
	DefaultMaxForegroundQueries = 128
	DefaultForegroundPerApp     = 16
	DefaultBackgroundWorkers    = 2
	DefaultMaxQueuedJobs        = 1024
	DefaultQueuedJobsPerApp     = 128
	DefaultQueueWait            = 5 * time.Second
	DefaultNativeCacheBytes     = 256 << 20 // 256 MiB
	DefaultDiskReserveBytes     = 1 << 30   // 1 GiB
	DefaultWalSoftWarningBytes  = 256 << 20 // 256 MiB per DB
)

// Bounds carries the admission limits. Zero fields fall back to defaults.
type Bounds struct {
	MaxDbHandles         int
	MaxAppLeases         int
	MaxConcurrentOpens   int
	MaxSQLConnections    int
	ReservedConnections  int
	MaxForegroundQueries int
	ForegroundPerApp     int
	BackgroundWorkers    int
	MaxQueuedJobs        int
	QueuedJobsPerApp     int
	QueueWait            time.Duration
	NativeCacheBytes     int64
	DiskReserveBytes     int64
	WalSoftWarningBytes  int64
}

// DefaultBounds returns the v0.1 hard defaults.
func DefaultBounds() Bounds {
	return Bounds{
		MaxDbHandles:         DefaultMaxDbHandles,
		MaxAppLeases:         DefaultMaxAppLeases,
		MaxConcurrentOpens:   DefaultMaxConcurrentOpens,
		MaxSQLConnections:    DefaultMaxSQLConnections,
		ReservedConnections:  DefaultReservedConnections,
		MaxForegroundQueries: DefaultMaxForegroundQueries,
		ForegroundPerApp:     DefaultForegroundPerApp,
		BackgroundWorkers:    DefaultBackgroundWorkers,
		MaxQueuedJobs:        DefaultMaxQueuedJobs,
		QueuedJobsPerApp:     DefaultQueuedJobsPerApp,
		QueueWait:            DefaultQueueWait,
		NativeCacheBytes:     DefaultNativeCacheBytes,
		DiskReserveBytes:     DefaultDiskReserveBytes,
		WalSoftWarningBytes:  DefaultWalSoftWarningBytes,
	}
}

// Release releases an acquired token. Safe to call multiple times for the
// same token (extra calls are no-ops).
type Release func()

// Manager owns dynamic process and application budgets
// (server/internal/admission.Manager per the source ownership contract).
type Manager struct {
	bounds Bounds

	pool           *semaphore // MaxSQLConnections - ReservedConnections (foreground/pool tokens)
	reserved       *semaphore // ReservedConnections (auth/instance/recovery)
	opens          *semaphore // MaxConcurrentOpens
	handles        *semaphore // MaxDbHandles
	leases         *semaphore // MaxAppLeases
	foreground     *semaphore // MaxForegroundQueries (process-wide)
	background     *semaphore // BackgroundWorkers
	queue          *semaphore // MaxQueuedJobs (global)
}

// NewManager creates a Manager with zero-fields defaulted from
// DefaultBounds.
func NewManager(b Bounds) *Manager {
	if b.MaxDbHandles <= 0 {
		b.MaxDbHandles = DefaultMaxDbHandles
	}
	if b.MaxAppLeases <= 0 {
		b.MaxAppLeases = DefaultMaxAppLeases
	}
	if b.MaxConcurrentOpens <= 0 {
		b.MaxConcurrentOpens = DefaultMaxConcurrentOpens
	}
	if b.MaxSQLConnections <= 0 {
		b.MaxSQLConnections = DefaultMaxSQLConnections
	}
	if b.ReservedConnections <= 0 {
		b.ReservedConnections = DefaultReservedConnections
	}
	if b.MaxForegroundQueries <= 0 {
		b.MaxForegroundQueries = DefaultMaxForegroundQueries
	}
	if b.ForegroundPerApp <= 0 {
		b.ForegroundPerApp = DefaultForegroundPerApp
	}
	if b.BackgroundWorkers <= 0 {
		b.BackgroundWorkers = DefaultBackgroundWorkers
	}
	if b.MaxQueuedJobs <= 0 {
		b.MaxQueuedJobs = DefaultMaxQueuedJobs
	}
	if b.QueuedJobsPerApp <= 0 {
		b.QueuedJobsPerApp = DefaultQueuedJobsPerApp
	}
	if b.QueueWait <= 0 {
		b.QueueWait = DefaultQueueWait
	}
	if b.NativeCacheBytes <= 0 {
		b.NativeCacheBytes = DefaultNativeCacheBytes
	}
	if b.DiskReserveBytes <= 0 {
		b.DiskReserveBytes = DefaultDiskReserveBytes
	}
	if b.WalSoftWarningBytes <= 0 {
		b.WalSoftWarningBytes = DefaultWalSoftWarningBytes
	}
	poolCap := b.MaxSQLConnections - b.ReservedConnections
	if poolCap < 0 {
		poolCap = 0
	}
	return &Manager{
		bounds:      b,
		pool:        newSemaphore(poolCap),
		reserved:    newSemaphore(b.ReservedConnections),
		opens:       newSemaphore(b.MaxConcurrentOpens),
		handles:     newSemaphore(b.MaxDbHandles),
		leases:      newSemaphore(b.MaxAppLeases),
		foreground:  newSemaphore(b.MaxForegroundQueries),
		background:  newSemaphore(b.BackgroundWorkers),
		queue:       newSemaphore(b.MaxQueuedJobs),
	}
}

// Bounds returns the effective bounds.
func (m *Manager) Bounds() Bounds { return m.bounds }

// acquire tries the semaphore immediately; on failure it waits up to the
// queue wait with context cancellation, then fails with
// ErrResourceExhausted (retryable).
func acquire(ctx context.Context, s *semaphore, wait time.Duration) (Release, error) {
	if r, ok := s.tryAcquire(); ok {
		return r, nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	// Poll the semaphore on a bounded ticker so a released slot is picked
	// up without a condition variable.
	tick := time.NewTicker(500 * time.Microsecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, ErrResourceExhausted
		case <-tick.C:
			if r, ok := s.tryAcquire(); ok {
				return r, nil
			}
		}
	}
}

// AcquireConnection acquires an aggregate SQL connection token before a
// real connection is opened. Reserved-class work (auth, instance,
// recovery) draws from the reserved pool and never from background work.
// Pool maximums are ceilings, not preallocated promises.
func (m *Manager) AcquireConnection(ctx context.Context, reservedClass bool) (Release, error) {
	if reservedClass {
		return acquire(ctx, m.reserved, m.bounds.QueueWait)
	}
	return acquire(ctx, m.pool, m.bounds.QueueWait)
}

// AcquireOpen bounds concurrent database opens.
func (m *Manager) AcquireOpen(ctx context.Context) (Release, error) {
	return acquire(ctx, m.opens, m.bounds.QueueWait)
}

// AcquireHandle bounds managed database handles.
func (m *Manager) AcquireHandle(ctx context.Context) (Release, error) {
	return acquire(ctx, m.handles, m.bounds.QueueWait)
}

// AcquireLease bounds active application metadata leases.
func (m *Manager) AcquireLease(ctx context.Context) (Release, error) {
	return acquire(ctx, m.leases, m.bounds.QueueWait)
}

// AcquireForegroundQuery bounds active foreground queries process-wide.
// Per-application fairness (round-robin among applications with one FIFO
// queue per app) is layered by the caller; this enforces the hard ceiling.
func (m *Manager) AcquireForegroundQuery(ctx context.Context) (Release, error) {
	return acquire(ctx, m.foreground, m.bounds.QueueWait)
}

// AcquireBackground bounds background workers on separate slots; background
// work cannot consume the recovery reservation.
func (m *Manager) AcquireBackground(ctx context.Context) (Release, error) {
	return acquire(ctx, m.background, m.bounds.QueueWait)
}

// AcquireJobQueue bounds globally queued jobs.
func (m *Manager) AcquireJobQueue(ctx context.Context) (Release, error) {
	return acquire(ctx, m.queue, m.bounds.QueueWait)
}

// semaphore is a minimal counting semaphore with non-blocking tryAcquire.
type semaphore struct {
	ch chan struct{}
}

func newSemaphore(n int) *semaphore {
	if n < 1 {
		n = 1
	}
	return &semaphore{ch: make(chan struct{}, n)}
}

func (s *semaphore) tryAcquire() (Release, bool) {
	select {
	case s.ch <- struct{}{}:
		var once bool
		return func() {
			if once {
				return
			}
			once = true
			<-s.ch
		}, true
	default:
		return nil, false
	}
}
