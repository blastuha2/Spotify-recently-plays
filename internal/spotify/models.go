package spotify

import (
	"strings"
	"time"
)

// Event is one listening event in the normalized export format.
type Event struct {
	PlayedAt       time.Time `json:"played_at"`
	Artist         string    `json:"artist"`
	Track          string    `json:"track"`
	Album          string    `json:"album"`
	TrackDuration  int       `json:"track_duration_ms"`
	SpotifyTrackID string    `json:"spotify_track_id"`
	SpotifyURI     string    `json:"spotify_track_uri"`
	ContextType    string    `json:"context_type"`
	ContextURI     string    `json:"context_uri"`

	// Artists preserves individual names in JSON; Artist is the CSV-friendly joined form.
	Artists []string `json:"artists,omitempty"`
}

type page struct {
	Items   []playItem `json:"items"`
	Next    *string    `json:"next"`
	Cursors cursors    `json:"cursors"`
	Limit   int        `json:"limit"`
	Total   int        `json:"total"`
}

type cursors struct {
	After  string `json:"after"`
	Before string `json:"before"`
}

type playItem struct {
	Track    track        `json:"track"`
	PlayedAt time.Time    `json:"played_at"`
	Context  *playContext `json:"context"`
}

type track struct {
	Album      album    `json:"album"`
	Artists    []artist `json:"artists"`
	DurationMS int      `json:"duration_ms"`
	ID         string   `json:"id"`
	URI        string   `json:"uri"`
	Name       string   `json:"name"`
}

type album struct {
	Name string `json:"name"`
}

type artist struct {
	Name string `json:"name"`
}

type playContext struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

func (i playItem) event() Event {
	artists := make([]string, 0, len(i.Track.Artists))
	for _, a := range i.Track.Artists {
		if a.Name != "" {
			artists = append(artists, a.Name)
		}
	}

	e := Event{
		PlayedAt:       i.PlayedAt,
		Artist:         strings.Join(artists, "; "),
		Artists:        artists,
		Track:          i.Track.Name,
		Album:          i.Track.Album.Name,
		TrackDuration:  i.Track.DurationMS,
		SpotifyTrackID: i.Track.ID,
		SpotifyURI:     i.Track.URI,
	}
	if i.Context != nil {
		e.ContextType = i.Context.Type
		e.ContextURI = i.Context.URI
	}
	return e
}

func eventKey(e Event) string {
	trackKey := e.SpotifyTrackID
	if trackKey == "" {
		trackKey = e.SpotifyURI
	}
	return e.PlayedAt.UTC().Format(time.RFC3339Nano) + "\x00" + trackKey
}
