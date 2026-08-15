package feed

import (
	"testing"
	"time"
)

func TestFutureEventsFiltersAndSorts(t *testing.T) {
	now := time.Date(2026, time.August, 15, 10, 0, 0, 0, moscowTime)
	items := []Event{
		validTestEvent("later", "Later", "2026-10-01"),
		validTestEvent("past", "Past", "2026-08-14"),
		validTestEvent("soon", "Soon", "2026-09-01"),
	}

	got := futureEvents(items, now)
	if len(got) != 2 {
		t.Fatalf("expected 2 upcoming events, got %d", len(got))
	}
	if got[0].ID != "soon" || got[1].ID != "later" {
		t.Fatalf("events are not sorted by date: %#v", got)
	}
}

func TestMergeEventsUpdatesCuratedEventAndRejectsInvalidDiscovery(t *testing.T) {
	now := time.Date(2026, time.August, 15, 10, 0, 0, 0, moscowTime)
	curated := validTestEvent("practical-ml-conf-2026", "Practical ML Conf 2026", "2026-09-19")
	curated.Featured = true
	discovered := validTestEvent("practical-ml-conf-2026", "Practical ML Conf 2026", "2026-09-19")
	discovered.Summary = "Updated from the official organizer website."
	invalid := validTestEvent("invented", "Invented event", "2026-09-20")
	invalid.URL = "javascript:alert(1)"

	got := mergeEvents([]Event{curated}, []Event{discovered, invalid}, now)
	if len(got) != 1 {
		t.Fatalf("expected only the verified event, got %d", len(got))
	}
	if got[0].Summary != discovered.Summary {
		t.Fatalf("expected discovery to update the curated event, got %q", got[0].Summary)
	}
	if !got[0].Featured {
		t.Fatal("expected curated featured flag to be preserved")
	}
}

func TestCuratedEventsContainRussianAndGlobalFlagships(t *testing.T) {
	items := futureEvents(curatedEvents(), time.Date(2026, time.August, 15, 10, 0, 0, 0, moscowTime))
	wanted := map[string]bool{
		"practical-ml-conf-2026": false,
		"highload-2026":          false,
		"neurips-2026":           false,
		"aaai-2027":              false,
	}
	for _, item := range items {
		if _, ok := wanted[item.ID]; ok {
			wanted[item.ID] = true
		}
	}
	for id, found := range wanted {
		if !found {
			t.Errorf("curated catalog is missing %s", id)
		}
	}
}

func validTestEvent(id, title, eventDate string) Event {
	return Event{
		ID: id, Title: title, Summary: "A confirmed technology event with an official source.",
		StartDate: eventDate, EndDate: eventDate, City: "Moscow", Country: "Russia",
		Format: "in_person", Kind: "conference", Region: "ru", Topics: []string{"AI"},
		URL: "https://example.com/" + id, SourceURL: "https://example.com/" + id,
	}
}
