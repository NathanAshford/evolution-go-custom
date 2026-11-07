package whatsmeow_service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type webVersionTestTransport func(*http.Request) (*http.Response, error)

func (f webVersionTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func isolateWebVersionFetch(t *testing.T, transport webVersionTestTransport) {
	t.Helper()
	originalClient := http.DefaultClient
	oldCache, oldTime := cachedWebVersion, cachedWebVersionAt
	cachedWebVersion = nil
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() { http.DefaultClient = originalClient; cachedWebVersion = oldCache; cachedWebVersionAt = oldTime })
}

func TestWebVersionFetchHasStartupDeadline(t *testing.T) {
	started := time.Now()
	isolateWebVersionFetch(t, func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok || deadline.After(started.Add(11*time.Second)) {
			t.Error("metadata request has no bounded startup deadline")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"client_revision":1052556802}`))}, nil
	})
	if _, err := fetchWhatsAppWebVersion(); err != nil {
		t.Fatal(err)
	}
}

func TestWebVersionCacheKeepsHourlyRefresh(t *testing.T) {
	calls := 0
	isolateWebVersionFetch(t, func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"client_revision":%d}`, 1052556800+calls)))}, nil
	})
	first, err := fetchWhatsAppWebVersion()
	if err != nil {
		t.Fatal(err)
	}
	cached, err := fetchWhatsAppWebVersion()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || cached.Patch != first.Patch {
		t.Fatal("unexpired metadata cache was not reused")
	}
	cachedWebVersionAt = time.Now().Add(-webVersionCacheTTL - time.Second)
	refreshed, err := fetchWhatsAppWebVersion()
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || refreshed.Patch == first.Patch {
		t.Fatal("expired cache did not refresh the version")
	}
}

type failedWebVersionBody struct{}

func (failedWebVersionBody) Read([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestWebVersionFetchReportsTransportAndBodyFailure(t *testing.T) {
	for _, mode := range []string{"transport", "body"} {
		t.Run(mode, func(t *testing.T) {
			isolateWebVersionFetch(t, func(request *http.Request) (*http.Response, error) {
				if mode == "transport" {
					return nil, fmt.Errorf("synthetic metadata transport failure")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(failedWebVersionBody{})}, nil
			})
			if _, err := fetchWhatsAppWebVersion(); err == nil {
				t.Fatal("metadata failure was lost")
			}
			if cachedWebVersion != nil {
				t.Fatal("failed metadata was cached as a usable version")
			}
		})
	}
}
