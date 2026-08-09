package main

import (
	"log"
	"net/http"

	"github.com/centraluniversity/researcher/internal/modules/translation"
	"github.com/centraluniversity/researcher/internal/platform/config"
)

func main() {
	cfg := config.Load()
	log.Printf("translator listening on %s", cfg.TranslationHTTPAddr)
	log.Fatal(http.ListenAndServe(cfg.TranslationHTTPAddr, translation.Handler(cfg)))
}
