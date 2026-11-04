package admission

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultBoundsAreTheContractNumbers(t *testing.T) {
	b := DefaultBounds()
	checks := map[string]int{
		"MaxDbHandles":         b.MaxDbHandles,
		"MaxAppLeases":         b.MaxAppLeases,
		"MaxConcurrentOpens":   b.MaxConcurrentOpens,
		"MaxSQLConnections":    b.MaxSQLConnections,
		"ReservedConnections":  b.ReservedConnections,
		"MaxForegroundQueries": b.MaxForegroundQueries,
		"ForegroundPerApp":     b.ForegroundPerApp,
		"BackgroundWorkers":    b.BackgroundWorkers,
		"MaxQueuedJobs":        b.MaxQueuedJobs,
		"QueuedJobsPerApp":     b.QueuedJobsPerApp,
	}
	want := map[string]int{
		"MaxDbHandles":         64,
		"MaxAppLeases":         16,
		"MaxConcurrentOpens":   8,
		"MaxSQLConnections":    256,
		"ReservedConnections":  16,
		"MaxForegroundQueries": 128,
		"ForegroundPerApp":     16,
		"BackgroundWorkers":    2,
		"MaxQueuedJobs":        1024,
		"QueuedJobsPerApp":     128,
	}
	for k, got := range checks {
		if want[k] != got {
			t.Fatalf("%s = %d, want %d", k, got, want[k])
		}
	}
	if b.QueueWait != 5*time.Second {
		t.Fatalf("QueueWait = %v, want 5s", b.QueueWait)
	}
	if b.NativeCacheBytes != 256<<20 || b.DiskReserveBytes != 1<<30 || b.WalSoftWarningBytes != 256<<20 {
		t.Fatalf("byte budgets wrong: %+v", b)
	}
}

func TestConnectionCeilingAndReservedPoolSeparation(t *testing.T) {
	m := NewManager(Bounds{MaxSQLConnections: 4, ReservedConnections: 2, QueueWait: 50 * time.Millisecond})

	// 2 foreground pool tokens (4 - 2 reserved).
	r1, err := m.AcquireConnection(context.Background(), false)
	if err != nil {
		t.Fatalf("first pool token: %v", err)
	}
	r2, err := m.AcquireConnection(context.Background(), false)
	if err != nil {
		t.Fatalf("second pool token: %v", err)
	}
	if _, err := m.AcquireConnection(context.Background(), false); err != ErrResourceExhausted {
		t.Fatalf("third pool token: want ErrResourceExhausted, got %v", err)
	}

	// Reserved pool is separate: 2 reserved tokens remain available.
	rr1, err := m.AcquireConnection(context.Background(), true)
	if err != nil {
		t.Fatalf("reserved token: %v", err)
	}
	rr2, err := m.AcquireConnection(context.Background(), true)
	if err != nil {
		t.Fatalf("reserved token 2: %v", err)
	}
	if _, err := m.AcquireConnection(context.Background(), true); err != ErrResourceExhausted {
		t.Fatalf("reserved token 3: want ErrResourceExhausted, got %v", err)
	}

	r1()
	r2()
	rr1()
	rr2()

	// Tokens are released; pool is available again.
	if r, err := m.AcquireConnection(context.Background(), false); err != nil {
		t.Fatalf("token after release: %v", err)
	} else {
		r()
	}
}

func TestQueueWaitReturnsRetryableAfterDeadline(t *testing.T) {
	m := NewManager(Bounds{MaxConcurrentOpens: 1, QueueWait: 120 * time.Millisecond})
	r, err := m.AcquireOpen(context.Background())
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	defer r()
	start := time.Now()
	_, err = m.AcquireOpen(context.Background())
	if err != ErrResourceExhausted {
		t.Fatalf("want ErrResourceExhausted, got %v", err)
	}
	if d := time.Since(start); d < 100*time.Millisecond {
		t.Fatalf("returned before the queue wait elapsed: %v", d)
	}
}

func TestAcquireHonoursContextCancellation(t *testing.T) {
	m := NewManager(Bounds{MaxDbHandles: 1, QueueWait: 10 * time.Second})
	r, _ := m.AcquireHandle(context.Background())
	defer r()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	if _, err := m.AcquireHandle(ctx); err == nil {
		t.Fatalf("want cancellation error, got nil")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("cancellation waited too long: %v", d)
	}
}

func TestConcurrentAcquireNeverExceedsCeiling(t *testing.T) {
	const ceiling = 4
	m := NewManager(Bounds{MaxForegroundQueries: ceiling, QueueWait: 100 * time.Millisecond})
	var live, peak, over int64
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := m.AcquireForegroundQuery(context.Background())
			if err != nil {
				atomic.AddInt64(&over, 1)
				return
			}
			n := atomic.AddInt64(&live, 1)
			for {
				p := atomic.LoadInt64(&peak)
				if n <= p || atomic.CompareAndSwapInt64(&peak, p, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt64(&live, -1)
			r()
		}()
	}
	wg.Wait()
	if peak > ceiling {
		t.Fatalf("peak concurrent %d exceeds ceiling %d", peak, ceiling)
	}
	t.Logf("peak=%d over=%d (contention admitted and queued as designed)", peak, over)
}

func TestReleaseIsIdempotent(t *testing.T) {
	m := NewManager(Bounds{MaxConcurrentOpens: 1, QueueWait: 50 * time.Millisecond})
	r, err := m.AcquireOpen(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	r()
	r()
	r()
	if _, err := m.AcquireOpen(context.Background()); err != nil {
		t.Fatalf("acquire after triple release: %v", err)
	}
}
