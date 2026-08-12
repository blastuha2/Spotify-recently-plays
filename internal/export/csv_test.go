package export

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"spotify-history/internal/spotify"
)

func TestCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.csv")
	events := []spotify.Event{{
		PlayedAt:       time.Date(2026, 8, 11, 13, 20, 10, 0, time.UTC),
		Artist:         "Исполнитель 1; Исполнитель 2",
		Track:          "Название, трека",
		Album:          "Альбом",
		TrackDuration:  183000,
		SpotifyTrackID: "abc123",
		SpotifyURI:     "spotify:track:abc123",
	}}
	if err := CSV(path, events); err != nil {
		t.Fatalf("CSV: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("CSV has no UTF-8 BOM")
	}
	rows, err := csv.NewReader(bytes.NewReader(data[3:])).ReadAll()
	if err != nil {
		t.Fatalf("parse resulting CSV: %v", err)
	}
	if len(rows) != 2 || len(rows[0]) != 9 {
		t.Fatalf("unexpected CSV dimensions: %#v", rows)
	}
	if rows[1][1] != events[0].Artist || rows[1][2] != events[0].Track {
		t.Fatalf("CSV fields did not round trip: %#v", rows[1])
	}
}
