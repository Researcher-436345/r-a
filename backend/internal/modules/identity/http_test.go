package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/mailer"
)

// --- fakes --------------------------------------------------------------------

type fakeUserRow struct {
	user User
	pass string
}

type fakeSessionRow struct {
	Session
	tokenHash string
}

type fakeOneTime struct {
	userID    uuid.UUID
	typ       string
	expiresAt time.Time
	usedAt    *time.Time
}

type fakeStore struct {
	mu       sync.Mutex
	users    map[string]*fakeUserRow // by email
	byID     map[uuid.UUID]*fakeUserRow
	sessions map[string]*fakeSessionRow // by token hash
	tokens   map[string]*fakeOneTime    // by "type|hash"
	now      func() time.Time
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:    map[string]*fakeUserRow{},
		byID:     map[uuid.UUID]*fakeUserRow{},
		sessions: map[string]*fakeSessionRow{},
		tokens:   map[string]*fakeOneTime{},
		now:      time.Now,
	}
}

func (f *fakeStore) CreateUser(_ context.Context, email, passwordHash string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.users[email]; ok {
		return User{}, errors.New("duplicate key value violates unique constraint")
	}
	u := User{ID: uuid.New(), Email: email, PasswordHash: passwordHash}
	f.users[email] = &fakeUserRow{user: u, pass: passwordHash}
	f.byID[u.ID] = f.users[email]
	return u, nil
}

func (f *fakeStore) GetByEmail(_ context.Context, email string) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row, ok := f.users[email]; ok {
		return row.user, nil
	}
	return User{}, pgx.ErrNoRows
}

func (f *fakeStore) GetByID(_ context.Context, id uuid.UUID) (User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row, ok := f.byID[id]; ok {
		return row.user, nil
	}
	return User{}, pgx.ErrNoRows
}

func (f *fakeStore) MarkVerified(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.byID[userID]
	if row == nil {
		return pgx.ErrNoRows
	}
	now := f.now()
	row.user.EmailVerifiedAt = &now
	return nil
}

func (f *fakeStore) UpdatePassword(_ context.Context, userID uuid.UUID, passwordHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.byID[userID]
	if row == nil {
		return pgx.ErrNoRows
	}
	row.pass = passwordHash
	row.user.PasswordHash = passwordHash
	return nil
}

func (f *fakeStore) CreateSession(_ context.Context, userID uuid.UUID, tokenHash, userAgent, ip string, expiresAt time.Time) (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	sess := Session{
		ID: uuid.New(), UserID: userID, UserAgent: userAgent, IP: ip,
		CreatedAt: now, LastUsedAt: now, ExpiresAt: expiresAt,
	}
	f.sessions[tokenHash] = &fakeSessionRow{Session: sess, tokenHash: tokenHash}
	return sess, nil
}

func (f *fakeStore) GetSessionByToken(_ context.Context, tokenHash string) (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if row, ok := f.sessions[tokenHash]; ok {
		return row.Session, nil
	}
	return Session{}, pgx.ErrNoRows
}

func (f *fakeStore) RotateSession(_ context.Context, old Session, tokenHash string, expiresAt time.Time) (Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var oldRow *fakeSessionRow
	for _, row := range f.sessions {
		if row.Session.ID == old.ID {
			oldRow = row
			break
		}
	}
	if oldRow == nil || oldRow.Session.RevokedAt != nil {
		return Session{}, pgx.ErrNoRows
	}
	now := f.now()
	newSess := Session{
		ID: uuid.New(), UserID: oldRow.Session.UserID,
		UserAgent: oldRow.Session.UserAgent, IP: oldRow.Session.IP,
		CreatedAt: now, LastUsedAt: now, ExpiresAt: expiresAt,
	}
	oldRow.Session.RevokedAt = &now
	oldRow.Session.ReplacedBy = &newSess.ID
	f.sessions[tokenHash] = &fakeSessionRow{Session: newSess, tokenHash: tokenHash}
	return newSess, nil
}

