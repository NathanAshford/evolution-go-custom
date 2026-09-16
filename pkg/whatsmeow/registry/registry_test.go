package registry

import (
	"sync"
	"testing"
	"time"
)

// Stop used to be a close() on a channel other goroutines still sent to, which
// panicked with "send on closed channel" and killed the whole process. It must
// be safe to call any number of times, from anywhere.
func TestStopIsIdempotent(t *testing.T) {
	clients := New()
	entry := clients.Begin("i1")

	entry.Stop()
	entry.Stop()

	if !clients.Stop("i1") {
		t.Error("Stop on a live entry reported no entry")
	}

	select {
	case <-entry.Done():
	default:
		t.Error("Done was not closed by Stop")
	}
}

// Begin must stop whatever was running for the instance. Two concurrent QR
// requests otherwise left two live clients fighting over one instance.
func TestBeginSupersedesThePreviousEntry(t *testing.T) {
	clients := New()
	first := clients.Begin("i1")
	second := clients.Begin("i1")

	if first == second {
		t.Fatal("Begin returned the same entry twice")
	}

	select {
	case <-first.Done():
	default:
		t.Error("the superseded entry was not told to stop")
	}

	select {
	case <-second.Done():
		t.Error("the new entry was stopped")
	default:
	}
}

func TestRemoveStopsAndForgets(t *testing.T) {
	clients := New()
	entry := clients.Begin("i1")

	if !clients.Remove("i1") {
		t.Fatal("Remove reported no entry")
	}

	select {
	case <-entry.Done():
	default:
		t.Error("Remove did not stop the entry")
	}

	if clients.Has("i1") {
		t.Error("the entry survived Remove")
	}
	if clients.Get("i1") != nil {
		t.Error("Get still returns a client after Remove")
	}
	if clients.Remove("i1") {
		t.Error("a second Remove reported an entry")
	}
}

// Attach must refuse to publish a client from a start that was superseded, and
// must not overwrite the client of the entry that now owns the instance.
func TestAttachRefusesASupersededEntry(t *testing.T) {
	clients := New()
	stale := clients.Begin("i1")
	current := clients.Begin("i1")

	if clients.Attach("i1", stale, nil) {
		t.Error("Attach accepted a superseded entry")
	}
	if !clients.Attach("i1", current, nil) {
		t.Error("Attach refused the entry that owns the instance")
	}

	current.Stop()
	if clients.Attach("i1", current, nil) {
		t.Error("Attach accepted a stopped entry")
	}
}

// Get on an instance that was never started must return nil rather than panic:
// callers check for nil, and the old bare map lookup on a missing key was what
// ForceReconnect dereferenced.
func TestGetUnknownInstanceReturnsNil(t *testing.T) {
	clients := New()

	if clients.Get("nope") != nil {
		t.Error("Get invented a client")
	}
	if _, ok := clients.Entry("nope"); ok {
		t.Error("Entry invented an entry")
	}
	if clients.Stop("nope") {
		t.Error("Stop reported an entry that does not exist")
	}
}

// The maps this replaced were written from HTTP handlers, event handlers and
// reconnect goroutines with no synchronisation at all, which Go turns into
// "fatal error: concurrent map writes" — an unrecoverable process death. Run
// with -race to check the replacement holds.
func TestConcurrentUseIsRaceFree(t *testing.T) {
	clients := New()

	const workers = 16
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			id := "i" + string(rune('a'+n%4))
			for i := 0; i < 200; i++ {
				switch i % 6 {
				case 0:
					clients.Begin(id)
				case 1:
					clients.Get(id)
				case 2:
					clients.Stop(id)
				case 3:
					clients.Remove(id)
				case 4:
					clients.Has(id)
				case 5:
					clients.IDs()
				}
			}
		}(w)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent registry use deadlocked")
	}
}

// A superseded goroutine must clean up after itself without touching the entry
// that replaced it — otherwise a restart tore down the client that had just
// taken over, leaving the instance looking started with nothing serving it.
func TestRemoveOwnedLeavesANewerEntryAlone(t *testing.T) {
	clients := New()
	stale := clients.Begin("i1")
	current := clients.Begin("i1")

	if clients.RemoveOwned("i1", stale) {
		t.Error("a superseded goroutine was allowed to tear the instance down")
	}
	if entry, ok := clients.Entry("i1"); !ok || entry != current {
		t.Error("the newer entry was removed by the superseded one")
	}
	if current.Stopped() {
		t.Error("the newer entry was stopped by the superseded one")
	}

	// The owner may clean up, and so may an owner whose slot Remove already
	// dropped — nothing newer holds the instance in either case.
	if !clients.RemoveOwned("i1", current) {
		t.Error("the owning goroutine was refused")
	}
	if clients.Has("i1") {
		t.Error("the entry survived RemoveOwned")
	}

	entry := clients.Begin("i2")
	clients.Remove("i2")
	if !clients.RemoveOwned("i2", entry) {
		t.Error("an already-removed instance refused its own owner, so its state would leak")
	}
}
