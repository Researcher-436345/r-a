package identity

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	EmailVerifiedAt *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (u User) Verified() bool { return u.EmailVerifiedAt != nil }

// Session is a server-side refresh session (one row per logged-in device).
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	UserAgent  string
	IP         string
	CreatedAt  time.Time
	LastUsedAt time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *uuid.UUID
}

func (s Session) Active() bool { return s.RevokedAt == nil && time.Now().Before(s.ExpiresAt) }

type SessionView struct {
	ID         uuid.UUID `json:"id"`
	UserAgent  string    `json:"user_agent"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	Current    bool      `json:"current"`
}