func (f *fakeStore) RevokeSession(_ context.Context, userID, sessionID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.sessions {
		if row.Session.ID == sessionID && row.Session.UserID == userID && row.Session.RevokedAt == nil {
			now := f.now()
			row.Session.RevokedAt = &now
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeStore) RevokeAllSessions(_ context.Context, userID, exceptID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := f.now()
	for _, row := range f.sessions {
		if row.Session.UserID == userID && row.Session.RevokedAt == nil && row.Session.ID != exceptID {
			row.Session.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeStore) ListActiveSessions(_ context.Context, userID uuid.UUID) ([]Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Session
	for _, row := range f.sessions {
		if row.Session.UserID == userID && row.Session.RevokedAt == nil && f.now().Before(row.Session.ExpiresAt) {
			out = append(out, row.Session)
		}
	}
	return out, nil
}

func (f *fakeStore) CreateAuthToken(_ context.Context, userID uuid.UUID, typ, tokenHash string, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[typ+"|"+tokenHash] = &fakeOneTime{userID: userID, typ: typ, expiresAt: f.now().Add(ttl)}
	return nil
}

func (f *fakeStore) ConsumeAuthToken(_ context.Context, typ, tokenHash string) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rec, ok := f.tokens[typ+"|"+tokenHash]
	if !ok || rec.usedAt != nil || f.now().After(rec.expiresAt) {
		return uuid.Nil, pgx.ErrNoRows
	}
	now := f.now()
	rec.usedAt = &now
	return rec.userID, nil
}

// ExpireAuthToken forces a token's expiry (expiry-path testing).
func (f *fakeStore) ExpireAuthToken(typ, tokenHash string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if rec, ok := f.tokens[typ+"|"+tokenHash]; ok {
		rec.expiresAt = f.now().Add(-time.Second)
	}
}

// ActiveSessionCount returns how many non-revoked sessions the user has.
func (f *fakeStore) ActiveSessionCount(userID uuid.UUID) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, row := range f.sessions {
		if row.Session.UserID == userID && row.Session.RevokedAt == nil {
			n++
		}
	}
	return n
}

// SessionByID returns a session row by id (any state).
func (f *fakeStore) SessionByID(id uuid.UUID) (Session, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.sessions {
		if row.Session.ID == id {
			return row.Session, true
		}
	}
	return Session{}, false
}

type fakeMail struct {
	mu       sync.Mutex
	messages []mailer.Message
}

func (f *fakeMail) Send(msg mailer.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msg)
	return nil
}

func (f *fakeMail) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.messages)
}

var tokenFromLink = regexp.MustCompile(`token=([A-Za-z0-9_\-]+)`)

func (f *fakeMail) lastToken() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) == 0 {
		return ""
	}
	if m := tokenFromLink.FindStringSubmatch(f.messages[len(f.messages)-1].Text); m != nil {
		return m[1]
	}
	return ""
}

// --- harness -------------------------------------------------------------------

func newTestAPI(t *testing.T, verificationEnabled ...bool) (http.Handler, *fakeStore, *fakeMail) {
	t.Helper()
	fs := newFakeStore()
	fm := &fakeMail{}
	mr := miniredis.RunT(t)

	api := API{
		Config: config.Config{
			EmailVerificationEnabled: true,
			JWTSecret:                "test-secret",
			AccessTokenTTL:           30 * time.Minute,
			RefreshTokenTTL:          14 * 24 * time.Hour,
			FrontendURL:              "http://localhost:5173",
		},
		Store: fs,
		Redis: redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		Mail:  fm,
	}
	if len(verificationEnabled) > 0 {
		api.Config.EmailVerificationEnabled = verificationEnabled[0]
	}
	r := chi.NewRouter()
	api.Mount(r)
	return r, fs, fm
}

