package spotify

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParsePageAndNullContext(t *testing.T) {
	const payload = `{
		"items": [{
			"played_at": "2026-08-11T13:20:10Z",
			"track": {
				"name": "Track, with comma",
				"id": "abc123",
				"uri": "spotify:track:abc123",
				"duration_ms": 183000,
				"album": {"name": "Album"},
				"artists": [{"name": "Artist 1"}, {"name": "Artist 2"}]
			},
			"context": null
		}],
		"next": "https://example.test/next",
		"cursors": {"before": "123"},
		"limit": 50
	}`

	var parsed page
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("decode page: %v", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(parsed.Items))
	}
	event := parsed.Items[0].event()
	if event.PlayedAt.Format(time.RFC3339) != "2026-08-11T13:20:10Z" {
		t.Fatalf("unexpected played_at: %s", event.PlayedAt)
	}
	if event.Artist != "Artist 1; Artist 2" {
		t.Fatalf("unexpected artists: %q", event.Artist)
	}
	if event.ContextType != "" || event.ContextURI != "" {
		t.Fatalf("null context was not empty: %#v", event)
	}
	if parsed.Next == nil || *parsed.Next != "https://example.test/next" {
		t.Fatalf("next link was not parsed: %#v", parsed.Next)
	}
}
