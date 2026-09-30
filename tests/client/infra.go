package client

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// ResetThrottle wipes the auth/resolve rate-limit counters. The whole suite
// comes from one IP, so without this the register limit (5 per 15 minutes)
// would fail unrelated scenarios. Scenarios that test the throttle itself
// call it first and then count requests.
func (s Stand) ResetThrottle(t testing.TB) {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: s.RedisAddr})
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	iter := rdb.Scan(ctx, 0, "ratelimit:*", 500).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		t.Fatalf("redis scan: %v", err)
	}
	if len(keys) > 0 {
		if err := rdb.Del(ctx, keys...).Err(); err != nil {
			t.Fatalf("redis del: %v", err)
		}
	}
}

// DB opens a direct connection for "time travel" (expiring tokens) and for
// checks the API deliberately does not expose. Scenarios should prefer the API.
func (s Stand) DB(t testing.TB) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, s.PostgresURL)
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return conn
}

// ExpireAuthTokens moves every one-time token of the user into the past.
func (s Stand) ExpireAuthTokens(t testing.TB, email string) {
	t.Helper()
	conn := s.DB(t)
	_, err := conn.Exec(context.Background(),
		`UPDATE auth_tokens SET expires_at = now() - interval '1 minute'
		 WHERE user_id = (SELECT id FROM users WHERE email = $1)`, email)
	if err != nil {
		t.Fatalf("expire tokens: %v", err)
	}
}

// AppliedMigrations lists schema_migrations ids.
func (s Stand) AppliedMigrations(t testing.TB) []string {
	t.Helper()
	conn := s.DB(t)
	rows, err := conn.Query(context.Background(), `SELECT id FROM schema_migrations ORDER BY id`)
	if err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, id)
	}
	return out
}

// DetachSummaryVersion forgets which parse a cached overview was built from,
// which is what a re-parse does to it: the API must then report it stale.
func (s Stand) DetachSummaryVersion(t testing.TB, paperID, lang string) {
	t.Helper()
	conn := s.DB(t)
	tag, err := conn.Exec(context.Background(), `UPDATE paper_summaries SET version_id = NULL WHERE paper_id = $1 AND lang = $2`, paperID, lang)
	if err != nil || tag.RowsAffected() == 0 {
		t.Fatalf("detach summary version (%d rows): %v", tag.RowsAffected(), err)
	}
}

// DocumentStatus reads paper_documents.status ("" when never parsed).
func (s Stand) DocumentStatus(t testing.TB, paperID string) (status string, errorMessage string) {
	t.Helper()
	conn := s.DB(t)
	var msg *string
	err := conn.QueryRow(context.Background(), `SELECT status, error_message FROM paper_documents WHERE paper_id = $1`, paperID).Scan(&status, &msg)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", ""
		}
		t.Fatalf("paper_documents: %v", err)
	}
	if msg != nil {
		errorMessage = *msg
	}
	return status, errorMessage
}

// SetDocumentStatus rewrites paper_documents.status, e.g. back to 'pending'
// to reproduce "the text is still parsing" without racing the parser.
func (s Stand) SetDocumentStatus(t testing.TB, paperID, status string) {
	t.Helper()
	conn := s.DB(t)
	tag, err := conn.Exec(context.Background(), `UPDATE paper_documents SET status = $2 WHERE paper_id = $1`, paperID, status)
	if err != nil || tag.RowsAffected() == 0 {
		t.Fatalf("set document status (%d rows): %v", tag.RowsAffected(), err)
	}
}
