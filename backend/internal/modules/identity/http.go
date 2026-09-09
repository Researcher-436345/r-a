package identity

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/mailer"
	"github.com/centraluniversity/researcher/internal/platform/throttle"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// MailSender decouples handlers from the SMTP mailer (tests use fakes).
type MailSender interface {
	Send(mailer.Message) error
}

type API struct {
	Config config.Config
	DB     *pgxpool.Pool
	Redis  redis.UniversalClient // nil disables throttling (tests)
	Mail   MailSender            // nil disables emails (tests)
	Store  SessionStore          // optional override (tests); falls back to DB-backed Store
}

func (a API) store() SessionStore {
	if a.Store != nil {
		return a.Store
	}
	return Store{DB: a.DB}
}

type userKey struct{}
type sessionKey struct{}

// UserIDHeader is set by the gateway after validating the access token.
const UserIDHeader = "X-User-Id"

// Fixed-window throttle rules (platform/throttle, Redis-backed).
var (
	ruleLoginIP     = throttle.Rule{Name: "login:ip", Limit: 10, Window: time.Minute}
	ruleLoginEmail  = throttle.Rule{Name: "login:email", Limit: 5, Window: 15 * time.Minute}
	ruleAuthIP      = throttle.Rule{Name: "auth:ip", Limit: 5, Window: 15 * time.Minute}    // register / resend / forgot
	ruleAuthEmail   = throttle.Rule{Name: "auth:email", Limit: 5, Window: 15 * time.Minute} // register / resend / forgot
	ruleSensitiveIP = throttle.Rule{Name: "sensitive:ip", Limit: 10, Window: time.Hour}     // verify / reset
)

// UserID returns the authenticated user id from request context.
func UserID(r *http.Request) uuid.UUID {
	return r.Context().Value(userKey{}).(uuid.UUID)
}

// SessionID returns the session id bound to the access token (Nil if absent).
func SessionID(r *http.Request) uuid.UUID {
	v, _ := r.Context().Value(sessionKey{}).(uuid.UUID)
	return v
}

// WithUserID stores user id in context (tests / gateway adapters).
func WithUserID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, userKey{}, id)
}

// WithSessionID stores the session id bound to the access token in context.
func WithSessionID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, sessionKey{}, id)
}

// Middleware validates Bearer access tokens.
func (a API) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if v == r.Header.Get("Authorization") || v == "" {
			httpx.Error(w, 401, "Not authenticated")
			return
		}
		id, sid, err := ParseAccessToken(a.Config.JWTSecret, v)
		if err != nil {
			httpx.Error(w, 401, "Could not validate credentials")
			return
		}
		ctx := WithUserID(r.Context(), id)
		if sid != uuid.Nil {
			ctx = WithSessionID(ctx, sid)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MiddlewareFromGateway trusts X-User-Id set by the API gateway (no JWT re-check).
func MiddlewareFromGateway(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := strings.TrimSpace(r.Header.Get(UserIDHeader))
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.Error(w, 401, "Not authenticated")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), id)))
	})
}

func (a API) Mount(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", a.register)
		r.Post("/verify-email", a.verifyEmail)
		r.Post("/login", a.login)
		r.Post("/refresh", a.refresh)
		r.Post("/logout", a.logout)
		r.Post("/forgot-password", a.forgotPassword)
		r.Post("/reset-password", a.resetPassword)
		r.Post("/resend-verification", a.resendVerification)
		r.With(a.Middleware).Get("/me", a.me)
		r.With(a.Middleware).Get("/sessions", a.listSessions)
		r.With(a.Middleware).Delete("/sessions/{sessionID}", a.revokeSession)
		r.With(a.Middleware).Delete("/sessions", a.revokeOtherSessions)
	})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type tokenReq struct {
	Token string `json:"token"`
}
type emailReq struct {
	Email string `json:"email"`
}
type resetReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}
type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

