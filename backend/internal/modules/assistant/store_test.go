package assistant

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Run with ASSISTANT_TEST_DATABASE_URL pointing to a PostgreSQL test instance.
// All rows live in a connection-local temporary table, not the application's table.
func TestMessageHistoryPairOrder(t *testing.T) {
	dsn := os.Getenv("ASSISTANT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ASSISTANT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TEMP TABLE chat_messages (
		id uuid PRIMARY KEY, paper_id uuid, user_id uuid, role text,
		content text, context_text text, created_at timestamptz DEFAULT now()
	)`)
	if err != nil {
		t.Fatal(err)
	}
	userID, paperID := uuid.New(), uuid.New()
	// UUID order is deliberately opposite to conversational order for both pairs.
	_, err = pool.Exec(ctx, `INSERT INTO chat_messages
		(id, paper_id, user_id, role, content, created_at) VALUES
		('00000000-0000-0000-0000-000000000004', $1, $2, 'user', 'question 1', '2026-01-01'),
		('00000000-0000-0000-0000-000000000003', $1, $2, 'assistant', 'answer 1', '2026-01-01'),
		('00000000-0000-0000-0000-000000000002', $1, $2, 'user', 'question 2', '2026-01-02'),
		('00000000-0000-0000-0000-000000000001', $1, $2, 'assistant', 'answer 2', '2026-01-02')`, paperID, userID)
	if err != nil {
		t.Fatal(err)
	}
	store := Store{DB: pool}
	assertContents := func(items []Message, err error, want ...string) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != len(want) {
			t.Fatalf("got %d messages, want %d", len(items), len(want))
		}
		for i := range want {
			if items[i].Content != want[i] {
				t.Fatalf("message %d: got %q, want %q", i, items[i].Content, want[i])
			}
		}
	}
	items, err := store.List(ctx, userID, paperID)
	assertContents(items, err, "question 1", "answer 1", "question 2", "answer 2")
	items, err = store.ListRecent(ctx, userID, paperID, 2)
	assertContents(items, err, "question 2", "answer 2")
	items, err = store.ListRecent(ctx, userID, paperID, 1)
	assertContents(items, err, "answer 2")
	items, err = store.ListRecent(ctx, userID, paperID, 3)
	assertContents(items, err, "answer 1", "question 2", "answer 2")
}
