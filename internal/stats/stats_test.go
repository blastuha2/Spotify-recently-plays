package stats

import (
	"testing"
	"time"

	"spotify-history/internal/spotify"
)

func TestCalculate(t *testing.T) {
	events := []spotify.Event{
		{PlayedAt: time.Date(2026, 7, 31, 23, 0, 0, 0, time.UTC), SpotifyTrackID: "1", Artists: []string{"A", "B"}},
		{PlayedAt: time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC), SpotifyTrackID: "1", Artists: []string{"A"}},
	}
	summary := Calculate(events)
	if summary.TotalPlays != 2 || summary.UniqueTracks != 1 || summary.UniqueArtists != 2 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if summary.PlaysPerMonth["2026-07"] != 1 || summary.PlaysPerMonth["2026-08"] != 1 {
		t.Fatalf("unexpected monthly stats: %#v", summary.PlaysPerMonth)
	}
}