func TestVerificationDisabled(t *testing.T) {
	h, fs, fm := newTestAPI(t, false)
	credentials := map[string]string{"email": "no-verify@example.com", "password": "Password123"}
	for i := 0; i < 2; i++ {
		rec := doJSON(t, h, http.MethodPost, "/auth/register", credentials, nil, "")
		if rec.Code != 200 || loginBody(t, rec)["email_verification_required"] != false {
			t.Fatalf("registration should allow sign-in without email: %d %s", rec.Code, rec.Body.String())
		}
	}
	u, _ := fs.GetByEmail(context.Background(), credentials["email"])
	if u.Verified() {
		t.Fatal("disabling verification must not mark an email as verified")
	}
	rec := doJSON(t, h, http.MethodPost, "/auth/login", credentials, nil, "")
	if rec.Code != 200 {
		t.Fatalf("unverified login: %d %s", rec.Code, rec.Body.String())
	}
	refreshCookieOf(t, rec)
	wrong := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": credentials["email"], "password": "WrongPassword"}, nil, "")
	if wrong.Code != 401 {
		t.Fatalf("wrong password must still fail: %d", wrong.Code)
	}
	doJSON(t, h, http.MethodPost, "/auth/resend-verification", map[string]string{"email": credentials["email"]}, nil, "")
	if fm.count() != 0 || len(fs.tokens) != 0 {
		t.Fatal("disabled verification must not send emails or create verification tokens")
	}
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("User-Agent", "test-agent/1.0")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func loginBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
	}
	return m
}

func refreshCookieOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == RefreshCookieName {
			return c
		}
	}
	t.Fatalf("no %s cookie in response", RefreshCookieName)
	return nil
}

