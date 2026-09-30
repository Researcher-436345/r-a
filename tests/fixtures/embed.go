// Package fixtures holds deterministic test data shared by the fakes and the
// integration tests: small PDFs with known text on known pages.
package fixtures

import (
	"embed"
	"fmt"
)

//go:embed pdf/*
var FS embed.FS

// PDF returns the bytes of a fixture from ./pdf (e.g. "paper-5p.pdf").
func PDF(name string) []byte {
	data, err := FS.ReadFile("pdf/" + name)
	if err != nil {
		panic(fmt.Sprintf("fixtures: %s: %v", name, err))
	}
	return data
}
