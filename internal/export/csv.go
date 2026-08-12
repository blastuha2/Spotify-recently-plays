package export

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"spotify-history/internal/spotify"
)

var csvHeader = []string{
	"played_at",
	"artist",
	"track",
	"album",
	"track_duration_ms",
	"spotify_track_id",
	"spotify_track_uri",
	"context_type",
	"context_uri",
}

func CSV(path string, events []spotify.Event) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create CSV: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close CSV: %w", closeErr)
		}
	}()

	// A UTF-8 BOM makes non-ASCII text open correctly in older Excel versions.
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("write CSV BOM: %w", err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(csvHeader); err != nil {
		return fmt.Errorf("write CSV header: %w", err)
	}
	for _, event := range events {
		record := []string{
			event.PlayedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
			event.Artist,
			event.Track,
			event.Album,
			strconv.Itoa(event.TrackDuration),
			event.SpotifyTrackID,
			event.SpotifyURI,
			event.ContextType,
			event.ContextURI,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush CSV: %w", err)
	}
	return nil
}