// POST /auth/register — always a generic response against user enumeration.
// Creates an unverified user + verify email; existing verified users get the
// same response with no email sent.
func (a API) register(w http.ResponseWriter, r *http.Request) {
	var b credentials
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if b.Email == "" || len(b.Password) < 8 {
		httpx.Error(w, 400, "Invalid email or password")
		return
	}
	if !a.enforce(w, r, ruleAuthIP, clientIP(r), ruleAuthEmail, b.Email) {
		return
	}
	hash, e := HashPassword(b.Password)
	if e != nil {
		httpx.Error(w, 500, "Failed to hash password")
		return
	}
	u, err := a.store().GetByEmail(r.Context(), b.Email)
	switch {
	case err == nil:
		if !u.Verified() {
			a.sendVerification(r, u)
		}
	case errors.Is(err, pgx.ErrNoRows):
		u, err = a.store().CreateUser(r.Context(), b.Email, hash)
		if err != nil {
			httpx.Error(w, 500, "Failed to create user")
			return
		}
		a.sendVerification(r, u)
	default:
		httpx.Error(w, 500, "Failed to create user")
		return
	}
	if !a.Config.EmailVerificationEnabled {
		httpx.JSON(w, 200, map[string]any{"message": "you can sign in", "email_verification_required": false})
		return
	}
	httpx.JSON(w, 200, map[string]string{"message": "check your email"})
}

// POST /auth/verify-email — one-time token → mark verified → auto-login.
func (a API) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var b tokenReq
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Token) == "" {
		httpx.Error(w, 400, "Invalid or expired token")
		return
	}
	if !a.enforce(w, r, ruleSensitiveIP, clientIP(r)) {
		return
	}
	userID, err := a.store().ConsumeAuthToken(r.Context(), "verify_email", hashToken(b.Token))
	if err != nil {
		httpx.Error(w, 400, "Invalid or expired token")
		return
	}
	if err := a.store().MarkVerified(r.Context(), userID); err != nil {
		httpx.Error(w, 500, "Failed to verify email")
		return
	}
	a.startSession(w, r, userID, 200)
}

// POST /auth/login — generic 401, 403 email_not_verified (+ auto-resend).
func (a API) login(w http.ResponseWriter, r *http.Request) {
	var b credentials
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if !a.enforce(w, r, ruleLoginIP, clientIP(r), ruleLoginEmail, b.Email) {
		return
	}
	u, e := a.store().GetByEmail(r.Context(), b.Email)
	if e != nil || !CheckPassword(u.PasswordHash, b.Password) {
		slog.Warn("auth failure", "endpoint", "login", "email", b.Email, "ip", clientIP(r), "reason", "invalid_credentials")
		httpx.Error(w, 401, "Incorrect email or password")
		return
	}
	if a.Config.EmailVerificationEnabled && !u.Verified() {
		a.sendVerification(r, u)
		httpx.JSON(w, 403, map[string]any{
			"detail": "Email is not verified. We sent you a new confirmation link.",
			"code":   "email_not_verified",
		})
		return
	}
	a.startSession(w, r, u.ID, 200)
}

// POST /auth/refresh — cookie-first, DB-backed rotation with reuse detection.
func (a API) refresh(w http.ResponseWriter, r *http.Request) {
	raw := refreshCookie(r)
	if raw == "" {
		// Fallback for non-browser clients: refresh_token in JSON body.
		var b refreshReq
		if !httpx.DecodeJSONOptional(w, r, &b) {
			return
		}
		raw = strings.TrimSpace(b.RefreshToken)
	}
	if raw == "" {
		httpx.Error(w, 401, "Invalid or expired refresh token")
		return
	}
	sess, err := a.store().GetSessionByToken(r.Context(), hashToken(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.Error(w, 401, "Invalid or expired refresh token")
		return
	}
	if err != nil {
		httpx.Error(w, 500, "Failed to refresh session")
		return
	}
	if !sess.Active() {
		if sess.RevokedAt != nil {
			// Reuse detection: an already-rotated/revoked token was replayed —
			// assume theft and kill every session of the user.
			slog.Warn("refresh token reuse detected, revoking all user sessions",
				"user_id", sess.UserID, "ip", clientIP(r), "session_id", sess.ID)
			_ = a.store().RevokeAllSessions(r.Context(), sess.UserID, uuid.Nil)
		}
		clearRefreshCookie(w, cookieConfig{Secure: a.Config.CookieSecure})
		httpx.Error(w, 401, "Invalid or expired refresh token")
		return
	}
	newRaw, newHash, err := newToken()
	if err != nil {
		httpx.Error(w, 500, "Failed to refresh session")
		return
	}
	newSess, err := a.store().RotateSession(r.Context(), sess, newHash, time.Now().Add(a.Config.RefreshTokenTTL))
	if err != nil {
		httpx.Error(w, 500, "Failed to refresh session")
		return
	}
	access, err := IssueAccessToken(a.Config.JWTSecret, newSess.UserID, newSess.ID, a.Config.AccessTokenTTL)
	if err != nil {
		httpx.Error(w, 500, "Failed to issue token")
		return
	}
	setRefreshCookie(w, cookieConfig{Secure: a.Config.CookieSecure}, newRaw, a.Config.RefreshTokenTTL)
	httpx.JSON(w, 200, map[string]any{"access_token": access, "token_type": "bearer"})
}

// POST /auth/logout — revoke the cookie's session and clear the cookie.
func (a API) logout(w http.ResponseWriter, r *http.Request) {
	if raw := refreshCookie(r); raw != "" {
		if sess, err := a.store().GetSessionByToken(r.Context(), hashToken(raw)); err == nil && sess.Active() {
			_, _ = a.store().RevokeSession(r.Context(), sess.UserID, sess.ID)
		}
	}
	clearRefreshCookie(w, cookieConfig{Secure: a.Config.CookieSecure})
	httpx.JSON(w, 200, map[string]string{"message": "logged out"})
}

// POST /auth/forgot-password — always generic 200.
func (a API) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var b emailReq
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if b.Email == "" {
		httpx.Error(w, 400, "Invalid email")
		return
	}
	if !a.enforce(w, r, ruleAuthIP, clientIP(r), ruleAuthEmail, b.Email) {
		return
	}
	if u, err := a.store().GetByEmail(r.Context(), b.Email); err == nil {
		a.sendPasswordReset(r, u)
	}
	httpx.JSON(w, 200, map[string]string{"message": "check your email"})
}

