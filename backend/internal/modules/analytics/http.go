package analytics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/throttle"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	DB      *pgxpool.Pool
	Limiter *throttle.Limiter
}

func (a API) Mount(r chi.Router) { r.Post("/analytics/events", a.events) }
func (a API) events(w http.ResponseWriter, r *http.Request) {
	user := identity.UserID(r)
	if user == uuid.Nil {
		httpx.Error(w, 401, "Authentication required")
		return
	}
	if allowed, after := a.Limiter.Hit(r.Context(), throttle.Rule{Name: "analytics:user", Limit: 60, Window: time.Minute}, user.String()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(after.Seconds())+1))
		httpx.Error(w, 429, "Analytics rate limit")
		return
	}
	var body struct {
		Events []Event `json:"events"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if len(body.Events) == 0 || len(body.Events) > 20 {
		httpx.Error(w, 400, "Expected 1 to 20 events")
		return
	}
	for _, e := range body.Events {
		if !ValidateClient(e) {
			httpx.Error(w, 400, "Invalid analytics event")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	for _, e := range body.Events {
		if e.ClientAt != nil {
			now := time.Now()
			if e.ClientAt.Before(now.Add(-24*time.Hour)) || e.ClientAt.After(now.Add(2*time.Minute)) {
				continue
			}
			e.At = *e.ClientAt
			if e.At.After(now) {
				e.At = now
			}
		}
		if e.ProjectID != nil {
			root, err := rootProject(ctx, a.DB, user, *e.ProjectID)
			if err != nil {
				httpx.Error(w, 503, "Analytics temporarily unavailable")
				return
			}
			if root == nil {
				continue
			}
			e.ProjectID = root
		} else if e.PaperID != nil {
			project, err := projectForPaper(ctx, a.DB, user, *e.PaperID)
			if err != nil {
				httpx.Error(w, 503, "Analytics temporarily unavailable")
				return
			}
			e.ProjectID = project
		}
		// A client may only attribute a conversion to its own completed search.
		if e.SourceActionID != nil {
			var allowed bool
			err := a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM analytics_events WHERE user_id=$1 AND action_id=$2 AND event_type='action_finished' AND outcome='success' AND feature IN ('search','deep_research'))`, user, *e.SourceActionID).Scan(&allowed)
			if err != nil || !allowed {
				e.SourceActionID = nil
			}
		}
		if err := insert(ctx, a.DB, user, e, "client"); err != nil {
			httpx.Error(w, 503, "Analytics temporarily unavailable")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
