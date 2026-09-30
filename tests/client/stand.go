// Package client is the "artificial user" of the integration suite: it talks
// to the test stand the way the frontend does (through the gateway) and hides
// URLs, JSON and polling from the scenarios.
package client

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

// Stand describes where the running test stand is reachable from the host.
// Defaults match docker-compose.test.yml; override with env for a remote stand.
type Stand struct {
	GatewayURL     string
	MailpitURL     string
	FakeLLMURL     string
	FakeScholarURL string
	PostgresURL    string
	RedisAddr      string
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func FromEnv() Stand {
	return Stand{
		GatewayURL:     envOr("STAND_GATEWAY_URL", "http://localhost:18081"),
		MailpitURL:     envOr("STAND_MAILPIT_URL", "http://localhost:18025"),
		FakeLLMURL:     envOr("STAND_FAKE_LLM_URL", "http://localhost:18094"),
		FakeScholarURL: envOr("STAND_FAKE_SCHOLAR_URL", "http://localhost:18093"),
		PostgresURL:    envOr("STAND_POSTGRES_URL", "postgres://researcher:researcher@localhost:15433/researcher"),
		RedisAddr:      envOr("STAND_REDIS_ADDR", "localhost:16379"),
	}
}

// WaitReady blocks until every HTTP piece of the stand answers its health
// endpoint, or fails after timeout with the last error per endpoint.
func (s Stand) WaitReady(timeout time.Duration) error {
	endpoints := map[string]string{
		"gateway":      s.GatewayURL + "/health",
		"mailpit":      s.MailpitURL + "/readyz",
		"fake-llm":     s.FakeLLMURL + "/health",
		"fake-scholar": s.FakeScholarURL + "/health",
	}
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	last := map[string]error{}
	for {
		pending := 0
		for name, url := range endpoints {
			resp, err := client.Get(url)
			if err != nil {
				last[name] = err
				pending++
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				last[name] = fmt.Errorf("status %d", resp.StatusCode)
				pending++
				continue
			}
			delete(last, name)
		}
		if pending == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("stand not ready: %v", last)
		}
		time.Sleep(time.Second)
	}
}

// Guest returns a session without credentials.
func (s Stand) Guest(t testing.TB) *Session {
	t.Helper()
	return newSession(t, s)
}
