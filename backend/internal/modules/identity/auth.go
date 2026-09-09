package identity

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Claims struct {
	Type string `json:"type"`
	// SID is the bound auth_sessions.id (access tokens only, omitempty for
	// compatibility with tokens issued before server-side sessions).
	SID string `json:"sid,omitempty"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// IssueAccessToken signs an access JWT bound to a server-side session.
func IssueAccessToken(secret string, userID, sessionID uuid.UUID, ttl time.Duration) (string, error) {
	return sign(secret, userID, sessionID, "access", ttl)
}

// ParseToken returns the subject user id (also used by the gateway).
func ParseToken(secret, token, expectType string) (uuid.UUID, error) {
	userID, _, err := parseTokenFull(secret, token, expectType)
	return userID, err
}

// ParseAccessToken returns the user id and the bound session id (Nil if absent).
func ParseAccessToken(secret, token string) (uuid.UUID, uuid.UUID, error) {
	return parseTokenFull(secret, token, "access")
}

func parseTokenFull(secret, token, expectType string) (uuid.UUID, uuid.UUID, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return uuid.Nil, uuid.Nil, errors.New("invalid token")
	}
	if claims.Type != expectType {
		return uuid.Nil, uuid.Nil, errors.New("wrong token type")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	var sid uuid.UUID
	if claims.SID != "" {
		if sid, err = uuid.Parse(claims.SID); err != nil {
			return uuid.Nil, uuid.Nil, err
		}
	}
	return id, sid, nil
}

func sign(secret string, userID, sessionID uuid.UUID, typ string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		Type: typ,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	if sessionID != uuid.Nil {
		claims.SID = sessionID.String()
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}
