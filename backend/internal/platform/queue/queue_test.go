package queue

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
)

func TestEnqueueDropsDuplicateVersionTasks(t *testing.T) {
	mr := miniredis.RunT(t)
	c := asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })

	for i := 0; i < 3; i++ {
		if err := Enqueue(c, ProcessArxivPDF, "version-1"); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	payload, _ := json.Marshal(Payload{VersionID: "version-1"})
	if _, err := c.Enqueue(asynq.NewTask(ProcessArxivPDF, payload), asynq.Unique(dedupeWindow)); !errors.Is(err, asynq.ErrDuplicateTask) {
		t.Fatalf("the first task must hold the lock, got %v", err)
	}
	if err := Enqueue(c, ProcessArxivPDF, "version-2"); err != nil {
		t.Fatalf("another version is not a duplicate: %v", err)
	}
	if err := Enqueue(c, ProcessPaperParse, "version-1"); err != nil {
		t.Fatalf("another task type is not a duplicate: %v", err)
	}
}