// registerUser creates a fresh (unverified) account through the API.
func registerUser(t *testing.T, h http.Handler, fs *fakeStore, email string) {
	t.Helper()
	if rec := doJSON(t, h, http.MethodPost, "/auth/register", map[string]string{"email": email, "password": "Password123"}, nil, ""); rec.Code != 200 {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := fs.GetByEmail(context.Background(), email); err != nil {
		t.Fatal(err)
	}
}

// verifiedUser registers and confirms the account through the verify-email flow.
func verifiedUser(t *testing.T, h http.Handler, fs *fakeStore, fm *fakeMail, email string) {
	t.Helper()
	registerUser(t, h, fs, email)
	token := fm.lastToken()
	if token == "" {
		t.Fatal("expected a verify email with a token")
	}
	if rec := doJSON(t, h, http.MethodPost, "/auth/verify-email", map[string]string{"token": token}, nil, ""); rec.Code != 200 {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
}

// --- tests ---------------------------------------------------------------------

func TestNewOpaqueTokenShape(t *testing.T) {
	raw1, hash1, err := newToken()
	if err != nil {
		t.Fatal(err)
	}
	_, hash2, _ := newToken()
	if len(raw1) != 43 || strings.ContainsAny(raw1, "+/=") {
		t.Fatalf("unexpected token shape %q", raw1)
	}
	if len(hash1) != 64 {
		t.Fatalf("hash must be sha256 hex, got %q", hash1)
	}
	if hash1 == hash2 {
		t.Fatal("two tokens must not share a hash")
	}
	if hashToken(raw1) != hash1 {
		t.Fatal("hashToken must be deterministic")
	}
}

func TestRegisterIsGenericAndSendsVerifyEmail(t *testing.T) {
	h, fs, fm := newTestAPI(t)

	rec := doJSON(t, h, http.MethodPost, "/auth/register", map[string]string{"email": "a@example.com", "password": "Password123"}, nil, "")
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "check your email") || strings.Contains(body, "access_token") {
		t.Fatalf("register must be generic without tokens, got %s", body)
	}
	if fm.count() != 1 || fm.lastToken() == "" {
		t.Fatalf("expected one verify email with token, got %d", fm.count())
	}

	// Same unverified email: same response, resend.
	rec2 := doJSON(t, h, http.MethodPost, "/auth/register", map[string]string{"email": "a@example.com", "password": "Password123"}, nil, "")
	if rec2.Code != 200 || rec2.Body.String() != body {
		t.Fatalf("repeat register must be identical: %d %s", rec2.Code, rec2.Body.String())
	}
	if fm.count() != 2 {
		t.Fatalf("expected resend, got %d emails", fm.count())
	}

	// Verified existing email: same generic response, no new email.
	u, _ := fs.GetByEmail(context.Background(), "a@example.com")
	_ = fs.MarkVerified(context.Background(), u.ID)
	rec3 := doJSON(t, h, http.MethodPost, "/auth/register", map[string]string{"email": "a@example.com", "password": "Password123"}, nil, "")
	if rec3.Code != 200 || rec3.Body.String() != body {
		t.Fatalf("verified re-register must stay generic: %d %s", rec3.Code, rec3.Body.String())
	}
	if fm.count() != 2 {
		t.Fatalf("no email for verified users, got %d", fm.count())
	}

	// Weak password → 400 (not generic-mapped: it is a client error).
	rec4 := doJSON(t, h, http.MethodPost, "/auth/register", map[string]string{"email": "b@example.com", "password": "short"}, nil, "")
	if rec4.Code != 400 {
		t.Fatalf("weak password must 400, got %d", rec4.Code)
	}
}

func TestVerifyEmailSingleUseAutoLogin(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	registerUser(t, h, fs, "v@example.com")
	token := fm.lastToken()
	if token == "" {
		t.Fatal("no verify token captured")
	}

	rec := doJSON(t, h, http.MethodPost, "/auth/verify-email", map[string]string{"token": token}, nil, "")
	if rec.Code != 200 {
		t.Fatalf("verify: %d %s", rec.Code, rec.Body.String())
	}
	resp := loginBody(t, rec)
	if resp["access_token"] == "" {
		t.Fatal("verify must auto-login and return access_token")
	}
	refreshCookieOf(t, rec) // session cookie is set

	// Second use of the same token → 400.
	rec2 := doJSON(t, h, http.MethodPost, "/auth/verify-email", map[string]string{"token": token}, nil, "")
	if rec2.Code != 400 {
		t.Fatalf("token reuse must 400, got %d", rec2.Code)
	}

	// Unknown token → 400.
	rec3 := doJSON(t, h, http.MethodPost, "/auth/verify-email", map[string]string{"token": "no-such-token"}, nil, "")
	if rec3.Code != 400 {
		t.Fatalf("unknown token must 400, got %d", rec3.Code)
	}
}

func TestVerifyEmailExpiredToken(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	registerUser(t, h, fs, "e@example.com")
	token := fm.lastToken()
	hash := hashToken(token)

	fs.mu.Lock()
	if rec, ok := fs.tokens["verify_email|"+hash]; ok {
		rec.expiresAt = fs.now().Add(-time.Minute)
	}
	fs.mu.Unlock()

	rec := doJSON(t, h, http.MethodPost, "/auth/verify-email", map[string]string{"token": token}, nil, "")
	if rec.Code != 400 {
		t.Fatalf("expired token must 400, got %d", rec.Code)
	}
}

func TestLoginRequiresVerification(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	registerUser(t, h, fs, "l@example.com")

	rec := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "l@example.com", "password": "Password123"}, nil, "")
	if rec.Code != 403 {
		t.Fatalf("unverified login must 403, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "email_not_verified") {
		t.Fatalf("403 must carry code, got %s", rec.Body.String())
	}
	if fm.count() != 2 { // register + auto-resend
		t.Fatalf("403 must auto-resend, emails=%d", fm.count())
	}

	// Wrong password → generic 401 (and it must not reveal verification state).
	recWrong := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "l@example.com", "password": "WrongPass123"}, nil, "")
	if recWrong.Code != 401 || !strings.Contains(recWrong.Body.String(), "Incorrect email or password") {
		t.Fatalf("wrong password must be generic 401: %d %s", recWrong.Code, recWrong.Body.String())
	}

	// After verification login succeeds and sets the session cookie.
	token := fm.lastToken()
	if recV := doJSON(t, h, http.MethodPost, "/auth/verify-email", map[string]string{"token": token}, nil, ""); recV.Code != 200 {
		t.Fatalf("verify: %d %s", recV.Code, recV.Body.String())
	}
	recOK := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "l@example.com", "password": "Password123"}, nil, "")
	if recOK.Code != 200 {
		t.Fatalf("verified login: %d %s", recOK.Code, recOK.Body.String())
	}
	c := refreshCookieOf(t, recOK)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("refresh cookie must be HttpOnly+Lax, got %+v", c)
	}
	if loginBody(t, recOK)["refresh_token"] != nil {
		t.Fatal("refresh token must not be in the JSON body")
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	verifiedUser(t, h, fs, fm, "r@example.com")
	u, _ := fs.GetByEmail(context.Background(), "r@example.com")

	recLogin := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "r@example.com", "password": "Password123"}, nil, "")
	cookie1 := refreshCookieOf(t, recLogin)

	// Rotate: old session must be revoked with replaced_by pointing to the new one.
	recRefresh := doJSON(t, h, http.MethodPost, "/auth/refresh", map[string]string{}, cookie1, "")
	if recRefresh.Code != 200 {
		t.Fatalf("refresh: %d %s", recRefresh.Code, recRefresh.Body.String())
	}
	cookie2 := refreshCookieOf(t, recRefresh)
	if cookie2.Value == cookie1.Value {
		t.Fatal("refresh must rotate the cookie value")
	}

	newHash := hashToken(cookie2.Value)
	newRow := fs.sessions[newHash]
	oldHash := hashToken(cookie1.Value)
	oldRow := fs.sessions[oldHash]
	if oldRow.Session.RevokedAt == nil || oldRow.Session.ReplacedBy == nil || *oldRow.Session.ReplacedBy != newRow.Session.ID {
		t.Fatal("old session must be revoked and replaced_by the new session")
	}

	// Reuse of the rotated cookie1 → 401 and ALL sessions revoked.
	recReuse := doJSON(t, h, http.MethodPost, "/auth/refresh", map[string]string{}, cookie1, "")
	if recReuse.Code != 401 {
		t.Fatalf("reuse must 401, got %d", recReuse.Code)
	}
	if n := fs.ActiveSessionCount(u.ID); n != 0 {
		t.Fatalf("reuse must revoke every session, active=%d", n)
	}

	// The newer cookie2 is dead too.
	recDead := doJSON(t, h, http.MethodPost, "/auth/refresh", map[string]string{}, cookie2, "")
	if recDead.Code != 401 {
		t.Fatalf("post-reuse refresh must 401, got %d", recDead.Code)
	}
}

