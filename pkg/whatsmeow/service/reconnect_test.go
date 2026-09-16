package whatsmeow_service

import (
	"testing"
	"time"
)

// An instance that never paired must give up: those failures come from missing
// credentials, which retrying cannot fix.
func TestReconnectTrackerStopsForUnpairedInstance(t *testing.T) {
	tracker := newReconnectTracker()

	for i := 1; i <= maxAutoReconnectAttempts; i++ {
		attempt, _, ok := tracker.begin("i1", false)
		if !ok {
			t.Fatalf("attempt %d refused, want allowed", i)
		}
		if attempt != i {
			t.Errorf("attempt number = %d, want %d", attempt, i)
		}
		tracker.done("i1")
	}

	if _, _, ok := tracker.begin("i1", false); ok {
		t.Error("budget was not enforced for an unpaired instance")
	}
}

// A paired instance keeps trying: a drop there is a network problem and the
// operator expects it to come back on its own.
func TestReconnectTrackerNeverStopsForPairedInstance(t *testing.T) {
	tracker := newReconnectTracker()

	for i := 1; i <= maxAutoReconnectAttempts*5; i++ {
		attempt, _, ok := tracker.begin("i1", true)
		if !ok {
			t.Fatalf("paired instance refused at attempt %d", i)
		}
		if attempt != i {
			t.Errorf("attempt number = %d, want %d", attempt, i)
		}
		tracker.done("i1")
	}
}

// Two Disconnected events arriving together must not stack up retries.
func TestReconnectTrackerRefusesConcurrentAttempts(t *testing.T) {
	tracker := newReconnectTracker()

	if _, _, ok := tracker.begin("i1", true); !ok {
		t.Fatal("first attempt refused")
	}
	if _, _, ok := tracker.begin("i1", true); ok {
		t.Error("a second attempt started while one was in flight")
	}

	tracker.done("i1")
	if _, _, ok := tracker.begin("i1", true); !ok {
		t.Error("attempt refused after the in-flight one finished")
	}
}

// next lets the retry loop advance without going back through begin, which
// would deadlock against the in-flight flag the loop itself holds.
func TestReconnectTrackerNextAdvancesWithoutReleasingInFlight(t *testing.T) {
	tracker := newReconnectTracker()

	attempt, _, ok := tracker.begin("i1", true)
	if !ok || attempt != 1 {
		t.Fatalf("begin returned (%d, %v)", attempt, ok)
	}

	if got := tracker.next("i1"); got != 2 {
		t.Errorf("next = %d, want 2", got)
	}
	if got := tracker.count("i1"); got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
	if _, _, ok := tracker.begin("i1", true); ok {
		t.Error("begin succeeded while the loop still held the in-flight slot")
	}
}

func TestResetClearsTheBudget(t *testing.T) {
	tracker := newReconnectTracker()

	for i := 0; i < maxAutoReconnectAttempts; i++ {
		tracker.begin("i1", false)
		tracker.done("i1")
	}
	if _, _, ok := tracker.begin("i1", false); ok {
		t.Fatal("expected the budget to be exhausted")
	}

	tracker.reset("i1")
	if _, _, ok := tracker.begin("i1", false); !ok {
		t.Error("reset did not refill the budget")
	}
}

// The delay must grow and then hold, so a long outage costs a few attempts an
// hour instead of hammering the server.
func TestReconnectDelayGrowsThenCaps(t *testing.T) {
	previous := time.Duration(0)
	for attempt := 1; attempt <= 6; attempt++ {
		delay := reconnectDelay(attempt)
		if delay < previous {
			t.Errorf("attempt %d: delay %s went backwards from %s", attempt, delay, previous)
		}
		if delay > reconnectBackoffCap {
			t.Errorf("attempt %d: delay %s exceeds the cap %s", attempt, delay, reconnectBackoffCap)
		}
		previous = delay
	}

	for _, attempt := range []int{6, 20, 500} {
		if got := reconnectDelay(attempt); got != reconnectBackoffCap {
			t.Errorf("attempt %d: delay = %s, want the cap %s", attempt, got, reconnectBackoffCap)
		}
	}

	if reconnectDelay(1) != 5*time.Second {
		t.Errorf("first retry should be quick, got %s", reconnectDelay(1))
	}
}

// A sleeping retry used to check "is the attempt counter still my number?".
// After an operator reconnect (which resets the counter) a fresh attempt lands
// back on 1 — so an old attempt 1 saw its own number, believed it was still
// current, and reconnected an instance the operator had just taken over. The
// generation makes "reset happened" observable independently of the count.
func TestResetSupersedesASleepingRetry(t *testing.T) {
	tracker := newReconnectTracker()

	_, gen, ok := tracker.begin("i1", true)
	if !ok {
		t.Fatal("first attempt refused")
	}

	if !tracker.current("i1", gen) {
		t.Error("a fresh attempt was reported as superseded")
	}

	// The operator reconnects: budget refilled, in-flight retry no longer wanted.
	tracker.reset("i1")
	tracker.done("i1")

	if tracker.current("i1", gen) {
		t.Error("a retry from before the reset was still reported as current")
	}

	// The attempt number is back where it started, which is exactly why
	// comparing counts could not detect this.
	attempt, newGen, ok := tracker.begin("i1", true)
	if !ok {
		t.Fatal("attempt after reset refused")
	}
	if attempt != 1 {
		t.Errorf("attempt after reset = %d, want 1", attempt)
	}
	if newGen == gen {
		t.Error("the generation did not change across a reset")
	}
	if !tracker.current("i1", newGen) {
		t.Error("the new attempt was reported as superseded")
	}
}
