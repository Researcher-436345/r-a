package identity

import (
	"context"
	"net/http"
	"strings"

	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	Config config.Config
	DB     *pgxpool.Pool
}

func (a API) store() Store { return Store{DB: a.DB} }

type userKey struct{}

// UserID returns the authenticated user id from request context.
func UserID(r *http.Request) uuid.UUID {
	return r.Context().Value(userKey{}).(uuid.UUID)
}

// Middleware validates Bearer access tokens.
func (a API) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if v == r.Header.Get("Authorization") || v == "" {
			httpx.Error(w, 401, "Not authenticated")
			return
		}
		id, e := ParseToken(a.Config.JWTSecret, v, "access")
		if e != nil {
			httpx.Error(w, 401, "Could not validate credentials")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, id)))
	})
}

func (a API) Mount(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", a.register)
		r.Post("/login", a.login)
		r.Post("/refresh", a.refresh)
		r.With(a.Middleware).Get("/me", a.me)
	})
}

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

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
	hash, e := HashPassword(b.Password)
	if e != nil {
		httpx.Error(w, 500, "Failed to hash password")
		return
	}
	u, e := a.store().CreateUser(r.Context(), b.Email, hash)
	if e != nil {
		if strings.Contains(e.Error(), "duplicate") {
			httpx.Error(w, 409, "Email already registered")
			return
		}
		httpx.Error(w, 500, "Failed to create user")
		return
	}
	a.tokens(w, u.ID, 201)
}

func (a API) login(w http.ResponseWriter, r *http.Request) {
	var b credentials
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	u, e := a.store().GetByEmail(r.Context(), strings.ToLower(strings.TrimSpace(b.Email)))
	if e != nil || !CheckPassword(u.PasswordHash, b.Password) {
		httpx.Error(w, 401, "Incorrect email or password")
		return
	}
	a.tokens(w, u.ID, 200)
}

func (a API) refresh(w http.ResponseWriter, r *http.Request) {
	var b refreshReq
	if !httpx.DecodeJSON(w, r, &b) {
		return
	}
	id, e := ParseToken(a.Config.JWTSecret, b.RefreshToken, "refresh")
	if e != nil {
		httpx.Error(w, 401, "Invalid or expired refresh token")
		return
	}
	if _, e = a.store().GetByID(r.Context(), id); e != nil {
		httpx.Error(w, 401, "User not found")
		return
	}
	a.tokens(w, id, 200)
}

func (a API) tokens(w http.ResponseWriter, id uuid.UUID, status int) {
	t, e := IssueTokens(a.Config.JWTSecret, id, a.Config.AccessTokenTTL, a.Config.RefreshTokenTTL)
	if e != nil {
		httpx.Error(w, 500, "Failed to issue token")
		return
	}
	httpx.JSON(w, status, t)
}

func (a API) me(w http.ResponseWriter, r *http.Request) {
	u, err := a.store().GetByID(r.Context(), UserID(r))
	if err != nil {
		httpx.Error(w, 401, "User not found")
		return
	}
	httpx.JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "created_at": u.CreatedAt})
}
