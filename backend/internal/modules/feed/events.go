package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

const eventsCacheVersion = "v1"

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
	Discover(context.Context) ([]Event, error)
}

type EventDiscoveryClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c EventDiscoveryClient) Discover(ctx context.Context) ([]Event, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(c.BaseURL, "/")+"/v1/events/discover",
		bytes.NewReader([]byte("{}")),
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
		client = &http.Client{Timeout: 3 * time.Minute}
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
	cacheKey := fmt.Sprintf("feed:events:%s:%s", eventsCacheVersion, now.Format("2006-01-02"))
	if f.Redis != nil {
		if raw, err := f.Redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var cached EventCatalog
			if json.Unmarshal(raw, &cached) == nil && len(cached.Items) > 0 {
				return cached, nil
			}
		}
	}
	return f.RefreshEvents(ctx)
}

func (f Service) RefreshEvents(ctx context.Context) (EventCatalog, error) {
	now := time.Now().In(moscowTime)
	items := curatedEvents()
	automatic := false
	if f.EventProvider != nil {
		discovered, err := f.EventProvider.Discover(ctx)
		if err == nil {
			items = mergeEvents(items, discovered, now)
			automatic = true
		}
	}
	items = futureEvents(items, now)
	catalog := EventCatalog{
		Items:        items,
		UpdatedAt:    now,
		NextUpdateAt: nextEventRefresh(now),
		Automatic:    automatic,
	}
	if f.Redis != nil {
		if raw, err := json.Marshal(catalog); err == nil {
			cacheKey := fmt.Sprintf("feed:events:%s:%s", eventsCacheVersion, now.Format("2006-01-02"))
			ttl := time.Until(catalog.NextUpdateAt.Add(time.Hour))
			if ttl < time.Hour {
				ttl = time.Hour
			}
			_ = f.Redis.Set(ctx, cacheKey, raw, ttl).Err()
		}
	}
	return catalog, nil
}

// StartEventRefreshLoop refreshes the shared Redis catalog shortly after midnight
// in Moscow. UpcomingEvents also refreshes lazily, so restarts and missed timers are safe.
func (f Service) StartEventRefreshLoop(ctx context.Context) {
	for {
		refreshCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		_, _ = f.RefreshEvents(refreshCtx)
		cancel()

		delay := time.Until(nextEventRefresh(time.Now().In(moscowTime)))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func nextEventRefresh(now time.Time) time.Time {
	local := now.In(moscowTime)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 5, 0, 0, moscowTime)
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
		if i, ok := index[key]; ok {
			item.Featured = item.Featured || merged[i].Featured
			merged[i] = item
			continue
		}
		index[key] = len(merged)
		merged = append(merged, item)
	}
	return merged
}

func eventKey(item Event) string {
	title := strings.ToLower(strings.Join(strings.Fields(item.Title), " "))
	return title + "|" + item.StartDate[:min(4, len(item.StartDate))]
}

var eventIDPattern = regexp.MustCompile(`[^a-z0-9-]+`)

func normalizeEvent(item Event) Event {
	item.ID = strings.ToLower(strings.TrimSpace(item.ID))
	item.ID = eventIDPattern.ReplaceAllString(item.ID, "-")
	item.ID = strings.Trim(item.ID, "-")
	item.Title = strings.TrimSpace(item.Title)
	item.Summary = strings.TrimSpace(item.Summary)
	item.StartDate = strings.TrimSpace(item.StartDate)
	item.EndDate = strings.TrimSpace(item.EndDate)
	if item.EndDate == "" {
		item.EndDate = item.StartDate
	}
	item.City = strings.TrimSpace(item.City)
	item.Country = strings.TrimSpace(item.Country)
	item.Format = strings.ToLower(strings.TrimSpace(item.Format))
	item.Kind = strings.ToLower(strings.TrimSpace(item.Kind))
	item.Region = strings.ToLower(strings.TrimSpace(item.Region))
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
	if item.Topics == nil {
		item.Topics = []string{}
	}
	return item
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