// POST /auth/reset-password — consume token, set new hash, revoke all sessions.
func (a API) resetPassword(w http.ResponseWriter, r *http.Request) {
	var b resetReq
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	if strings.TrimSpace(b.Token) == "" || len(b.NewPassword) < 8 {
		httpx.Error(w, 400, "Invalid token or password")
		return
	}
	if !a.enforce(w, r, ruleSensitiveIP, clientIP(r)) {
		return
	}
	userID, err := a.store().ConsumeAuthToken(r.Context(), "reset_password", hashToken(b.Token))
	if err != nil {
		httpx.Error(w, 400, "Invalid or expired token")
		return
	}
	hash, err := HashPassword(b.NewPassword)
	if err != nil {
		httpx.Error(w, 500, "Failed to hash password")
		return
	}
	if err := a.store().UpdatePassword(r.Context(), userID, hash); err != nil {
		httpx.Error(w, 500, "Failed to update password")
		return
	}
	_ = a.store().RevokeAllSessions(r.Context(), userID, uuid.Nil)
	httpx.JSON(w, 200, map[string]string{"message": "password updated"})
}

// POST /auth/resend-verification — generic 200; shares the strict register bucket.
func (a API) resendVerification(w http.ResponseWriter, r *http.Request) {
	var b emailReq
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	b.Email = strings.ToLower(strings.TrimSpace(b.Email))
	if b.Email == "" {
		httpx.Error(w, 400, "Invalid email")
		return
	}
	if !a.enforce(w, r, ruleAuthIP, clientIP(r), ruleAuthEmail, b.Email) {
		return
	}
	if u, err := a.store().GetByEmail(r.Context(), b.Email); err == nil && !u.Verified() {
		a.sendVerification(r, u)
	}
	httpx.JSON(w, 200, map[string]string{"message": "check your email"})
}

// GET /auth/me
func (a API) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.store().GetByID(r.Context(), UserID(r))
	if err != nil {
		httpx.Error(w, 401, "User not found")
		return
	}
	httpx.JSON(w, 200, map[string]any{
		"id":             u.ID,
		"email":          u.Email,
		"created_at":     u.CreatedAt,
		"email_verified": u.Verified(),
	})
}

// GET /auth/sessions — active sessions; the caller's own session is flagged.
func (a API) listSessions(w http.ResponseWriter, r *http.Request) {
	current := SessionID(r)
	sessions, err := a.store().ListActiveSessions(r.Context(), UserID(r))
	if err != nil {
		httpx.Error(w, 500, "Failed to list sessions")
		return
	}
	views := make([]SessionView, 0, len(sessions))
	for _, s := range sessions {
		views = append(views, SessionView{
			ID:         s.ID,
			UserAgent:  s.UserAgent,
			IP:         s.IP,
			CreatedAt:  s.CreatedAt,
			LastUsedAt: s.LastUsedAt,
			Current:    current != uuid.Nil && s.ID == current,
		})
	}
	httpx.JSON(w, 200, map[string]any{"sessions": views})
}

