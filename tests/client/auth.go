package client

import (
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

const DefaultPassword = "correct-horse-battery"

// Nonce returns a short unique token to make prompts and texts distinguishable
// in the fakes' request logs, even across runs on a persistent stand.
func Nonce() string {
	return fmt.Sprintf("%d-%04d", time.Now().UnixNano()/1e6%1e9, rand.Intn(10000))
}

// UniqueEmail returns an address nobody else in the suite uses.
func UniqueEmail(prefix string) string {
	return fmt.Sprintf("%s-%d-%04d@test.local", prefix, time.Now().UnixNano()/1e6, rand.Intn(10000))
}

// User is a registered, verified account with one logged-in session.
type User struct {
	*Session
	Password string
}

// Register submits the sign-up form. Returns the raw response so scenarios
// can check the generic reply.
func (s *Session) Register(email, password string) Response {
	s.t.Helper()
	s.Email = email
	return s.Post("/auth/register", map[string]string{"email": email, "password": password})
}

// Login submits the sign-in form and keeps the access token on success.
func (s *Session) Login(email, password string) Response {
	s.t.Helper()
	resp := s.Post("/auth/login", map[string]string{"email": email, "password": password})
	if resp.Status == 200 {
		s.Email = email
		s.Token = resp.Map(s.t)["access_token"].(string)
	}
	return resp
}

// Refresh exchanges the cookie for a new access token (silent refresh).
func (s *Session) Refresh() Response {
	s.t.Helper()
	resp := s.Post("/auth/refresh", nil)
	if resp.Status == 200 {
		s.Token = resp.Map(s.t)["access_token"].(string)
	}
	return resp
}

// Logout clears the server session and the in-memory token.
func (s *Session) Logout() Response {
	s.t.Helper()
	resp := s.Post("/auth/logout", nil)
	s.Token = ""
	return resp
}

// VerifyEmail consumes a verification token; on success the session is logged in.
func (s *Session) VerifyEmail(token string) Response {
	s.t.Helper()
	resp := s.Post("/auth/verify-email", map[string]string{"token": token})
	if resp.Status == 200 {
		s.Token = resp.Map(s.t)["access_token"].(string)
	}
	return resp
}

// RegisterAndVerify walks the whole sign-up: register → mail → verify. The
// returned user is logged in. The auth throttle is reset first so a long
// suite from one IP does not trip the 5-per-15-minutes register limit.
func (st Stand) RegisterAndVerify(t testing.TB, prefix string) *User {
	t.Helper()
	st.ResetThrottle(t)
	s := newSession(t, st)
	email := UniqueEmail(prefix)
	before := st.Mailpit().Count(t, email)
	resp := s.Register(email, DefaultPassword)
	if resp.Status != http.StatusOK {
		t.Fatalf("register %s: %s", email, resp)
	}
	mail := st.Mailpit().WaitMessage(t, email, before, 15*time.Second)
	if resp := s.VerifyEmail(mail.Token(t, "verify-email")); resp.Status != http.StatusOK {
		t.Fatalf("verify %s: %s", email, resp)
	}
	return &User{Session: s, Password: DefaultPassword}
}

// LoginAs opens a second device for an existing user.
func (u *User) LoginAs(t testing.TB) *Session {
	t.Helper()
	s := u.Clone()
	if resp := s.Login(u.Email, u.Password); resp.Status != 200 {
		t.Fatalf("login %s: %s", u.Email, resp)
	}
	return s
}
