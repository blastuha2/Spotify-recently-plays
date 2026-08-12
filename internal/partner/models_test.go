package partner

import (
	"testing"
	"time"
)

func TestItemTrack(t *testing.T) {
	item := item{
		AddedAt: calendarDate{Year: 2026, Month: 7, Day: 31},
		Entity: &entity{
			URI: "spotify:track:abc123",
			Data: entityData{
				EntityTypeTrait: entityTypeTrait{Type: trackEntityType},
				IdentityTrait: identityTrait{
					Name:                   "Track",
					ContentHierarchyParent: &hierarchyParent{},
					Contributors: contributorListing{Items: []contributor{
						{Name: "Artist 1"},
						{Name: "Artist 2"},
					}},
				},
				ConsumptionExperienceTrait: consumptionExperienceTrait{
					Duration: duration{Seconds: 183, NanoSeconds: 500_000_000},
				},
			},
		},
	}
	item.Entity.Data.IdentityTrait.ContentHierarchyParent.IdentityTrait.Name = "Album"

	track, ok, err := item.track(123)
	if err != nil {
		t.Fatalf("track: %v", err)
	}
	if !ok {
		t.Fatal("track entity was filtered out")
	}
	if track.PlayedDate != "2026-07-31" || track.Date.Location() != time.UTC {
		t.Fatalf("unexpected date: %#v", track)
	}
	if track.Artist != "Artist 1; Artist 2" || track.Album != "Album" {
		t.Fatalf("unexpected metadata: %#v", track)
	}
	if track.TrackDuration != 183500 || track.SpotifyTrackID != "abc123" || track.SourcePosition != 123 {
		t.Fatalf("unexpected normalized track: %#v", track)
	}
}

func TestItemTrackFiltersNonTrackAndNull(t *testing.T) {
	date := calendarDate{Year: 2026, Month: 7, Day: 31}
	if _, ok, err := (item{AddedAt: date}).track(0); err != nil || ok {
		t.Fatalf("null entity: ok=%v err=%v", ok, err)
	}
	album := item{AddedAt: date, Entity: &entity{URI: "spotify:album:123"}}
	if _, ok, err := album.track(0); err != nil || ok {
		t.Fatalf("album entity: ok=%v err=%v", ok, err)
	}
	if _, err := (calendarDate{Year: 2026, Month: 2, Day: 31}).time(); err == nil {
		t.Fatal("expected invalid calendar date")
	}
}
