package spotify

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchHistoryPaginationCutoffAndDeduplication(t *testing.T) {
	var server *httptest.Server
	var calls atomic.Int32
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `{
				"items": [
					{"played_at":"2026-01-10T10:00:00Z","track":{"id":"one","uri":"spotify:track:one","name":"One","album":{"name":"A"},"artists":[{"name":"Artist"}]}},
					{"played_at":"2026-01-08T10:00:00Z","track":{"id":"old","uri":"spotify:track:old","name":"Old","album":{"name":"A"},"artists":[{"name":"Artist"}]}}
				],
				"next": null
			}`)
			return
		}
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("first request limit = %q, want 50", r.URL.Query().Get("limit"))
		}
		fmt.Fprintf(w, `{
			"items": [{"played_at":"2026-01-10T10:00:00Z","track":{"id":"one","uri":"spotify:track:one","name":"One","album":{"name":"A"},"artists":[{"name":"Artist"}]}}],
			"next": %q
		}`, server.URL+"?page=2")
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	from := time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)
	result, err := client.FetchHistory(context.Background(), from, nil)
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if calls.Load() != 2 || result.APIRequests != 2 {
		t.Fatalf("requests: server=%d result=%d, want 2", calls.Load(), result.APIRequests)
	}
	if len(result.Events) != 1 {
		t.Fatalf("got %d deduplicated events, want 1", len(result.Events))
	}
	if !result.ReachedCutoff || result.StopReason != "requested start date reached" {
		t.Fatalf("unexpected cutoff result: %#v", result)
	}
}

func TestFetchHistoryUsesNextAndHandlesEmptyItems(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "empty" {
			fmt.Fprint(w, `{"items":[],"next":null}`)
			return
		}
		fmt.Fprintf(w, `{
			"items":[{"played_at":"2026-01-10T10:00:00Z","track":{"id":"one"}}],
			"next":%q
		}`, server.URL+"?page=empty")
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	result, err := client.FetchHistory(context.Background(), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if result.APIRequests != 2 || result.StopReason != "Spotify returned empty items" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestFetchHistoryRetriesRateLimit(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, `{"items":[],"next":null}`)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.BaseURL = server.URL
	result, err := client.FetchHistory(context.Background(), time.Time{}, nil)
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}
	if calls.Load() != 2 || result.APIRequests != 1 {
		t.Fatalf("calls=%d successful requests=%d", calls.Load(), result.APIRequests)
	}
}

func TestFetchHistoryUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient("bad-token")
	client.BaseURL = server.URL
	_, err := client.FetchHistory(context.Background(), time.Time{}, nil)
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestNextPageURL(t *testing.T) {
	base, err := url.Parse("https://api.spotify.test/v1/recently-played?limit=50")
	if err != nil {
		t.Fatal(err)
	}
	next, err := nextPageURL(base, "/v1/recently-played?before=123")
	if err != nil {
		t.Fatalf("nextPageURL: %v", err)
	}
	parsed, err := url.Parse(next)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("before") != "123" || parsed.Query().Get("limit") != "50" {
		t.Fatalf("next query was not preserved: %s", next)
	}
	if _, err := nextPageURL(base, "https://attacker.test/steal"); err == nil {
		t.Fatal("expected foreign host to be rejected")
	}
}
