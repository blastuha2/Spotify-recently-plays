package stats

import (
	"sort"
	"strings"
	"time"

	"spotify-history/internal/spotify"
)

type Summary struct {
	TotalPlays    int
	UniqueTracks  int
	UniqueArtists int
	First         time.Time
	Last          time.Time
	PlaysPerMonth map[string]int
	PlaysPerDay   map[string]int
}

func Calculate(events []spotify.Event) Summary {
	summary := Summary{
		TotalPlays:    len(events),
		PlaysPerMonth: make(map[string]int),
		PlaysPerDay:   make(map[string]int),
	}
	tracks := make(map[string]struct{})
	artists := make(map[string]struct{})
	for index, event := range events {
		playedAt := event.PlayedAt.UTC()
		if index == 0 || playedAt.Before(summary.First) {
			summary.First = playedAt
		}
		if index == 0 || playedAt.After(summary.Last) {
			summary.Last = playedAt
		}
		trackKey := event.SpotifyTrackID
		if trackKey == "" {
			trackKey = event.SpotifyURI
		}
		if trackKey == "" {
			trackKey = event.Artist + "\x00" + event.Track
		}
		tracks[trackKey] = struct{}{}
		eventArtists := event.Artists
		if len(eventArtists) == 0 && event.Artist != "" {
			eventArtists = strings.Split(event.Artist, "; ")
		}
		for _, artist := range eventArtists {
			if artist != "" {
				artists[artist] = struct{}{}
			}
		}
		summary.PlaysPerMonth[playedAt.Format("2006-01")]++
		summary.PlaysPerDay[playedAt.Format("2006-01-02")]++
	}
	summary.UniqueTracks = len(tracks)
	summary.UniqueArtists = len(artists)
	return summary
}

func SortedKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
