// Package throttle implements fixed-window rate limiting on top of Redis.
// Redis being down never blocks auth: limits fail open (and log).
package throttle

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type Rule struct {
	// Name is the logical limit name, e.g. "login:ip" — part of the Redis key.
	Name   string
	Limit  int
	Window time.Duration
}

func (r Rule) key(subject string) string {
	return fmt.Sprintf("ratelimit:%s:%s", r.Name, subject)
}

type Limiter struct {
	Redis redis.UniversalClient
}

// Hit records an attempt and reports whether it is allowed.
// retryAfter is meaningful only when allowed == false.
func (l *Limiter) Hit(ctx context.Context, rule Rule, subject string) (allowed bool, retryAfter time.Duration) {
	if l == nil || l.Redis == nil {
		return true, 0
	}
	key := rule.key(subject)
	n, err := l.Redis.Incr(ctx, key).Result()
	if err != nil {
		slog.Warn("throttle: redis incr failed, allowing request", "rule", rule.Name, "error", err)
		return true, 0
	}
	if n == 1 {
		if err := l.Redis.Expire(ctx, key, rule.Window).Err(); err != nil {
			slog.Warn("throttle: redis expire failed", "rule", rule.Name, "error", err)
		}
	}
	if n <= int64(rule.Limit) {
		return true, 0
	}
	ttl, err := l.Redis.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		ttl = rule.Window
	}
	return false, ttl
}

// HitAll enforces several rules at once (e.g. per-IP and per-email for login)
// and returns the longest retry-after among the denied rules.
func (l *Limiter) HitAll(ctx context.Context, subjectOf map[Rule]string) (allowed bool, retryAfter time.Duration) {
	allowed = true
	longest := time.Duration(0)
	for rule, subject := range subjectOf {
		ok, after := l.Hit(ctx, rule, subject)
		if !ok {
			allowed = false
			if after > longest {
				longest = after
			}
		}
	}
	return allowed, longest
}
