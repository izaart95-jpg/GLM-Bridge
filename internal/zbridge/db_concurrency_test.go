// Regression tests for the agent-mode startup crash (issue #40:
// "fatal error: concurrent map writes" in acquireDB, db.go).
//
// Agent mode starts the background captcha cache, whose Run loop spawns
// parallel generate() goroutines; each goes getNextToken -> withTokenDB ->
// acquireDB. acquireDB historically wrote s.inflight[h.db]++ under a READ
// lock — two concurrent readers both writing the map is an instant
// "fatal error: concurrent map writes", which kills the whole process (it
// cannot be recovered or deferred). withHandleForCount (the /health
// token-count path) had the identical RLock'd map write.
//
// These tests reproduce the exact read-write pattern with enough
// parallelism to trip it: before the fix the process crashes; after it,
// every acquire must succeed, every release must leave the in-flight
// registry clean, and concurrent queries must all complete.

package zbridge

import (
	"sync"
	"testing"
)

// TestAcquireDBConcurrentReaders hammers acquireDB from many goroutines at
// once — the same access pattern the parallel captcha generators produce
// at agent-mode startup.
func TestAcquireDBConcurrentReaders(t *testing.T) {
	attachTestDB(t, 32)

	const workers = 24
	const rounds = 200

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				h, release, err := globalDBState.acquireDB()
				if err != nil {
					t.Errorf("acquireDB: %v", err)
					return
				}
				if h.db == nil {
					t.Error("acquireDB returned nil handle")
					return
				}
				release()
			}
		}()
	}
	wg.Wait()

	// After every worker releases, the in-flight registry must be empty
	// for the live handle — the swap drain path relies on that.
	h, err := activeHandle()
	if err != nil {
		t.Fatal(err)
	}
	globalDBState.mu.Lock()
	if n, ok := globalDBState.inflight[h.db]; ok && n != 0 {
		globalDBState.mu.Unlock()
		t.Fatalf("inflight count for live handle = %d, want absent/0", n)
	}
	globalDBState.mu.Unlock()
}

// TestWithTokenDBConcurrentQueries exercises the full reader path
// (withTokenDB -> fn) concurrently — the exact shape of parallel captcha
// generation — and asserts the queries themselves succeed.
func TestWithTokenDBConcurrentQueries(t *testing.T) {
	attachTestDB(t, 64)

	const workers = 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < 50; r++ {
				var token string
				err := withTokenDB(func(h dbHandle) error {
					return h.db.QueryRow(
						"SELECT token FROM tokens ORDER BY id LIMIT 1;",
					).Scan(&token)
				})
				if err != nil {
					t.Errorf("withTokenDB: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestTokenCountConcurrentHealth hits the /health count path
// (withHandleForCount had the identical RLock'd map write) concurrently
// with token reads, mirroring health probes racing captcha generation.
func TestTokenCountConcurrentHealth(t *testing.T) {
	attachTestDB(t, 10)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if n := globalDBState.tokenCount(); n < 0 {
				t.Errorf("tokenCount = %d, want >= 0", n)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			var token string
			err := withTokenDB(func(h dbHandle) error {
				return h.db.QueryRow(
					"SELECT token FROM tokens ORDER BY id LIMIT 1;",
				).Scan(&token)
			})
			if err != nil {
				t.Errorf("withTokenDB: %v", err)
				return
			}
		}
	}()
	wg.Wait()
}