func TestRefreshWithoutToken(t *testing.T) {
	h, _, _ := newTestAPI(t)
	rec := doJSON(t, h, http.MethodPost, "/auth/refresh", nil, nil, "")
	if rec.Code != 401 {
		t.Fatalf("no-cookie refresh must 401, got %d", rec.Code)
	}
}

func TestLogoutRevokesAndClearsCookie(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	verifiedUser(t, h, fs, fm, "o@example.com")

	recLogin := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "o@example.com", "password": "Password123"}, nil, "")
	cookie := refreshCookieOf(t, recLogin)

	recOut := doJSON(t, h, http.MethodPost, "/auth/logout", nil, cookie, "")
	if recOut.Code != 200 {
		t.Fatalf("logout: %d %s", recOut.Code, recOut.Body.String())
	}
	// The login session is revoked (the verify auto-login session is untouched).
	if sess, err := fs.GetSessionByToken(context.Background(), hashToken(cookie.Value)); err != nil || sess.RevokedAt == nil {
		t.Fatalf("logout must revoke the cookie's session: err=%v rev-%v", err, sess.RevokedAt != nil)
	}
	cleared := false
	for _, c := range recOut.Result().Cookies() {
		if c.Name == RefreshCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("logout must clear the cookie (MaxAge < 0)")
	}

	recDead := doJSON(t, h, http.MethodPost, "/auth/refresh", map[string]string{}, cookie, "")
	if recDead.Code != 401 {
		t.Fatal("cookie must be dead after logout")
	}
}