// DELETE /auth/sessions/{id} — revoke one session (foreign/unknown → 404).
func (a API) revokeSession(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "sessionID"))
	if err != nil {
		httpx.Error(w, 404, "Not found")
		return
	}
	revoked, err := a.store().RevokeSession(r.Context(), UserID(r), id)
	if err != nil {
		httpx.Error(w, 500, "Failed to revoke session")
		return
	}
	if !revoked {
		httpx.Error(w, 404, "Not found")
		return
	}
	if id == SessionID(r) {
		clearRefreshCookie(w, cookieConfig{Secure: a.Config.CookieSecure})
	}
	httpx.NoContent(w)
}

// DELETE /auth/sessions — "log out everywhere": revoke all but the current one.
func (a API) revokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	if err := a.store().RevokeAllSessions(r.Context(), UserID(r), SessionID(r)); err != nil {
		httpx.Error(w, 500, "Failed to revoke sessions")
		return
	}
	httpx.NoContent(w)
}

// --- session plumbing ---------------------------------------------------------

// startSession creates a DB-backed refresh session, sets the cookie and
// returns the access token.
func (a API) startSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID, status int) {
	raw, hash, err := newToken()
	if err != nil {
		httpx.Error(w, 500, "Failed to issue token")
		return
	}
	sess, err := a.store().CreateSession(r.Context(), userID, hash, r.UserAgent(), clientIP(r), time.Now().Add(a.Config.RefreshTokenTTL))
	if err != nil {
		httpx.Error(w, 500, "Failed to create session")
		return
	}
	access, err := IssueAccessToken(a.Config.JWTSecret, userID, sess.ID, a.Config.AccessTokenTTL)
	if err != nil {
		httpx.Error(w, 500, "Failed to issue token")
		return
	}
	setRefreshCookie(w, cookieConfig{Secure: a.Config.CookieSecure}, raw, a.Config.RefreshTokenTTL)
	httpx.JSON(w, status, map[string]any{"access_token": access, "token_type": "bearer"})
}

func refreshCookie(r *http.Request) string {
	c, err := r.Cookie(RefreshCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// --- one-time emails ------------------------------------------------------------

func (a API) sendVerification(r *http.Request, u User) {
	if !a.Config.EmailVerificationEnabled {
		return
	}
	raw, hash, err := newToken()
	if err != nil {
		slog.Error("failed to generate verify token", "error", err)
		return
	}
	if err := a.store().CreateAuthToken(r.Context(), u.ID, "verify_email", hash, verifyEmailTTL); err != nil {
		slog.Error("failed to store verify token", "email", u.Email, "error", err)
		return
	}
	if a.Mail != nil {
		a.sendVerifyEmail(u.Email, raw)
	}
}

func (a API) sendPasswordReset(r *http.Request, u User) {
	raw, hash, err := newToken()
	if err != nil {
		slog.Error("failed to generate reset token", "error", err)
		return
	}
	if err := a.store().CreateAuthToken(r.Context(), u.ID, "reset_password", hash, resetPasswordTTL); err != nil {
		slog.Error("failed to store reset token", "email", u.Email, "error", err)
		return
	}
	if a.Mail != nil {
		a.sendResetPasswordEmail(u.Email, raw)
	}
}

// --- throttling / request metadata ----------------------------------------------

// enforce hits rate limit rules; on denial it writes 429 + Retry-After.
func (a API) enforce(w http.ResponseWriter, r *http.Request, rules ...any) bool {
	subjects := make(map[throttle.Rule]string, len(rules)/2)
	for i := 0; i+1 < len(rules); i += 2 {
		rule := rules[i].(throttle.Rule)
		subject := rules[i+1].(string)
		if subject == "" {
			continue
		}
		subjects[rule] = subject
	}
	allowed, after := (&throttle.Limiter{Redis: a.Redis}).HitAll(r.Context(), subjects)
	if allowed {
		return true
	}
	slog.Warn("auth failure", "endpoint", r.URL.Path, "ip", clientIP(r), "reason", "rate_limited")
	if after > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(after.Seconds()))))
	}
	httpx.Error(w, 429, "Too many requests")
	return false
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
