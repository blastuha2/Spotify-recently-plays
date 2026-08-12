package export

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"spotify-history/internal/partner"
)

var partnerCSVHeader = []string{
	"played_date",
	"artist",
	"track",
	"album",
	"track_duration_ms",
	"spotify_track_id",
	"spotify_track_uri",
	"source_position",
}

func PartnerCSV(path string, tracks []partner.Track) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create partner CSV: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close partner CSV: %w", closeErr)
		}
	}()
	if _, err := file.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return fmt.Errorf("write partner CSV BOM: %w", err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(partnerCSVHeader); err != nil {
		return fmt.Errorf("write partner CSV header: %w", err)
	}
	for _, track := range tracks {
		record := []string{
			track.PlayedDate,
			track.Artist,
			track.Track,
			track.Album,
			strconv.FormatInt(track.TrackDuration, 10),
			track.SpotifyTrackID,
			track.SpotifyURI,
			strconv.Itoa(track.SourcePosition),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("write partner CSV row: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush partner CSV: %w", err)
	}
	return nil
}

func PartnerJSON(path string, tracks []partner.Track) (err error) {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create partner JSON: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close partner JSON: %w", closeErr)
		}
	}()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(tracks); err != nil {
		return fmt.Errorf("write partner JSON: %w", err)
	}
	return nil
}
