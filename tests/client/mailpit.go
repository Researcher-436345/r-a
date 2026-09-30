package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Mailpit reads the dev mailbox the stand delivers to.
type Mailpit struct{ URL string }

func (s Stand) Mailpit() Mailpit { return Mailpit{URL: s.MailpitURL} }

type mailSummary struct {
	ID      string `json:"ID"`
	Subject string `json:"Subject"`
	Created string `json:"Created"`
	To      []struct {
		Address string `json:"Address"`
	} `json:"To"`
}

// Mail is one delivered message.
type Mail struct {
	ID      string
	Subject string
	Text    string
	HTML    string
}

// Messages lists messages addressed to `to`, newest first.
func (m Mailpit) Messages(t testing.TB, to string) []mailSummary {
	t.Helper()
	q := url.QueryEscape(fmt.Sprintf("to:%q", to))
	resp, err := http.Get(m.URL + "/api/v1/search?query=" + q + "&limit=50")
	if err != nil {
		t.Fatalf("mailpit search: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Messages []mailSummary `json:"messages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("mailpit search decode: %v", err)
	}
	var filtered []mailSummary
	for _, msg := range out.Messages {
		for _, rcpt := range msg.To {
			if strings.EqualFold(rcpt.Address, to) {
				filtered = append(filtered, msg)
				break
			}
		}
	}
	return filtered
}

// Count returns how many messages `to` has received.
func (m Mailpit) Count(t testing.TB, to string) int {
	t.Helper()
	return len(m.Messages(t, to))
}

// WaitMessage waits until `to` has more than `already` messages and returns
// the newest one. Mail is sent asynchronously by the backend, hence the poll.
func (m Mailpit) WaitMessage(t testing.TB, to string, already int, timeout time.Duration) Mail {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		msgs := m.Messages(t, to)
		if len(msgs) > already {
			return m.Read(t, msgs[0].ID)
		}
		if time.Now().After(deadline) {
			t.Fatalf("no new mail for %s within %s (have %d)", to, timeout, len(msgs))
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// Read fetches the full message body.
func (m Mailpit) Read(t testing.TB, id string) Mail {
	t.Helper()
	resp, err := http.Get(m.URL + "/api/v1/message/" + id)
	if err != nil {
		t.Fatalf("mailpit read: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		ID      string `json:"ID"`
		Subject string `json:"Subject"`
		Text    string `json:"Text"`
		HTML    string `json:"HTML"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("mailpit read decode: %v", err)
	}
	return Mail{ID: out.ID, Subject: out.Subject, Text: out.Text, HTML: out.HTML}
}

var tokenRE = regexp.MustCompile(`[?&]token=([A-Za-z0-9_\-]+)`)

// Token extracts the one-time token from a verify/reset link in the mail. The
// path argument ("verify-email" / "reset-password") guards against picking a
// link of the wrong kind.
func (ml Mail) Token(t testing.TB, path string) string {
	t.Helper()
	for _, line := range strings.Fields(ml.Text) {
		if strings.Contains(line, "/"+path+"?") {
			if m := tokenRE.FindStringSubmatch(line); len(m) == 2 {
				return m[1]
			}
		}
	}
	t.Fatalf("no %s token in mail %q:\n%s", path, ml.Subject, ml.Text)
	return ""
}
