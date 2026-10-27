package analytics

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/observability"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var eventWrites = promauto.NewCounterVec(prometheus.CounterOpts{Name: "ra_analytics_event_writes_total", Help: "Event persistence outcomes; no user labels."}, []string{"producer", "outcome"})

type Event struct {
	ClientAt       *time.Time `json:"client_at,omitempty"`
	ActionID       uuid.UUID  `json:"action_id"`
	SessionID      *uuid.UUID `json:"session_id,omitempty"`
	ProjectID      *uuid.UUID `json:"project_id,omitempty"`
	PaperID        *uuid.UUID `json:"paper_id,omitempty"`
	ResultID       *uuid.UUID `json:"result_id,omitempty"`
	SourceActionID *uuid.UUID `json:"source_action_id,omitempty"`
	Type           string     `json:"event_type"`
	Feature        string     `json:"feature,omitempty"`
	Outcome        string     `json:"-"`
	CacheHit       bool       `json:"cache_hit,omitempty"`
	Locale         string     `json:"locale,omitempty"`
	Device         string     `json:"device,omitempty"`
	Source         string     `json:"source,omitempty"`
	Feedback       *int16     `json:"feedback,omitempty"`
	At             time.Time  `json:"-"`
}

func nullableUUID(value string) *uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return nil
	}
	return &id
}

// Bind records logical actions independently of HTTP retries. It never changes a business response.
func Bind(pool *pgxpool.Pool) {
	observability.SetObserver(func(op observability.Operation, stage string) {
		user, err := uuid.Parse(op.UserID)
		if err != nil || user == uuid.Nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
		defer cancel()
		at := op.StartedAt
		kind := "action_started"
		if stage == "finished" {
			at = op.FinishedAt
			kind = "action_finished"
		}
		e := Event{ActionID: uuid.MustParse(op.ID), SessionID: nullableUUID(op.SessionID), PaperID: nullableUUID(op.PaperID), ResultID: nullableUUID(op.ResultID), Type: kind, Feature: op.Feature, Outcome: op.Outcome, CacheHit: op.CacheHit, Locale: op.Locale, Device: op.Device, At: at}
		if e.PaperID != nil {
			e.ProjectID, _ = projectForPaper(ctx, pool, user, *e.PaperID)
		}
		if err := insert(ctx, pool, user, e, "server"); err != nil {
			log.Print("analytics: action metadata could not be saved")
		}
	})
}

func enum(value string, allowed ...string) string {
	for _, v := range allowed {
		if value == v {
			return value
		}
	}
	return allowed[len(allowed)-1]
}

func insert(ctx context.Context, pool *pgxpool.Pool, user uuid.UUID, e Event, producer string) error {
	if e.ActionID == uuid.Nil {
		return errors.New("action id is required")
	}
	if e.At.IsZero() {
		e.At = time.Now()
	}
	e.Locale = enum(e.Locale, "ru", "en", "unknown")
	e.Device = enum(e.Device, "mobile", "desktop", "unknown")
	e.Source = enum(e.Source, "feed", "search", "library", "reader", "other")
	_, err := pool.Exec(ctx, `INSERT INTO analytics_events
		(user_id,action_id,session_id,project_id,paper_id,result_id,source_action_id,event_type,producer,feature,outcome,cache_hit,locale,device,source,feedback,occurred_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (user_id,action_id,event_type) DO UPDATE SET
		 feature=CASE WHEN EXCLUDED.producer='server' THEN EXCLUDED.feature ELSE analytics_events.feature END,
		 outcome=CASE WHEN analytics_events.outcome='success' THEN 'success' ELSE EXCLUDED.outcome END,
		 cache_hit=analytics_events.cache_hit OR EXCLUDED.cache_hit,
		 occurred_at=CASE WHEN EXCLUDED.event_type='action_finished' AND analytics_events.outcome<>'success' THEN EXCLUDED.occurred_at ELSE analytics_events.occurred_at END,
		 result_id=COALESCE(EXCLUDED.result_id,analytics_events.result_id),
		 feedback=COALESCE(EXCLUDED.feedback,analytics_events.feedback)`, user, e.ActionID, e.SessionID, e.ProjectID, e.PaperID, e.ResultID, e.SourceActionID, e.Type, producer, e.Feature, e.Outcome, e.CacheHit, e.Locale, e.Device, e.Source, e.Feedback, e.At)
	outcome := "saved"
	if err != nil {
		outcome = "failed"
	}
	eventWrites.WithLabelValues(producer, outcome).Inc()
	return err
}

func rootProject(ctx context.Context, pool *pgxpool.Pool, user, id uuid.UUID) (*uuid.UUID, error) {
	var root uuid.UUID
	err := pool.QueryRow(ctx, `WITH RECURSIVE parents AS (
	 SELECT id,parent_id,system_key,ARRAY[id] AS path FROM library_folders WHERE id=$1 AND user_id=$2
	 UNION ALL SELECT f.id,f.parent_id,f.system_key,p.path||f.id FROM library_folders f JOIN parents p ON f.id=p.parent_id
	 WHERE f.user_id=$2 AND NOT f.id=ANY(p.path)
	) SELECT id FROM parents WHERE parent_id IS NULL AND system_key IS NULL`, id, user).Scan(&root)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &root, nil
}
func projectForPaper(ctx context.Context, pool *pgxpool.Pool, user, paper uuid.UUID) (*uuid.UUID, error) {
	var folder *uuid.UUID
	err := pool.QueryRow(ctx, `SELECT folder_id FROM user_library_items WHERE user_id=$1 AND paper_id=$2`, user, paper).Scan(&folder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if folder == nil {
		return nil, nil
	}
	return rootProject(ctx, pool, user, *folder)
}

func RegisterProject(ctx context.Context, pool *pgxpool.Pool, user, id uuid.UUID, parent *uuid.UUID, created time.Time) {
	if parent != nil || user == uuid.Nil || id == uuid.Nil {
		return
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 750*time.Millisecond)
	defer cancel()
	_, err := pool.Exec(bounded, `INSERT INTO analytics_projects(project_id,user_id,created_at) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, id, user, created)
	if err != nil {
		eventWrites.WithLabelValues("server", "failed").Inc()
	}
}

func ValidateClient(e Event) bool {
	if e.ActionID == uuid.Nil {
		return false
	}
	switch e.Type {
	case "project_opened", "paper_opened", "paper_saved", "note_created", "result_copied", "result_viewed", "feedback":
	default:
		return false
	}
	if e.Feature != "" {
		switch e.Feature {
		case "summary", "translation", "chat", "explanation", "search", "deep_research":
		default:
			return false
		}
	}
	if e.Type == "result_viewed" && e.Feature == "" {
		return false
	}
	if e.Type == "feedback" && (e.Feedback == nil || (*e.Feedback != 1 && *e.Feedback != -1)) {
		return false
	}
	if e.Type == "project_opened" && e.ProjectID == nil {
		return false
	}
	if e.Type == "paper_opened" && e.PaperID == nil {
		return false
	}
	for _, id := range []*uuid.UUID{e.SessionID, e.ProjectID, e.PaperID, e.ResultID, e.SourceActionID} {
		if id != nil && *id == uuid.Nil {
			return false
		}
	}
	return !strings.ContainsAny(e.Feature, "\n\r")
}
