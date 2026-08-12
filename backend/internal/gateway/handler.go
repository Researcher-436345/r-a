package gateway

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/centraluniversity/researcher/internal/modules/identity"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/httpx"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

// Handler is the public edge: CORS, JWT, reverse-proxy to domain services.
func Handler(cfg config.Config) http.Handler {
	identityProxy := mustProxy(cfg.IdentityURL)
	catalogProxy := mustProxy(cfg.CatalogURL)
	libraryProxy := mustProxy(cfg.LibraryURL)
	annotationsProxy := mustProxy(cfg.AnnotationsURL)
	assistantProxy := mustProxy(cfg.AssistantURL)
	feedProxy := mustProxy(cfg.FeedURL)
	searchProxy := mustProxy(cfg.SearchAPIURL)

	assistantProxy.FlushInterval = -1
	assistantProxy.Transport = &http.Transport{
		ResponseHeaderTimeout: 10 * time.Minute,
		IdleConnTimeout:       90 * time.Second,
	}
	searchProxy.FlushInterval = -1
	searchProxy.Transport = &http.Transport{
		ResponseHeaderTimeout: 10 * time.Minute,
		IdleConnTimeout:       90 * time.Second,
	}

	mux := http.NewServeMux()
	var root http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/health":
			httpx.JSON(w, 200, map[string]string{"status": "ok", "service": "gateway"})
			return
		case path == "/" || path == "":
			httpx.JSON(w, 200, map[string]string{"service": "gateway", "health": "/health"})
			return
		case strings.HasPrefix(path, "/auth"):
			identityProxy.ServeHTTP(w, r)
			return
		}

		v := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if v == r.Header.Get("Authorization") || v == "" {
			httpx.Error(w, 401, "Not authenticated")
			return
		}
		id, err := identity.ParseToken(cfg.JWTSecret, v, "access")
		if err != nil {
			httpx.Error(w, 401, "Could not validate credentials")
			return
		}
		r.Header.Set(identity.UserIDHeader, id.String())

		switch {
		case strings.HasPrefix(path, "/annotations") || paperSubresource(path, "annotations"):
			annotationsProxy.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/assistant") ||
			paperSubresource(path, "chat") ||
			paperSubresource(path, "explain") ||
			paperSubresource(path, "translate"):
			assistantProxy.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/library"):
			libraryProxy.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/feed"):
			feedProxy.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/search"):
			searchProxy.ServeHTTP(w, r)
		case strings.HasPrefix(path, "/papers"):
			catalogProxy.ServeHTTP(w, r)
		default:
			httpx.Error(w, 404, "Not found")
		}
	})

	root = cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept"},
		AllowCredentials: true,
		MaxAge:           300,
	})(root)

	mux.Handle("/", root)
	return mux
}

// paperSubresource reports paths like /papers/{uuid}/chat or /papers/{uuid}/chat/messages.
func paperSubresource(path, name string) bool {
	const prefix = "/papers/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return false
	}
	if _, err := uuid.Parse(parts[0]); err != nil {
		return false
	}
	return parts[1] == name
}

func mustProxy(raw string) *httputil.ReverseProxy {
	u, err := url.Parse(raw)
	if err != nil {
		panic("gateway: bad upstream URL " + raw + ": " + err.Error())
	}
	p := httputil.NewSingleHostReverseProxy(u)
	orig := p.Director
	p.Director = func(r *http.Request) {
		orig(r)
		// Preserve path/query; SingleHost already sets scheme/host.
		r.Host = u.Host
	}
	return p
}
