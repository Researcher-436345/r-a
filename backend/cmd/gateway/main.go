package main

import (
	"log"
	"net/http"
	"time"

	"github.com/centraluniversity/researcher/internal/gateway"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/observability"
)

func main() {
	observability.Start("gateway")
	cfg := config.Load()
	log.Printf("gateway listening on %s", cfg.HTTPAddr)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           observability.HTTP("gateway", gateway.Handler(cfg)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}
