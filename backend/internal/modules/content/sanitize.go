package content

import (
	"strings"
	"unicode/utf8"
)

// SanitizeText makes parser/TeX output storable in Postgres text columns:
// NUL bytes are rejected there (SQLSTATE 22021) and so is invalid UTF-8, and
// either one used to fail the whole SaveReady transaction on every retry.
func SanitizeText(s string) string {
	if utf8.ValidString(s) && strings.IndexByte(s, 0) < 0 {
		return s
	}
	return strings.ReplaceAll(strings.ToValidUTF8(s, "�"), "\x00", "")
}

// sanitizeChunks returns cleaned copies so callers' chunk slices and Section
// pointers are left untouched.
func sanitizeChunks(chunks []Chunk) []Chunk {
	out := make([]Chunk, len(chunks))
	for i, c := range chunks {
		c.Text = SanitizeText(c.Text)
		if c.Section != nil {
			section := SanitizeText(*c.Section)
			c.Section = &section
		}
		out[i] = c
	}
	return out
}