func TestForgotPasswordGenericAndResetRevokesSessions(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	verifiedUser(t, h, fs, fm, "f@example.com")

	// Two logins → plus the verify auto-login session, three active in total.
	recLogin := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "f@example.com", "password": "Password123"}, nil, "")
	_ = refreshCookieOf(t, recLogin)
	recLogin2 := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "f@example.com", "password": "Password123"}, nil, "")
	_ = refreshCookieOf(t, recLogin2)
	u, _ := fs.GetByEmail(context.Background(), "f@example.com")
	if n := fs.ActiveSessionCount(u.ID); n != 3 {
		t.Fatalf("expected 3 sessions, got %d", n)
	}

	// Forgot: known and unknown emails get byte-identical responses.
	recKnown := doJSON(t, h, http.MethodPost, "/auth/forgot-password", map[string]string{"email": "f@example.com"}, nil, "")
	recUnknown := doJSON(t, h, http.MethodPost, "/auth/forgot-password", map[string]string{"email": "ghost@example.com"}, nil, "")
	if recKnown.Code != 200 || recUnknown.Code != 200 {
		t.Fatalf("forgot must be 200: %d/%d", recKnown.Code, recUnknown.Code)
	}
	if recKnown.Body.String() != recUnknown.Body.String() {
		t.Fatal("forgot responses must be identical for known/unknown emails")
	}

	resetToken := fm.lastToken()
	if resetToken == "" {
		t.Fatal("reset email with token expected")
	}

	recReset := doJSON(t, h, http.MethodPost, "/auth/reset-password", map[string]string{"token": resetToken, "new_password": "NewPassword123"}, nil, "")
	if recReset.Code != 200 {
		t.Fatalf("reset: %d %s", recReset.Code, recReset.Body.String())
	}
	if n := fs.ActiveSessionCount(u.ID); n != 0 {
		t.Fatalf("reset must revoke all sessions, active=%d", n)
	}

	// New password works, old one does not.
	if rec := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "f@example.com", "password": "Password123"}, nil, ""); rec.Code != 401 {
		t.Fatal("old password must stop working")
	}
	if rec := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "f@example.com", "password": "NewPassword123"}, nil, ""); rec.Code != 200 {
		t.Fatal("new password must work")
	}

	// Reset token is single-use.
	if rec := doJSON(t, h, http.MethodPost, "/auth/reset-password", map[string]string{"token": resetToken, "new_password": "AnotherPass123"}, nil, ""); rec.Code != 400 {
		t.Fatal("reset token reuse must 400")
	}
}

