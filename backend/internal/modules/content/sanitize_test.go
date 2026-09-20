package content

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain ascii", in: "Attention is all you need", want: "Attention is all you need"},
		{name: "cyrillic and symbols untouched", in: "Обзор методов — α≤β, 日本語, 🙂\n\ttab", want: "Обзор методов — α≤β, 日本語, 🙂\n\ttab"},
		{name: "nul removed", in: "a\x00b\x00", want: "ab"},
		{name: "only nul", in: "\x00\x00", want: ""},
		{name: "invalid byte replaced", in: "ok\xffok", want: "ok�ok"},
		{name: "truncated cyrillic rune replaced", in: "При\xd0", want: "При�"},
		{name: "nul and invalid together", in: "x\x00\xc3\x28y", want: "x�(y"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeText(tc.in)
			if got != tc.want {
				t.Fatalf("SanitizeText(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !utf8.ValidString(got) || strings.IndexByte(got, 0) >= 0 {
				t.Fatalf("SanitizeText(%q) = %q is not storable", tc.in, got)
			}
		})
	}
}

func TestSanitizeChunksCopiesAndCleans(t *testing.T) {
	section := "Intro\x00duction"
	in := []Chunk{
		{ChunkIndex: 0, Text: "body\x00\xff", Section: &section},
		{ChunkIndex: 1, Text: "Текст", Section: nil},
	}
	out := sanitizeChunks(in)
	if len(out) != 2 {
		t.Fatalf("got %d chunks, want 2", len(out))
	}
	if out[0].Text != "body�" || out[0].Section == nil || *out[0].Section != "Introduction" {
		t.Fatalf("chunk 0 not sanitized: text=%q section=%v", out[0].Text, out[0].Section)
	}
	if out[1].Text != "Текст" || out[1].Section != nil {
		t.Fatalf("chunk 1 changed: text=%q section=%v", out[1].Text, out[1].Section)
	}
	if in[0].Text != "body\x00\xff" || section != "Intro\x00duction" {
		t.Fatal("sanitizeChunks must not mutate the caller's chunks")
	}
}
