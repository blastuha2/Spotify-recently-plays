package stats

import (
	"strings"
	"time"

	"spotify-history/internal/partner"
)

func CalculatePartner(tracks []partner.Track) Summary {
	summary := Summary{
		TotalPlays:    len(tracks),
		PlaysPerMonth: make(map[string]int),
		PlaysPerDay:   make(map[string]int),
	}
	uniqueTracks := make(map[string]struct{})
	uniqueArtists := make(map[string]struct{})
	for index, track := range tracks {
		date := track.Date
		if date.IsZero() {
			date, _ = time.Parse("2006-01-02", track.PlayedDate)
		}
		if index == 0 || date.Before(summary.First) {
			summary.First = date
		}
		if index == 0 || date.After(summary.Last) {
			summary.Last = date
		}
		key := track.SpotifyTrackID
		if key == "" {
			key = track.SpotifyURI
		}
		if key == "" {
			key = track.Artist + "\x00" + track.Track
		}
		uniqueTracks[key] = struct{}{}
		artists := track.Artists
		if len(artists) == 0 && track.Artist != "" {
			artists = strings.Split(track.Artist, "; ")
		}
		for _, artist := range artists {
			if artist != "" {
				uniqueArtists[artist] = struct{}{}
			}
		}
		if !date.IsZero() {
			summary.PlaysPerMonth[date.Format("2006-01")]++
			summary.PlaysPerDay[date.Format("2006-01-02")]++
		}
	}
	summary.UniqueTracks = len(uniqueTracks)
	summary.UniqueArtists = len(uniqueArtists)
	return summary
}