func TestSessionsListAndRevoke(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	verifiedUser(t, h, fs, fm, "s@example.com")

	rec1 := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "s@example.com", "password": "Password123"}, nil, "")
	cookie1 := refreshCookieOf(t, rec1)
	access1 := loginBody(t, rec1)["access_token"].(string)
	rec2 := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "s@example.com", "password": "Password123"}, nil, "")
	cookie2 := refreshCookieOf(t, rec2)

	// Rotate cookie2 so its SID is the current one for that access token.
	// (login access tokens carry their own session id — both are current for themselves)

	recList := doJSON(t, h, http.MethodGet, "/auth/sessions", nil, cookie1, access1)
	if recList.Code != 200 {
		t.Fatalf("list: %d %s", recList.Code, recList.Body.String())
	}
	var got struct {
		Sessions []SessionView `json:"sessions"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Sessions) != 3 { // verify auto-login + two logins
		t.Fatalf("expected 3 sessions, got %d", len(got.Sessions))
	}
	currentForFirst := 0
	for _, s := range got.Sessions {
		if s.Current {
			currentForFirst++
		}
	}
	if currentForFirst != 1 {
		t.Fatalf("exactly one session must be current, got %d", currentForFirst)
	}

	// Revoke cookie2's session (the other device) using the first device's access token.
	// Find it via the refresh token of the second login.
	sess2, err := fs.GetSessionByToken(context.Background(), hashToken(cookie2.Value))
	if err != nil {
		t.Fatal(err)
	}
	if recDel := doJSON(t, h, http.MethodDelete, "/auth/sessions/"+sess2.ID.String(), nil, cookie1, access1); recDel.Code != 204 {
		t.Fatalf("revoke other: %d", recDel.Code)
	}

	// Foreign / unknown session id → 404.
	if recForeign := doJSON(t, h, http.MethodDelete, "/auth/sessions/"+uuid.New().String(), nil, cookie1, access1); recForeign.Code != 404 {
		t.Fatalf("foreign session must 404, got %d", recForeign.Code)
	}

	// "Log out everywhere": everything except the current session.
	if recAll := doJSON(t, h, http.MethodDelete, "/auth/sessions", nil, cookie1, access1); recAll.Code != 204 {
		t.Fatalf("revoke others: %d", recAll.Code)
	}
	u, _ := fs.GetByEmail(context.Background(), "s@example.com")
	if n := fs.ActiveSessionCount(u.ID); n != 1 {
		t.Fatalf("only the current session must survive, active=%d", n)
	}

	// The other device's cookie is dead.
	if rec := doJSON(t, h, http.MethodPost, "/auth/refresh", map[string]string{}, cookie2, ""); rec.Code != 401 {
		t.Fatal("revoked device cookie must not refresh")
	}
}

func TestThrottleLoginPerEmail(t *testing.T) {
	h, _, _ := newTestAPI(t)
	body := map[string]string{"email": "thr@example.com", "password": "WrongPass123"}

	saw429 := false
	var retryAfter string
	for i := 0; i < 8; i++ {
		rec := doJSON(t, h, http.MethodPost, "/auth/login", body, nil, "")
		if rec.Code == 429 {
			saw429 = true
			retryAfter = rec.Header().Get("Retry-After")
			break
		}
		if rec.Code != 401 {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rec.Code)
		}
	}
	if !saw429 {
		t.Fatal("expected 429 after repeated failed logins")
	}
	if retryAfter == "" {
		t.Fatal("429 must carry Retry-After")
	}
}

func TestThrottleBlocksEvenCorrectPassword(t *testing.T) {
	h, fs, _ := newTestAPI(t)
	registerUser(t, h, fs, "block@example.com")

	ok := map[string]string{"email": "block@example.com", "password": "Password123"}
	wrong := map[string]string{"email": "block@example.com", "password": "WrongPass123"}
	// 5 wrong attempts exhaust the per-email bucket (limit 5 / 15 min).
	for i := 0; i < 5; i++ {
		doJSON(t, h, http.MethodPost, "/auth/login", wrong, nil, "")
	}
	// The 6th attempt is throttled even with the correct password.
	rec := doJSON(t, h, http.MethodPost, "/auth/login", ok, nil, "")
	if rec.Code != 429 {
		t.Fatalf("throttled correct login must 429, got %d", rec.Code)
	}
	// And no session was created.
	u, _ := fs.GetByEmail(context.Background(), "block@example.com")
	if n := fs.ActiveSessionCount(u.ID); n != 0 {
		t.Fatalf("throttled request must not create a session, active=%d", n)
	}
}

func TestMeIncludesEmailVerified(t *testing.T) {
	h, fs, fm := newTestAPI(t)
	verifiedUser(t, h, fs, fm, "m@example.com")

	recLogin := doJSON(t, h, http.MethodPost, "/auth/login", map[string]string{"email": "m@example.com", "password": "Password123"}, nil, "")
	access := loginBody(t, recLogin)["access_token"].(string)

	rec := doJSON(t, h, http.MethodGet, "/auth/me", nil, nil, access)
	if rec.Code != 200 {
		t.Fatalf("me: %d %s", rec.Code, rec.Body.String())
	}
	body := loginBody(t, rec)
	if body["email_verified"] != true {
		t.Fatalf("me must include email_verified=true, got %v", body["email_verified"])
	}
	if body["email"] != "m@example.com" {
		t.Fatalf("unexpected me body: %v", body)
	}
}
