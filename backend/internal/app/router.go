package app

import (
	"net/http"
	"strconv"

	"github.com/centraluniversity/researcher/internal/modules/annotations"
	"github.com/centraluniversity/researcher/internal/modules/assistant"
	"github.com/centraluniversity/researcher/internal/modules/catalog"
	"github.com/centraluniversity/researcher/internal/modules/feed"
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/modules/library"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/db"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/centraluniversity/researcher/internal/platform/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Deps is the composition root wiring for the modular monolith.
type Deps struct {
	Config  config.Config
	DB      *pgxpool.Pool
	Storage *storage.Client
	Queue   *asynq.Client
	Redis   *redis.Client
}

func Router(d Deps) http.Handler {
	libStore := library.Store{DB: d.DB, Catalog: catalog.Store{DB: d.DB}}
	catalogAPI := catalog.API{
		DB:         d.DB,
		Storage:    d.Storage,
		Queue:      d.Queue,
		Membership: libStore,
	}
	identityAPI := identity.API{Config: d.Config, DB: d.DB}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, 200, map[string]string{
			"service":      d.Config.AppName,
			"docs":         "/docs",
			"health":       "/health",
			"db_reachable": strconv.FormatBool(db.Check(r.Context(), d.DB)),
		})
	})
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		ok := db.Check(r.Context(), d.DB)
		m := "skipped"
		if ok {
			if db.MigrationsOK(r.Context(), d.DB) {
				m = "ok"
			} else {
				m = "error"
			}
		}
		status := "degraded"
		if ok && m == "ok" {
			status = "ok"
		}
		httpx.JSON(w, 200, map[string]string{
			"status":     status,
			"db":         map[bool]string{true: "ok", false: "error"}[ok],
			"migrations": m,
		})
	})

	identityAPI.Mount(r)

	r.Group(func(r chi.Router) {
		r.Use(identityAPI.Middleware)
		catalogAPI.Mount(r)
		library.API{Store: libStore}.Mount(r)
		annotations.API{Store: annotations.Store{DB: d.DB}, Papers: catalogAPI}.Mount(r)
		assistant.API{Config: d.Config, DB: d.DB, Papers: catalogAPI}.Mount(r)
		feed.API{Service: feed.Service{Redis: d.Redis}}.Mount(r)
	})

	return r
}
