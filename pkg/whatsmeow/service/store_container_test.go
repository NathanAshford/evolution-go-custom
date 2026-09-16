package whatsmeow_service

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func sqliteAddress(t *testing.T) string {
	t.Helper()
	return "file:" + filepath.Join(t.TempDir(), "devices.db") +
		"?_pragma=foreign_keys(1)&_busy_timeout=5000&mode=rwc&_journal_mode=WAL"
}

// The outage this fixes came from opening a connection pool per connection
// attempt: every StartClient, QR poll and auto-reconnect leaked one, until
// Postgres answered "sorry, too many clients already" to everything. The cache
// must hand out ONE container, however many callers ask, and bound its pool.
func TestStoreContainerIsOpenedOnce(t *testing.T) {
	var cache storeContainerCache
	address := sqliteAddress(t)

	first, err := cache.get(context.Background(), "sqlite", address, nil)
	if err != nil {
		t.Fatalf("first get failed: %v", err)
	}
	if first == nil {
		t.Fatal("first get returned no container")
	}

	second, err := cache.get(context.Background(), "sqlite", address, nil)
	if err != nil {
		t.Fatalf("second get failed: %v", err)
	}
	if first != second {
		t.Error("a second caller was given a different container, so a second pool was opened")
	}

	stats := cache.db.Stats()
	if stats.MaxOpenConnections != storeMaxOpenConns {
		t.Errorf("MaxOpenConnections = %d, want %d — an unbounded pool is what exhausted the server",
			stats.MaxOpenConnections, storeMaxOpenConns)
	}

	if err := cache.close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
}

// Concurrent starts are the normal case on boot (CONNECT_ON_STARTUP brings every
// instance up at once), so they must not each open a pool.
func TestStoreContainerConcurrentGetsShareOnePool(t *testing.T) {
	var cache storeContainerCache
	address := sqliteAddress(t)

	const callers = 12
	var wg sync.WaitGroup
	got := make([]interface{}, callers)

	for n := 0; n < callers; n++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			container, err := cache.get(context.Background(), "sqlite", address, nil)
			if err != nil {
				t.Errorf("caller %d failed: %v", i, err)
				return
			}
			got[i] = container
		}(n)
	}
	wg.Wait()

	for i, container := range got {
		if container != got[0] {
			t.Fatalf("caller %d got a different container: a pool was opened per caller", i)
		}
	}

	if err := cache.close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
}

// A failure must be remembered rather than retried with a fresh pool — retrying
// the open per attempt is precisely what turned a struggling database into an
// unreachable one.
func TestStoreContainerCachesFailure(t *testing.T) {
	var cache storeContainerCache

	if _, err := cache.get(context.Background(), "no-such-driver", "irrelevant", nil); err == nil {
		t.Fatal("opening an unknown driver succeeded")
	}

	if _, err := cache.get(context.Background(), "sqlite", sqliteAddress(t), nil); err == nil {
		t.Error("a later caller reopened the store after a failure")
	}

	if err := cache.close(); err != nil {
		t.Errorf("close on a failed cache should be a no-op, got: %v", err)
	}
}
