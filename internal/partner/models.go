package partner

import (
	"fmt"
	"strings"
	"time"
)

const trackEntityType = "ENTITY_TYPE_TRACK"

// Track is one track occurrence returned by Spotify Web Player's recents list.
// The internal API exposes only a calendar date, not an exact played_at time.
type Track struct {
	PlayedDate     string   `json:"played_date"`
	Artist         string   `json:"artist"`
	Artists        []string `json:"artists,omitempty"`
	Track          string   `json:"track"`
	Album          string   `json:"album"`
	SourceType     string   `json:"source_type"`
	SourceName     string   `json:"source_name"`
	SourceURI      string   `json:"source_uri"`
	ActivityType   string   `json:"activity_type"`
	TrackDuration  int64    `json:"track_duration_ms"`
	SpotifyTrackID string   `json:"spotify_track_id"`
	SpotifyURI     string   `json:"spotify_track_uri"`
	SourcePosition int      `json:"source_position"`

	Date time.Time `json:"-"`
}

type queryResponse struct {
	Data struct {
		Lists []list `json:"lists"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type list struct {
	Items page `json:"items"`
}

type page struct {
	Items      []item     `json:"items"`
	PagingInfo pagingInfo `json:"pagingInfo"`
	TotalCount int        `json:"totalCount"`
}

type pagingInfo struct {
	Limit      int `json:"limit"`
	NextOffset int `json:"nextOffset"`
	Offset     int `json:"offset"`
}

type item struct {
	AddedAt              calendarDate      `json:"addedAt"`
	Entity               *entity           `json:"entity"`
	FormatListAttributes []formatAttribute `json:"formatListAttributes"`
}

type formatAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type sourceContext struct {
	Type         string
	Name         string
	URI          string
	ActivityType string
}

type calendarDate struct {
	Day   int `json:"day"`
	Month int `json:"month"`
	Year  int `json:"year"`
}

type entity struct {
	URI  string     `json:"_uri"`
	Data entityData `json:"data"`
}

type entityData struct {
	URI                        string                     `json:"uri"`
	EntityTypeTrait            entityTypeTrait            `json:"entityTypeTrait"`
	IdentityTrait              identityTrait              `json:"identityTrait"`
	ConsumptionExperienceTrait consumptionExperienceTrait `json:"consumptionExperienceTrait"`
}

type entityTypeTrait struct {
	Type string `json:"type"`
}

type identityTrait struct {
	Name                   string             `json:"name"`
	ContentHierarchyParent *hierarchyParent   `json:"contentHierarchyParent"`
	Contributors           contributorListing `json:"contributors"`
}

type hierarchyParent struct {
	IdentityTrait struct {
		Name string `json:"name"`
	} `json:"identityTrait"`
}

type contributorListing struct {
	Items []contributor `json:"items"`
}

type contributor struct {
	Name string `json:"name"`
	URI  string `json:"uri"`
}

type consumptionExperienceTrait struct {
	Duration duration `json:"duration"`
}

type duration struct {
	Seconds     int64 `json:"seconds"`
	NanoSeconds int64 `json:"nanoSeconds"`
}

func (d calendarDate) time() (time.Time, error) {
	if d.Year == 0 || d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > 31 {
		return time.Time{}, fmt.Errorf("invalid recents date %04d-%02d-%02d", d.Year, d.Month, d.Day)
	}
	value := time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
	if value.Year() != d.Year || int(value.Month()) != d.Month || value.Day() != d.Day {
		return time.Time{}, fmt.Errorf("invalid recents date %04d-%02d-%02d", d.Year, d.Month, d.Day)
	}
	return value, nil
}

func (i item) track(position int, source sourceContext) (Track, bool, error) {
	date, err := i.AddedAt.time()
	if err != nil {
		return Track{}, false, err
	}
	if i.Entity == nil {
		return Track{}, false, nil
	}
	uri := i.Entity.Data.URI
	if uri == "" {
		uri = i.Entity.URI
	}
	entityType := i.Entity.Data.EntityTypeTrait.Type
	if entityType != trackEntityType && !strings.HasPrefix(uri, "spotify:track:") {
		return Track{}, false, nil
	}

	artists := make([]string, 0, len(i.Entity.Data.IdentityTrait.Contributors.Items))
	for _, artist := range i.Entity.Data.IdentityTrait.Contributors.Items {
		if artist.Name != "" {
			artists = append(artists, artist.Name)
		}
	}
	album := ""
	if parent := i.Entity.Data.IdentityTrait.ContentHierarchyParent; parent != nil {
		album = parent.IdentityTrait.Name
	}
	duration := i.Entity.Data.ConsumptionExperienceTrait.Duration
	durationMS := duration.Seconds*1000 + duration.NanoSeconds/int64(time.Millisecond)
	activityType := i.activityType()
	if activityType == "" {
		activityType = source.ActivityType
	}

	return Track{
		PlayedDate:     date.Format("2006-01-02"),
		Date:           date,
		Artist:         strings.Join(artists, "; "),
		Artists:        artists,
		Track:          i.Entity.Data.IdentityTrait.Name,
		Album:          album,
		SourceType:     source.Type,
		SourceName:     source.Name,
		SourceURI:      source.URI,
		ActivityType:   activityType,
		TrackDuration:  durationMS,
		SpotifyTrackID: strings.TrimPrefix(uri, "spotify:track:"),
		SpotifyURI:     uri,
		SourcePosition: position,
	}, true, nil
}

func (i item) attribute(key string) string {
	for _, attribute := range i.FormatListAttributes {
		if attribute.Key == key {
			return attribute.Value
		}
	}
	return ""
}

func (i item) childrenGroupID() string {
	return i.attribute("children_group_id")
}

func (i item) groupID() string {
	for _, attribute := range i.FormatListAttributes {
		if strings.HasPrefix(attribute.Key, "group_id_") {
			groupID := strings.TrimPrefix(attribute.Key, "group_id_")
			if groupID != "" && groupID != "0" {
				return groupID
			}
		}
	}
	return ""
}

func (i item) activityType() string {
	for _, attribute := range i.FormatListAttributes {
		if strings.HasPrefix(attribute.Key, "recent_type_") {
			return strings.TrimPrefix(attribute.Key, "recent_type_")
		}
	}
	return ""
}

func (i item) sourceContext() sourceContext {
	context := sourceContext{ActivityType: i.activityType()}
	if i.Entity == nil {
		return context
	}
	context.Type = strings.ToLower(strings.TrimPrefix(i.Entity.Data.EntityTypeTrait.Type, "ENTITY_TYPE_"))
	context.Name = i.Entity.Data.IdentityTrait.Name
	context.URI = i.Entity.Data.URI
	if context.URI == "" {
		context.URI = i.Entity.URI
	}
	return context
}
