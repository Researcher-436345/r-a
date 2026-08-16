package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	eventsCacheVersion  = "v2"
	legacyCacheVersion  = "v1"
	eventRefreshTimeout = 7 * time.Minute
	eventRefreshLockTTL = 8 * time.Minute
)

var moscowTime = time.FixedZone("Europe/Moscow", 3*60*60)

type Event struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	City            string   `json:"city"`
	Country         string   `json:"country"`
	Format          string   `json:"format"`
	Kind            string   `json:"kind"`
	Region          string   `json:"region"`
	Topics          []string `json:"topics"`
	URL             string   `json:"url"`
	RegistrationURL *string  `json:"registration_url,omitempty"`
	SourceURL       string   `json:"source_url"`
	Featured        bool     `json:"featured"`
}

type EventCatalog struct {
	Items        []Event   `json:"items"`
	UpdatedAt    time.Time `json:"updated_at"`
	NextUpdateAt time.Time `json:"next_update_at"`
	Automatic    bool      `json:"automatic"`
}

type EventProvider interface {
	Discover(context.Context, []Event) ([]Event, error)
}

type EventDiscoveryClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c EventDiscoveryClient) Discover(ctx context.Context, known []Event) ([]Event, error) {
	type knownEvent struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		StartDate string `json:"start_date"`
		URL       string `json:"url"`
	}
	discoveryRequest := struct {
		KnownEvents []knownEvent `json:"known_events"`
	}{KnownEvents: make([]knownEvent, 0, len(known))}
	for _, item := range known {
		discoveryRequest.KnownEvents = append(discoveryRequest.KnownEvents, knownEvent{
			ID: item.ID, Title: item.Title, StartDate: item.StartDate, URL: item.URL,
		})
	}
	body, err := json.Marshal(discoveryRequest)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/v1/events/discover",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("X-Internal-Token", c.Token)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 380 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("event discovery request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("event discovery returned %s", resp.Status)
	}
	var payload struct {
		Items []Event `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (f Service) UpcomingEvents(ctx context.Context) (EventCatalog, error) {
	now := time.Now().In(moscowTime)
	if cached, ok := f.cachedEventCatalog(ctx, now); ok {
		// Curated entries are authoritative. Reconcile cached discovery data on
		// every read so a model can add a missing series but never overwrite a
		// manually verified date, location or description.
		cached.Items = futureEvents(mergeEvents(curatedEvents(), cached.Items, now), now)
		cached.NextUpdateAt = nextEventRefresh(now)
		f.storeEventCatalog(ctx, cached)
		return cached, nil
	}

	// Event discovery is deliberately never called from an HTTP request. The
	// page gets the curated catalog immediately while the background loop owns
	// all potentially slow and billable provider calls.
	catalog := buildEventCatalog(curatedEvents(), now, false)
	f.storeEventCatalog(ctx, catalog)
	return catalog, nil
}

func (f Service) RefreshEvents(ctx context.Context) (EventCatalog, error) {
	now := time.Now().In(moscowTime)
	release, acquired := f.acquireEventRefreshLock(ctx, now)
	if !acquired {
		if cached, ok := f.cachedEventCatalog(ctx, now); ok {
			return cached, nil
		}
		return buildEventCatalog(curatedEvents(), now, false), nil
	}
	defer release()

	items := curatedEvents()
	automatic := false
	if cached, ok := f.cachedEventCatalog(ctx, now); ok {
		// Previously discovered events are part of the durable catalog. A later
		// provider failure or an incomplete result must never remove them.
		items = mergeEvents(items, cached.Items, now)
		automatic = cached.Automatic
	}
	if f.EventProvider != nil {
		discovered, err := f.EventProvider.Discover(ctx, items)
		if err == nil {
			items = mergeEvents(items, discovered, now)
			automatic = true
			log.Printf("event discovery completed: received=%d catalog=%d", len(discovered), len(items))
		} else {
			log.Printf("event discovery failed; keeping curated catalog: %v", err)
		}
	}
	catalog := buildEventCatalog(items, now, automatic)
	f.storeEventCatalog(ctx, catalog)
	return catalog, nil
}

func buildEventCatalog(items []Event, now time.Time, automatic bool) EventCatalog {
	return EventCatalog{
		Items:        futureEvents(items, now),
		UpdatedAt:    now,
		NextUpdateAt: nextEventRefresh(now),
		Automatic:    automatic,
	}
}

func eventCacheKey() string {
	return fmt.Sprintf("feed:events:%s:catalog", eventsCacheVersion)
}

func legacyEventCacheKey(now time.Time) string {
	return fmt.Sprintf("feed:events:%s:%s", legacyCacheVersion, now.In(moscowTime).Format("2006-01-02"))
}

func (f Service) cachedEventCatalog(ctx context.Context, now time.Time) (EventCatalog, bool) {
	if f.Redis == nil {
		return EventCatalog{}, false
	}
	if cached, ok := f.readEventCatalog(ctx, eventCacheKey()); ok {
		return cached, true
	}

	// Migrate the richer of today's and yesterday's old daily caches. This
	// keeps already paid-for discovery results when deploying the durable key.
	legacy := make([]EventCatalog, 0, 2)
	for _, day := range []time.Time{now, now.AddDate(0, 0, -1)} {
		if cached, ok := f.readEventCatalog(ctx, legacyEventCacheKey(day)); ok {
			legacy = append(legacy, cached)
		}
	}
	if len(legacy) == 0 {
		return EventCatalog{}, false
	}
	cached := preferredEventCatalog(legacy)
	f.storeEventCatalog(ctx, cached)
	log.Printf("migrated legacy event catalog to durable cache: items=%d", len(cached.Items))
	return cached, true
}

func (f Service) readEventCatalog(ctx context.Context, key string) (EventCatalog, bool) {
	raw, err := f.Redis.Get(ctx, key).Bytes()
	if err != nil {
		return EventCatalog{}, false
	}
	var cached EventCatalog
	if json.Unmarshal(raw, &cached) != nil || len(cached.Items) == 0 {
		return EventCatalog{}, false
	}
	return cached, true
}

func preferredEventCatalog(catalogs []EventCatalog) EventCatalog {
	best := catalogs[0]
	for _, candidate := range catalogs[1:] {
		if len(candidate.Items) > len(best.Items) ||
			(len(candidate.Items) == len(best.Items) && candidate.UpdatedAt.After(best.UpdatedAt)) {
			best = candidate
		}
	}
	return best
}

func (f Service) storeEventCatalog(ctx context.Context, catalog EventCatalog) {
	if f.Redis != nil {
		if raw, err := json.Marshal(catalog); err == nil {
			// The catalog is durable. Daily freshness is decided from UpdatedAt;
			// expiration must not erase discovered events while discovery is off.
			if err := f.Redis.Set(ctx, eventCacheKey(), raw, 0).Err(); err != nil {
				log.Printf("failed to cache event catalog: %v", err)
			}
		}
	}
}

func (f Service) acquireEventRefreshLock(ctx context.Context, now time.Time) (func(), bool) {
	if f.Redis == nil {
		return func() {}, true
	}
	key := fmt.Sprintf("feed:events:refresh-lock:%s:%s", eventsCacheVersion, now.In(moscowTime).Format("2006-01-02"))
	acquired, err := f.Redis.SetNX(ctx, key, "1", eventRefreshLockTTL).Result()
	if err != nil {
		log.Printf("failed to acquire event refresh lock; continuing without it: %v", err)
		return func() {}, true
	}
	if !acquired {
		log.Print("event discovery skipped: another refresh is already running")
		return func() {}, false
	}
	return func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = f.Redis.Del(cleanupCtx, key).Err()
	}, true
}

// StartEventRefreshLoop refreshes the shared Redis catalog shortly after midnight
// in Moscow. UpdatedAt suppresses duplicate searches while the durable catalog
// preserves discovered events across calendar days and restarts.
func (f Service) StartEventRefreshLoop(ctx context.Context) {
	now := time.Now().In(moscowTime)
	if cached, ok := f.cachedEventCatalog(ctx, now); !ok || !sameMoscowDay(cached.UpdatedAt, now) {
		f.refreshEventsWithTimeout(ctx)
	} else {
		log.Print("event discovery skipped on startup: today's catalog is already cached")
	}

	for {
		delay := time.Until(nextEventRefresh(time.Now().In(moscowTime)))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			f.refreshEventsWithTimeout(ctx)
		}
	}
}

func sameMoscowDay(a, b time.Time) bool {
	return a.In(moscowTime).Format("2006-01-02") == b.In(moscowTime).Format("2006-01-02")
}

func (f Service) refreshEventsWithTimeout(ctx context.Context) {
	refreshCtx, cancel := context.WithTimeout(ctx, eventRefreshTimeout)
	defer cancel()
	_, _ = f.RefreshEvents(refreshCtx)
}

func nextEventRefresh(now time.Time) time.Time {
	local := now.In(moscowTime)
	next := time.Date(local.Year(), local.Month(), local.Day(), 0, 5, 0, 0, moscowTime)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func futureEvents(items []Event, now time.Time) []Event {
	today := now.In(moscowTime).Format("2006-01-02")
	out := make([]Event, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item = normalizeEvent(item)
		if !validEvent(item, now) || item.EndDate < today || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartDate == out[j].StartDate {
			return out[i].Title < out[j].Title
		}
		return out[i].StartDate < out[j].StartDate
	})
	return out
}

func mergeEvents(curated, discovered []Event, now time.Time) []Event {
	merged := append([]Event{}, curated...)
	index := make(map[string]int, len(merged))
	for i, item := range merged {
		index[eventKey(item)] = i
	}
	for _, item := range discovered {
		item = normalizeEvent(item)
		if !validEvent(item, now) {
			continue
		}
		key := eventKey(item)
		if _, ok := index[key]; ok {
			// The curated catalog is the source of truth for known event series.
			// Discovery may add missing series, but cannot rewrite verified data.
			continue
		}
		index[key] = len(merged)
		merged = append(merged, item)
	}
	return merged
}

func eventKey(item Event) string {
	return eventSeriesName(item) + "|" + item.StartDate[:min(4, len(item.StartDate))]
}

var eventTitleSeparatorPattern = regexp.MustCompile(`[^\p{L}\p{N}]+`)
var eventYearTokenPattern = regexp.MustCompile(`^(?:19|20)\d{2}$`)

func eventSeriesName(item Event) string {
	title := strings.ToLower(eventTitleSeparatorPattern.ReplaceAllString(item.Title, " "))
	startYear := item.StartDate[:min(4, len(item.StartDate))]
	shortYear := ""
	if len(startYear) == 4 {
		shortYear = startYear[2:]
	}
	parts := make([]string, 0, 6)
	for _, part := range strings.Fields(title) {
		if eventYearTokenPattern.MatchString(part) || (len(parts) > 0 && shortYear != "" && part == shortYear) {
			break
		}
		parts = append(parts, part)
	}
	if len(parts) == 0 {
		return title
	}
	return strings.Join(parts, " ")
}

func isEventSubEvent(item Event) bool {
	title := " " + strings.ToLower(eventTitleSeparatorPattern.ReplaceAllString(item.Title, " ")) + " "
	for _, marker := range []string{
		" satellite ", " сателлит ", " workshop ", " воркшоп ",
		" tutorial ", " туториал ", " track ", " трек ",
	} {
		if strings.Contains(title, marker) {
			return true
		}
	}
	return false
}

var eventIDPattern = regexp.MustCompile(`[^a-z0-9-]+`)

func normalizeEvent(item Event) Event {
	item.ID = strings.ToLower(strings.TrimSpace(item.ID))
	item.ID = eventIDPattern.ReplaceAllString(item.ID, "-")
	item.ID = strings.Trim(item.ID, "-")
	item.Title = strings.TrimSpace(item.Title)
	item.Summary = strings.Join(strings.Fields(item.Summary), " ")
	if item.Summary != "" && !strings.ContainsAny(item.Summary[len(item.Summary)-1:], ".!?") {
		item.Summary += "."
	}
	item.StartDate = strings.TrimSpace(item.StartDate)
	item.EndDate = strings.TrimSpace(item.EndDate)
	if item.EndDate == "" {
		item.EndDate = item.StartDate
	}
	item.City = normalizeEventPlaces(item.City, eventCityNames)
	item.Country = normalizeEventPlaces(item.Country, eventCountryNames)
	item.Format = strings.ToLower(strings.TrimSpace(item.Format))
	item.Kind = strings.ToLower(strings.TrimSpace(item.Kind))
	item.Region = strings.ToLower(strings.TrimSpace(item.Region))
	if item.Country == "Россия" {
		item.Region = "ru"
	} else if item.Region == "ru" && item.Country == "" {
		item.Country = "Россия"
	}
	item.URL = strings.TrimSpace(item.URL)
	item.SourceURL = strings.TrimSpace(item.SourceURL)
	if item.RegistrationURL != nil {
		registrationURL := strings.TrimSpace(*item.RegistrationURL)
		if validHTTPURL(registrationURL) {
			item.RegistrationURL = &registrationURL
		} else {
			item.RegistrationURL = nil
		}
	}
	if item.SourceURL == "" {
		item.SourceURL = item.URL
	}
	item.Topics = normalizeEventTopics(item.Topics)
	return item
}

var eventCountryNames = map[string]string{
	"russia": "Россия", "russian federation": "Россия", "российская федерация": "Россия",
	"usa": "США", "us": "США", "united states": "США", "united states of america": "США",
	"australia": "Австралия", "france": "Франция", "canada": "Канада",
	"sweden": "Швеция", "japan": "Япония", "germany": "Германия",
	"united kingdom": "Великобритания", "uk": "Великобритания", "netherlands": "Нидерланды",
	"uae": "ОАЭ", "united arab emirates": "ОАЭ", "spain": "Испания",
	"italy": "Италия", "austria": "Австрия", "switzerland": "Швейцария",
	"singapore": "Сингапур", "south korea": "Южная Корея", "china": "Китай",
}

var eventCityNames = map[string]string{
	"moscow": "Москва", "saint petersburg": "Санкт-Петербург", "st. petersburg": "Санкт-Петербург",
	"sydney": "Сидней", "atlanta": "Атланта", "paris": "Париж", "orlando": "Орландо",
	"montreal": "Монреаль", "seattle": "Сиэтл", "kyoto": "Киото", "malmö": "Мальмё",
	"malmo": "Мальмё", "arlington": "Арлингтон", "london": "Лондон", "berlin": "Берлин",
	"amsterdam": "Амстердам", "barcelona": "Барселона", "vienna": "Вена",
}

func normalizeEventPlaces(raw string, names map[string]string) string {
	parts := strings.Split(strings.TrimSpace(raw), "·")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if normalized, ok := names[strings.ToLower(part)]; ok {
			part = normalized
		}
		parts[i] = part
	}
	return strings.Join(parts, " · ")
}

func normalizeEventTopics(topics []string) []string {
	aliases := map[string]string{
		"ai": "AI", "artificial intelligence": "AI", "machine learning": "ML", "ml": "ML",
		"deep learning": "Deep Learning", "large language models": "LLM", "llm": "LLM",
		"mlops": "MLOps", "devops": "DevOps", "backend": "Backend", "frontend": "Frontend",
		"cloud": "Cloud", "security": "Security", "data": "Data", "product": "Product",
		"computer vision": "Computer Vision", "nlp": "NLP", "qa": "QA",
		"genai": "GenAI", "highload": "HighLoad", "ot security": "OT Security",
		"neural information processing systems": "Neural Networks",
	}
	result := make([]string, 0, min(8, len(topics)))
	seen := map[string]bool{}
	for _, topic := range topics {
		topic = strings.Join(strings.Fields(topic), " ")
		if normalized, ok := aliases[strings.ToLower(topic)]; ok {
			topic = normalized
		} else {
			topic = titleEventTopic(topic)
		}
		key := strings.ToLower(topic)
		if topic == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, topic)
		if len(result) == 8 {
			break
		}
	}
	return result
}

func titleEventTopic(topic string) string {
	words := strings.Fields(topic)
	for i, word := range words {
		runes := []rune(strings.ToLower(word))
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func validEvent(item Event, now time.Time) bool {
	if item.ID == "" || item.Title == "" || item.Summary == "" {
		return false
	}
	start, startErr := time.Parse("2006-01-02", item.StartDate)
	end, endErr := time.Parse("2006-01-02", item.EndDate)
	if startErr != nil || endErr != nil || end.Before(start) || start.Year() > now.Year()+2 {
		return false
	}
	if item.Region != "ru" && item.Region != "global" {
		return false
	}
	if item.Kind != "conference" && item.Kind != "meetup" {
		return false
	}
	if item.Format != "in_person" && item.Format != "online" && item.Format != "hybrid" {
		return false
	}
	return validHTTPURL(item.URL) && validHTTPURL(item.SourceURL)
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func curatedEvents() []Event {
	return []Event{
		{
			ID: "deep-tech-night-2026", Title: "deep tech night", Summary: "Ночная IT-конференция с инженерными докладами и экскурсиями в офисы российских технологических компаний.",
			StartDate: "2026-09-05", EndDate: "2026-09-05", City: "Москва", Country: "Россия", Format: "in_person", Kind: "conference", Region: "ru",
			Topics: []string{"Engineering", "AI", "Product"}, URL: "https://deeptechnight.ru/", SourceURL: "https://deeptechnight.ru/", Featured: true,
		},
		{
			ID: "it-elements-2026", Title: "IT Elements 2026", Summary: "Крупная инженерная конференция о построении, защите и развитии IT-инфраструктуры.",
			StartDate: "2026-09-09", EndDate: "2026-09-10", City: "Москва", Country: "Россия", Format: "in_person", Kind: "conference", Region: "ru",
			Topics: []string{"Infrastructure", "Security", "DevOps"}, URL: "https://it-elements.ru/", SourceURL: "https://it-elements.ru/",
		},
		{
			ID: "practical-ml-conf-2026", Title: "Practical ML Conf 2026", Summary: "Хардовая конференция Яндекса о практическом ML: LLM, NLP, RecSys, CV, Speech и MLOps.",
			StartDate: "2026-09-19", EndDate: "2026-09-19", City: "Москва", Country: "Россия", Format: "hybrid", Kind: "conference", Region: "ru",
			Topics: []string{"ML", "LLM", "MLOps"}, URL: "https://pmlconf.yandex.ru/2026/index", SourceURL: "https://pmlconf.yandex.ru/2026/index", Featured: true,
		},
		{
			ID: "yandex-scale-2026", Title: "Yandex Scale 2026", Summary: "Главная конференция Yandex Cloud о технологиях облачной платформы, данных, инфраструктуре и AI.",
			StartDate: "2026-09-24", EndDate: "2026-09-24", City: "Москва", Country: "Россия", Format: "hybrid", Kind: "conference", Region: "ru",
			Topics: []string{"Cloud", "AI", "Infrastructure"}, URL: "https://scale.yandex.cloud/", SourceURL: "https://events.yandex.ru/", Featured: true,
		},
		{
			ID: "aies-2026", Title: "AIES 2026", Summary: "Международная конференция AAAI/ACM об этике, безопасности и общественном влиянии искусственного интеллекта.",
			StartDate: "2026-10-12", EndDate: "2026-10-14", City: "Мальмё", Country: "Швеция", Format: "in_person", Kind: "conference", Region: "global",
			Topics: []string{"Responsible AI", "AI Safety", "Ethics"}, URL: "https://www.aies-conference.com/2026/", SourceURL: "https://aaai.org/",
		},
		{
			ID: "itsec-2026", Title: "ITSEC 2026", Summary: "Форум по информационной безопасности, включая отдельную конференцию о защите локальных AI-моделей.",
			StartDate: "2026-10-13", EndDate: "2026-10-14", City: "Москва", Country: "Россия", Format: "hybrid", Kind: "conference", Region: "ru",
			Topics: []string{"Security", "Local AI", "Infrastructure"}, URL: "https://www.itsecexpo.ru/", SourceURL: "https://www.itsecexpo.ru/",
		},
		{
			ID: "ai-native-conf-2026", Title: "AI Native Conf 2026", Summary: "Практическая конференция о внедрении AI и построении продуктов и компаний вокруг искусственного интеллекта.",
			StartDate: "2026-10-20", EndDate: "2026-10-20", City: "Москва", Country: "Россия", Format: "in_person", Kind: "conference", Region: "ru",
			Topics: []string{"GenAI", "AI Product", "Business"}, URL: "https://ainativeconf.ru/", SourceURL: "https://ainativeconf.ru/", Featured: true,
		},
		{
			ID: "aaai-fss-2026", Title: "AAAI Fall Symposium Series 2026", Summary: "Серия исследовательских симпозиумов AAAI по новым направлениям искусственного интеллекта.",
			StartDate: "2026-11-05", EndDate: "2026-11-07", City: "Арлингтон", Country: "США", Format: "in_person", Kind: "conference", Region: "global",
			Topics: []string{"AI Research", "Agents", "Responsible AI"}, URL: "https://aaai.org/conference/fall-symposia/fss26/", SourceURL: "https://aaai.org/",
		},
		{
			ID: "highload-2026", Title: "HighLoad++ 2026", Summary: "Крупнейшая российская конференция разработчиков высоконагруженных систем.",
			StartDate: "2026-11-30", EndDate: "2026-12-01", City: "Москва", Country: "Россия", Format: "in_person", Kind: "conference", Region: "ru",
			Topics: []string{"HighLoad", "Backend", "Architecture"}, URL: "https://highload.ru/moscow/2026", SourceURL: "https://highload.ru/moscow/2026", Featured: true,
		},
		{
			ID: "neurips-2026", Title: "NeurIPS 2026", Summary: "Одна из главных мировых конференций по машинному обучению и нейронным информационным системам.",
			StartDate: "2026-12-06", EndDate: "2026-12-12", City: "Сидней · Париж · Атланта", Country: "Австралия · Франция · США", Format: "hybrid", Kind: "conference", Region: "global",
			Topics: []string{"ML", "Deep Learning", "AI Research"}, URL: "https://neurips.cc/Conferences/2026", SourceURL: "https://neurips.cc/Conferences/2026/Dates", Featured: true,
		},
		{
			ID: "wacv-2027", Title: "WACV 2027", Summary: "Зимняя конференция IEEE/CVF по компьютерному зрению и прикладным системам распознавания.",
			StartDate: "2027-01-04", EndDate: "2027-01-08", City: "Орландо", Country: "США", Format: "in_person", Kind: "conference", Region: "global",
			Topics: []string{"Computer Vision", "Recognition", "Applied AI"}, URL: "https://wacv.thecvf.com/", SourceURL: "https://www.thecvf.com/?p=137",
		},
		{
			ID: "aaai-2027", Title: "AAAI-27", Summary: "Ежегодная международная конференция AAAI по фундаментальным и прикладным направлениям искусственного интеллекта.",
			StartDate: "2027-02-16", EndDate: "2027-02-23", City: "Монреаль", Country: "Канада", Format: "in_person", Kind: "conference", Region: "global",
			Topics: []string{"AI Research", "AI Alignment", "Applied AI"}, URL: "https://aaai.org/conference/aaai/aaai-27/", SourceURL: "https://aaai.org/conference/aaai/aaai-27/", Featured: true,
		},
		{
			ID: "cvpr-2027", Title: "CVPR 2027", Summary: "Главная конференция IEEE/CVF по компьютерному зрению и распознаванию образов.",
			StartDate: "2027-06-20", EndDate: "2027-06-24", City: "Сиэтл", Country: "США", Format: "in_person", Kind: "conference", Region: "global",
			Topics: []string{"Computer Vision", "Multimodal AI", "Deep Learning"}, URL: "https://cvpr.thecvf.com/", SourceURL: "https://www.thecvf.com/?p=137", Featured: true,
		},
		{
			ID: "acl-2027", Title: "ACL 2027", Summary: "Ведущая мировая конференция по обработке естественного языка и вычислительной лингвистике.",
			StartDate: "2027-08-17", EndDate: "2027-08-22", City: "Киото", Country: "Япония", Format: "hybrid", Kind: "conference", Region: "global",
			Topics: []string{"NLP", "LLM", "Computational Linguistics"}, URL: "https://2027.aclweb.org/", SourceURL: "https://2027.aclweb.org/", Featured: true,
		},
	}
}
