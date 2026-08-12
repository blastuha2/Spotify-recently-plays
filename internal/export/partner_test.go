package export

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"spotify-history/internal/partner"
)

func TestPartnerCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partner.csv")
	tracks := []partner.Track{{
		PlayedDate:     "2026-07-31",
		Artist:         "Исполнитель 1; Исполнитель 2",
		Track:          "Название, трека",
		Album:          "Альбом",
		TrackDuration:  183000,
		SpotifyTrackID: "abc123",
		SpotifyURI:     "spotify:track:abc123",
		SourcePosition: 1000,
	}}
	if err := PartnerCSV(path, tracks); err != nil {
		t.Fatalf("PartnerCSV: %v", err)
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
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 2 || len(rows[0]) != 8 || rows[1][0] != "2026-07-31" || rows[1][7] != "1000" {
		t.Fatalf("unexpected CSV: %#v", rows)
	}
}
