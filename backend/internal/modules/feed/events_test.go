package feed

import (
	"context"
	"testing"
	"time"
)

type countingEventProvider struct {
	calls int
	items []Event
}

func (p *countingEventProvider) Discover(_ context.Context, known []Event) ([]Event, error) {
	p.calls++
	if len(known) == 0 {
		panic("expected curated events to be passed to discovery")
	}
	return p.items, nil
}

func TestUpcomingEventsNeverCallsDiscovery(t *testing.T) {
	provider := &countingEventProvider{}
	service := Service{EventProvider: provider}

	catalog, err := service.UpcomingEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 0 {
		t.Fatalf("expected HTTP catalog load not to call discovery, got %d calls", provider.calls)
	}
	if catalog.Automatic {
		t.Fatal("expected immediate curated catalog to be marked as non-automatic")
	}
}

func TestRefreshEventsCallsDiscoveryOnce(t *testing.T) {
	eventDate := time.Now().In(moscowTime).AddDate(0, 1, 0).Format("2006-01-02")
	provider := &countingEventProvider{items: []Event{
		validTestEvent("new-event", "New Event", eventDate),
	}}
	service := Service{EventProvider: provider}

	catalog, err := service.RefreshEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatalf("expected exactly one discovery call, got %d", provider.calls)
	}
	if !catalog.Automatic {
		t.Fatal("expected successful discovery catalog to be marked as automatic")
	}
}

func TestNextEventRefreshUsesTodayBeforeMidnightWindow(t *testing.T) {
	now := time.Date(2026, time.August, 16, 0, 1, 0, 0, moscowTime)
	want := time.Date(2026, time.August, 16, 0, 5, 0, 0, moscowTime)
	if got := nextEventRefresh(now); !got.Equal(want) {
		t.Fatalf("expected next refresh at %s, got %s", want, got)
	}
}

func TestPreferredEventCatalogKeepsRicherDiscoveryAcrossMidnight(t *testing.T) {
	yesterday := EventCatalog{
		Items:     []Event{{ID: "curated"}, {ID: "discovered"}},
		UpdatedAt: time.Date(2026, time.August, 16, 23, 45, 0, 0, moscowTime),
		Automatic: true,
	}
	today := EventCatalog{
		Items:     []Event{{ID: "curated"}},
		UpdatedAt: time.Date(2026, time.August, 17, 0, 1, 0, 0, moscowTime),
	}

	got := preferredEventCatalog([]EventCatalog{today, yesterday})
	if len(got.Items) != 2 || !got.Automatic {
		t.Fatalf("expected richer discovered catalog to survive midnight, got %#v", got)
	}
}

func TestSameMoscowDay(t *testing.T) {
	beforeMidnight := time.Date(2026, time.August, 16, 20, 59, 0, 0, time.UTC)
	afterMidnight := beforeMidnight.Add(2 * time.Minute)
	if sameMoscowDay(beforeMidnight, afterMidnight) {
		t.Fatal("expected timestamps on different Moscow dates")
	}
}

func TestNormalizeEventUnifiesCopyLocationRegionAndTopics(t *testing.T) {
	item := validTestEvent("normalized", "Normalized", "2026-09-20")
	item.Summary = "  Единое   описание российского технологического события  "
	item.City = "Moscow"
	item.Country = "Russia"
	item.Region = "global"
	item.Topics = []string{"machine learning", "AI", "ai", "devops"}

	got := normalizeEvent(item)
	if got.Summary != "Единое описание российского технологического события." {
		t.Fatalf("unexpected normalized summary: %q", got.Summary)
	}
	if got.City != "Москва" || got.Country != "Россия" || got.Region != "ru" {
		t.Fatalf("unexpected normalized location: %#v", got)
	}
	if len(got.Topics) != 3 || got.Topics[0] != "ML" || got.Topics[2] != "DevOps" {
		t.Fatalf("unexpected normalized topics: %#v", got.Topics)
	}
}

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

func TestMergeEventsPreservesCuratedEventAndRejectsInvalidDiscovery(t *testing.T) {
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
	if got[0].Summary != curated.Summary {
		t.Fatalf("expected curated data to remain authoritative, got %q", got[0].Summary)
	}
	if !got[0].Featured {
		t.Fatal("expected curated featured flag to be preserved")
	}
}

func TestMergeEventsCollapsesConferenceSatellitesIntoMainSeries(t *testing.T) {
	now := time.Date(2026, time.August, 16, 10, 0, 0, 0, moscowTime)
	main := validTestEvent("neurips-2026", "NeurIPS 2026", "2026-12-06")
	main.EndDate = "2026-12-12"
	updated := validTestEvent(
		"neurips-2026-updated",
		"NeurIPS 2026 – 40th Annual Conference",
		"2026-12-06",
	)
	updated.EndDate = "2026-12-12"
	updated.Summary = "Updated official description for the main conference."
	atlanta := validTestEvent(
		"neurips-2026-atlanta",
		"NeurIPS 2026 – Atlanta Satellite",
		"2026-12-09",
	)
	atlanta.EndDate = "2026-12-13"

	got := mergeEvents([]Event{main}, []Event{updated, atlanta}, now)
	if len(got) != 1 {
		t.Fatalf("expected one NeurIPS series entry, got %#v", got)
	}
	if got[0].ID != main.ID {
		t.Fatalf("expected the curated main event to win, got %s", got[0].ID)
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
