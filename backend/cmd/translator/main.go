package main

import (
	"log"
	"net/http"

	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
	"github.com/centraluniversity/researcher/internal/platform/observability"
)

func main() {
	observability.Start("translator")
	cfg := config.Load()
	log.Printf("translator listening on %s", cfg.TranslationHTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.TranslationHTTPAddr, observability.HTTP("translator", translation.Handler(cfg))))
}
