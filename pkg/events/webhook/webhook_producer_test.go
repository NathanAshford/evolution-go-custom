package webhook_producer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/EvolutionAPI/evolution-go/pkg/config"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
)

// newTestProducer builds a producer with waits short enough to test a real
// backoff without sleeping through it.
func newTestProducer(t *testing.T, globalURL string) *webhookProducer {
	t.Helper()

	logs := logger_wrapper.NewLoggerManager(&config.Config{LogDirectory: t.TempDir()})
	p := NewWebhookProducer(globalURL, logs).(*webhookProducer)
	p.maxAttempts = 3
	p.baseDelay = time.Millisecond
	p.maxDelay = 2 * time.Millisecond
	return p
}

// waitFor polls until cond holds or the deadline passes, so tests do not depend
// on a fixed sleep to observe an asynchronous delivery.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestDeliversPayloadToInstanceWebhook(t *testing.T) {
	var hits int64
	var body atomic.Value

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ := io.ReadAll(r.Body)
		body.Store(string(received))
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	if err := p.Produce("inst.message", []byte(`{"event":"Message"}`), server.URL, "i1"); err != nil {
		t.Fatalf("Produce: %v", err)
	}

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) == 1 }, "webhook was never delivered")
	if got := body.Load(); got != `{"event":"Message"}` {
		t.Errorf("body = %v, want the original payload", got)
	}
}

// A queue name without a dot used to be dropped before any HTTP call, which
// silently discarded every "sendstatus" event.
func TestDeliversEvenWhenQueueNameHasNoDot(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	_ = p.Produce("sendstatus", []byte(`{"event":"SendStatus"}`), server.URL, "i1")

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) == 1 }, "bare queue name was dropped")
}

// 4xx means the endpoint understood and refused; retrying only hammers it.
func TestDoesNotRetryClientErrors(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	_ = p.Produce("inst.message", []byte(`{}`), server.URL, "i1")

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) >= 1 }, "endpoint was never called")
	time.Sleep(50 * time.Millisecond) // long enough for retries to have happened

	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("called %d times, want exactly 1 (4xx must not be retried)", got)
	}
}

func TestRetriesServerErrorsThenSucceeds(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	_ = p.Produce("inst.message", []byte(`{}`), server.URL, "i1")

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) == 3 }, "5xx was not retried to success")
}

// 429 is the one 4xx that does resolve on its own.
func TestRetriesTooManyRequests(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&hits, 1) < 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	_ = p.Produce("inst.message", []byte(`{}`), server.URL, "i1")

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) == 2 }, "429 was not retried")
}

func TestStopsAfterMaxAttempts(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	_ = p.Produce("inst.message", []byte(`{}`), server.URL, "i1")

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) == 3 }, "did not reach the attempt limit")
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt64(&hits); got != 3 {
		t.Errorf("called %d times, want exactly maxAttempts (3)", got)
	}
}

// The global webhook and the instance webhook are both delivered, but the same
// URL configured in both places must not receive the event twice.
func TestGlobalAndInstanceWebhooksAreDeduplicated(t *testing.T) {
	var hits int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProducer(t, server.URL)
	_ = p.Produce("inst.message", []byte(`{}`), server.URL, "i1")

	waitFor(t, func() bool { return atomic.LoadInt64(&hits) >= 1 }, "webhook was never delivered")
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Errorf("delivered %d times, want 1 (same URL must not be sent twice)", got)
	}
}

func TestGlobalAndInstanceWebhooksBothReceiveWhenDistinct(t *testing.T) {
	var globalHits, instanceHits int64

	globalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&globalHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer globalServer.Close()

	instanceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&instanceHits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer instanceServer.Close()

	p := newTestProducer(t, globalServer.URL)
	_ = p.Produce("inst.message", []byte(`{}`), instanceServer.URL, "i1")

	waitFor(t, func() bool {
		return atomic.LoadInt64(&globalHits) == 1 && atomic.LoadInt64(&instanceHits) == 1
	}, "both webhooks should have received the event")
}

// A hung endpoint must not pin the delivery goroutine forever. Without a client
// timeout this test would run until the test binary's own deadline.
func TestSlowEndpointTimesOutInsteadOfHanging(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	p := newTestProducer(t, "")
	p.maxAttempts = 1
	p.client.Timeout = 50 * time.Millisecond

	done := make(chan struct{})
	go func() {
		p.sendWebhookWithRetry(server.URL, []byte(`{}`), "i1")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery hung past the client timeout")
	}
}

// An unreachable host is a network failure, which is retryable.
func TestUnreachableHostIsRetried(t *testing.T) {
	p := newTestProducer(t, "")
	p.maxAttempts = 2

	done := make(chan struct{})
	go func() {
		// Port 0 is never listening, so every attempt fails fast.
		p.sendWebhookWithRetry("http://127.0.0.1:0/hook", []byte(`{}`), "i1")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("retry loop did not finish")
	}
}

func TestInFlightBudgetIsReleased(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := newTestProducer(t, "")
	for i := 0; i < maxInFlight*2; i++ {
		_ = p.Produce("inst.message", []byte(`{}`), server.URL, "i1")
	}

	waitFor(t, func() bool { return len(p.inFlight) == 0 }, "in-flight slots were not released")
}
