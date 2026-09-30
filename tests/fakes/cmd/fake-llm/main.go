// fake-llm is an OpenAI-compatible provider double; see package llm.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/centraluniversity/researcher/tests/fakes/llm"
)

func main() {
	addr := os.Getenv("FAKE_LLM_ADDR")
	if addr == "" {
		addr = ":8094"
	}
	log.Printf("fake-llm listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, llm.New().Handler()))
}
