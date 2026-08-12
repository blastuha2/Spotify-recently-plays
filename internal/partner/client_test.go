package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchRecentsPaginationFilteringCutoffAndDuplicates(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer access" || r.Header.Get("Client-Token") != "client" {
			t.Errorf("missing authentication headers")
		}
		if r.Header.Get("App-Platform") != "WebPlayer" || r.Header.Get("Spotify-App-Version") != "1.2.test" {
			t.Errorf("missing Web Player headers")
		}
		if r.Header.Get("User-Agent") == "" || r.Header.Get("Sec-Fetch-Site") != "same-site" {
			t.Errorf("missing browser compatibility headers")
		}
		var request queryRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Variables.Limit != 100 || request.OperationName != "recents" {
			t.Errorf("unexpected request: %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Variables.Offset {
		case 0:
			fmt.Fprint(w, responseJSON(0, 3, 5, []string{
				trackJSON("2026-01-10", "same", "Track"),
				albumJSON("2026-01-10"),
				trackJSON("2026-01-09", "same", "Track"),
			}))
		case 3:
			fmt.Fprint(w, responseJSON(3, 5, 5, []string{
				trackJSON("2026-01-09", "same", "Track"),
				trackJSON("2026-01-07", "old", "Old"),
			}))
		default:
			t.Errorf("unexpected offset %d", request.Variables.Offset)
		}
	}))
	defer server.Close()

	client := NewClient("access", "client")
	client.Endpoint = server.URL
	client.AppVersion = "1.2.test"
	from := time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)
	result, err := client.FetchRecents(context.Background(), from, nil)
	if err != nil {
		t.Fatalf("FetchRecents: %v", err)
	}
	if calls.Load() != 2 || result.APIRequests != 2 {
		t.Fatalf("calls=%d requests=%d, want 2", calls.Load(), result.APIRequests)
	}
	if len(result.Tracks) != 3 {
		t.Fatalf("got %d track occurrences, want 3", len(result.Tracks))
	}
	if result.Tracks[1].SpotifyTrackID != "same" || result.Tracks[2].SpotifyTrackID != "same" {
		t.Fatal("duplicate track occurrences were not preserved")
	}
	if result.SkippedItems != 1 {
		t.Fatalf("skipped=%d, want 1", result.SkippedItems)
	}
	if !result.ReachedCutoff || result.StopReason != "requested start date reached" {
		t.Fatalf("unexpected stop: %#v", result)
	}
}

func TestFetchRecentsRateLimitAndEnd(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		fmt.Fprint(w, responseJSON(0, 1, 1, []string{trackJSON("2026-01-10", "one", "One")}))
	}))
	defer server.Close()

	client := NewClient("access", "client")
	client.Endpoint = server.URL
	result, err := client.FetchRecents(context.Background(), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("FetchRecents: %v", err)
	}
	if calls.Load() != 2 || result.APIRequests != 1 || result.StopReason != "all available recents fetched" {
		t.Fatalf("unexpected result: calls=%d result=%#v", calls.Load(), result)
	}
}

func TestFetchRecentsAddsPlaylistContextAndFiltersSavedActivity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		items := []string{
			playlistJSON("2026-01-10", "68", "played"),
			trackWithGroupJSON("2026-01-10", "played-track", "Played", "68", "played"),
			playlistJSON("2026-01-10", "69", "saved"),
			trackWithGroupJSON("2026-01-10", "saved-track", "Saved", "69", "saved"),
		}
		fmt.Fprint(w, responseJSON(0, 4, 4, items))
	}))
	defer server.Close()

	client := NewClient("access", "client")
	client.Endpoint = server.URL
	result, err := client.FetchRecents(context.Background(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("FetchRecents: %v", err)
	}
	if len(result.Tracks) != 1 {
		t.Fatalf("got %d tracks, want only the played track", len(result.Tracks))
	}
	track := result.Tracks[0]
	if track.SourceType != "playlist" || track.SourceName != "машина" || track.SourceURI != "spotify:playlist:car" || track.ActivityType != "played" {
		t.Fatalf("playlist context was not inherited: %#v", track)
	}
	if result.SkippedItems != 3 {
		t.Fatalf("skipped=%d, want 3", result.SkippedItems)
	}
}

func TestFetchRecentsUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	client := NewClient("access", "client")
	client.Endpoint = server.URL
	if _, err := client.FetchRecents(context.Background(), time.Time{}, nil); err == nil {
		t.Fatal("expected unauthorized error")
	}
}

func TestFetchRecentsForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	client := NewClient("access", "client")
	client.Endpoint = server.URL
	_, err := client.FetchRecents(context.Background(), time.Time{}, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("expected explicit 403 error, got %v", err)
	}
}

func responseJSON(offset, nextOffset, total int, items []string) string {
	return fmt.Sprintf(`{"data":{"lists":[{"items":{"items":[%s],"pagingInfo":{"offset":%d,"limit":100,"nextOffset":%d},"totalCount":%d}}]}}`,
		joinJSON(items), offset, nextOffset, total)
}

func joinJSON(items []string) string {
	result := ""
	for index, item := range items {
		if index > 0 {
			result += ","
		}
		result += item
	}
	return result
}

func trackJSON(date, id, name string) string {
	parsed, _ := time.Parse("2006-01-02", date)
	return fmt.Sprintf(`{"addedAt":{"year":%d,"month":%d,"day":%d},"entity":{"_uri":"spotify:track:%s","data":{"entityTypeTrait":{"type":"ENTITY_TYPE_TRACK"},"identityTrait":{"name":%q,"contributors":{"items":[{"name":"Artist"}]}},"consumptionExperienceTrait":{"duration":{"seconds":180,"nanoSeconds":0}}}}}`,
		parsed.Year(), parsed.Month(), parsed.Day(), id, name)
}

func albumJSON(date string) string {
	parsed, _ := time.Parse("2006-01-02", date)
	return fmt.Sprintf(`{"addedAt":{"year":%d,"month":%d,"day":%d},"entity":{"_uri":"spotify:album:album","data":{"entityTypeTrait":{"type":"ENTITY_TYPE_ALBUM"}}}}`,
		parsed.Year(), parsed.Month(), parsed.Day())
}

func playlistJSON(date, groupID, activity string) string {
	parsed, _ := time.Parse("2006-01-02", date)
	return fmt.Sprintf(`{"addedAt":{"year":%d,"month":%d,"day":%d},"entity":{"_uri":"spotify:playlist:car","data":{"entityTypeTrait":{"type":"ENTITY_TYPE_PLAYLIST"},"identityTrait":{"name":"машина"}}},"formatListAttributes":[{"key":"children_group_id","value":%q},{"key":"recent_type_%s","value":""}]}`,
		parsed.Year(), parsed.Month(), parsed.Day(), groupID, activity)
}

func trackWithGroupJSON(date, id, name, groupID, activity string) string {
	base := trackJSON(date, id, name)
	return strings.TrimSuffix(base, "}") + fmt.Sprintf(`,"formatListAttributes":[{"key":"group_id_%s","value":""},{"key":"recent_type_%s","value":""}]}`, groupID, activity)
}
