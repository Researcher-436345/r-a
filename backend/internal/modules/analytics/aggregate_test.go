package analytics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("ANALYTICS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ANALYTICS_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "analytics_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	_, err = pool.Exec(ctx, `CREATE TABLE users(id uuid PRIMARY KEY,created_at timestamptz NOT NULL);
	 CREATE TABLE library_folders(id uuid PRIMARY KEY,user_id uuid NOT NULL,parent_id uuid,system_key text,created_at timestamptz NOT NULL);
	 CREATE TABLE user_library_items(user_id uuid,paper_id uuid,folder_id uuid);`)
	if err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	for _, name := range []string{"011_observability.sql", "012_test_accounts.sql"} {
		migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../migrations/", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(migration)); err != nil {
			t.Fatal("migration:", err)
		}
	}
	return pool
}
func sampleValue(t *testing.T, samples []Sample, name string, labels ...string) float64 {
	t.Helper()
	for _, s := range samples {
		if s.Name == name && strings.Join(s.Labels, "|") == strings.Join(labels, "|") {
			return s.Value
		}
	}
	t.Fatalf("missing metric %s %v", name, labels)
	return 0
}
func TestMatureCohortsDifferentDayReturnsAndDedupe(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	now := time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC)
	start := now.AddDate(0, 0, -100)
	if _, err := pool.Exec(ctx, `UPDATE analytics_settings SET started_at=$1`, start); err != nil {
		t.Fatal(err)
	}
	u1, u2, u3 := uuid.New(), uuid.New(), uuid.New()
	base := time.Date(2026, 11, 23, 12, 0, 0, 0, time.UTC)
	for i, u := range []uuid.UUID{u1, u2, u3} {
		created := base
		if i == 2 {
			created = now.AddDate(0, 0, -2)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO users(id,created_at)VALUES($1,$2)`, u, created); err != nil {
			t.Fatal(err)
		}
	}
	p1, p2 := uuid.New(), uuid.New()
	for i, p := range []uuid.UUID{p1, p2} {
		u := []uuid.UUID{u1, u2}[i]
		if _, err := pool.Exec(ctx, `INSERT INTO library_folders(id,user_id,created_at) VALUES($1,$2,$3);`, p, u, base); err != nil {
			t.Fatal(err)
		}
		RegisterProject(ctx, pool, u, p, nil, base)
	}
	returnAt := base.Add(7*24*time.Hour - time.Hour)
	action := uuid.New()
	e := Event{ActionID: action, Type: "action_started", Feature: "summary", At: base, ProjectID: &p1}
	if err := insert(ctx, pool, u1, e, "server"); err != nil {
		t.Fatal(err)
	}
	e.Type = "action_finished"
	e.Outcome = "failed"
	e.At = returnAt
	if err := insert(ctx, pool, u1, e, "server"); err != nil {
		t.Fatal(err)
	}
	e.Outcome = "success"
	if err := insert(ctx, pool, u1, e, "server"); err != nil {
		t.Fatal(err)
	}
	e.Type = "result_viewed"
	e.Outcome = ""
	for i := 0; i < 2; i++ {
		if err := insert(ctx, pool, u1, e, "client"); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := insert(ctx, pool, u2, Event{ActionID: uuid.New(), Type: "project_opened", ProjectID: &p2, At: base.Add(time.Duration(i+1) * time.Hour)}, "client"); err != nil {
			t.Fatal(err)
		}
	}
	samples, err := Aggregate(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		name   string
		labels []string
		want   float64
	}{
		{"retention_users", []string{"d7", "eligible"}, 2}, {"retention_users", []string{"d7", "returned"}, 1},
		{"project_return_projects", []string{"7d", "eligible"}, 2}, {"project_return_projects", []string{"7d", "returned"}, 1},
		{"feature_uses", []string{"summary"}, 1}, {"feature_users", []string{"summary"}, 1},
		{"activation_users", []string{"eligible"}, 2}, {"activation_users", []string{"activated"}, 1},
	} {
		t.Run(fmt.Sprint(check.name, check.labels), func(t *testing.T) {
			if got := sampleValue(t, samples, check.name, check.labels...); got != check.want {
				t.Fatalf("got %v want %v", got, check.want)
			}
		})
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM analytics_events WHERE user_id=$1 AND action_id=$2 AND event_type='action_finished' AND outcome='success'`, u1, action).Scan(&n); err != nil || n != 1 {
		t.Fatalf("logical retries were not deduplicated: n=%d err=%v", n, err)
	}
}
func TestNoMatureCohortHasZeroDenominatorAndNoInventedLatency(t *testing.T) {
	pool := testDB(t)
	samples, err := Aggregate(context.Background(), pool, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if sampleValue(t, samples, "retention_users", "d30", "eligible") != 0 {
		t.Fatal("empty cohort is not empty")
	}
	for _, s := range samples {
		if s.Name == "time_to_first_value_seconds" {
			t.Fatal("invented latency without observations")
		}
	}
}
func TestClientCannotForgeServerOutcomes(t *testing.T) {
	for _, kind := range []string{"action_started", "action_finished", "unknown"} {
		if ValidateClient(Event{ActionID: uuid.New(), Type: kind}) {
			t.Fatal("accepted server event", kind)
		}
	}
	if ValidateClient(Event{ActionID: uuid.New(), Type: "result_viewed", Feature: "arbitrary"}) {
		t.Fatal("accepted unbounded feature")
	}
}

func TestTestAccountsDoNotAffectProductMetrics(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	now := time.Date(2026, 12, 1, 20, 0, 0, 0, time.UTC)
	created := now.AddDate(0, 0, -31)
	if _, err := pool.Exec(ctx, `UPDATE analytics_settings SET started_at=$1`, now.AddDate(0, 0, -100)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,created_at) VALUES($1,$2)`, uuid.New(), created); err != nil {
		t.Fatal(err)
	}
	before, err := Aggregate(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	user, project := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users(id,created_at,is_test) VALUES($1,$2,true)`, user, created); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO library_folders(id,user_id,created_at) VALUES($1,$2,$3)`, project, user, created); err != nil {
		t.Fatal(err)
	}
	RegisterProject(ctx, pool, user, project, nil, created)
	positive := int16(1)
	for _, kind := range []string{"action_started", "action_finished", "result_viewed", "project_opened", "paper_opened", "paper_saved", "note_created", "result_copied", "feedback"} {
		e := Event{ActionID: uuid.New(), Type: kind, Feature: "summary", ProjectID: &project, At: now.Add(-time.Hour)}
		if kind == "action_finished" {
			e.Outcome = "success"
		}
		if kind == "feedback" {
			e.Feedback = &positive
		}
		if err := insert(ctx, pool, user, e, "server"); err != nil {
			t.Fatal(err)
		}
	}
	for _, feature := range []string{"search", "deep_research"} {
		action := uuid.New()
		if err := insert(ctx, pool, user, Event{ActionID: action, Type: "action_finished", Feature: feature, Outcome: "success", At: now.AddDate(0, 0, -8)}, "server"); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"paper_opened", "paper_saved"} {
			if err := insert(ctx, pool, user, Event{ActionID: uuid.New(), Type: kind, SourceActionID: &action, ProjectID: &project, At: now.AddDate(0, 0, -7)}, "client"); err != nil {
				t.Fatal(err)
			}
		}
	}
	after, err := Aggregate(ctx, pool, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("test users added product metric series: before=%d after=%d", len(before), len(after))
	}
	for _, s := range before {
		if got := sampleValue(t, after, s.Name, s.Labels...); got != s.Value {
			t.Errorf("test user changed %s %v: got %v want %v", s.Name, s.Labels, got, s.Value)
		}
	}
	if got := sampleValue(t, after, "registered_users"); got != 1 {
		t.Fatalf("registered users includes test account: %v", got)
	}
	var raw int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM analytics_events WHERE user_id=$1`, user).Scan(&raw); err != nil || raw == 0 {
		t.Fatalf("test events must remain available for diagnostics: rows=%d err=%v", raw, err)
	}
}
