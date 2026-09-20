package content

import (
	"strings"
	"testing"
)

func TestCleanTeXKeepsMathAndSections(t *testing.T) {
	src := `
\documentclass{article}
\begin{document}
\title{Hello World}
\section{Intro}
Let $x=1$ and $$y=2$$.
\cite{foo}
\includegraphics[width=1]{fig.png}
% comment
\end{document}
`
	out := CleanTeX(src)
	if !contains(out, "$x=1$") {
		t.Fatalf("math missing: %q", out)
	}
	if !contains(out, "## Intro") && !contains(out, "Intro") {
		t.Fatalf("section missing: %q", out)
	}
	if contains(out, "includegraphics") || contains(out, "cite{foo}") {
		t.Fatalf("junk left: %q", out)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// arXiv-исходники почти всегда разбиты на sections/*.tex: без раскрытия
// \input главный файл — это оглавление из имён файлов, а не статья.
func TestPickAndJoinTeXExpandsInputs(t *testing.T) {
	files := map[string]string{
		"arxiv.tex":           "\\documentclass{article}\n\\begin{document}\n\\input{sections/intro}\n\\include{sections/method.tex}\n\\input{sections/missing}\n\\end{document}",
		"sections/intro.tex":  "\\section{Introduction}\nIntro body. \\input{../sections/method}",
		"sections/method.tex": "\\section{Method}\nMethod body.",
		"sections/unused.tex": "\\section{Unused}\nShould not appear.",
	}
	got := pickAndJoinTeX(files)
	for _, want := range []string{"Intro body.", "Method body."} {
		if !strings.Contains(got, want) {
			t.Errorf("expanded source must contain %q, got:\n%s", want, got)
		}
	}
	if strings.Count(got, "Method body.") != 1 {
		t.Errorf("a file included twice must be expanded once, got:\n%s", got)
	}
	if strings.Contains(got, "Should not appear") {
		t.Error("files not reachable from the root must not be joined in")
	}
	if strings.Contains(got, "sections/missing") || strings.Contains(got, "\\input") {
		t.Errorf("unresolved inputs must be dropped, got:\n%s", got)
	}
}

func TestExpandTeXInputsSurvivesCycles(t *testing.T) {
	files := map[string]string{
		"main.tex": "\\begin{document}\nA \\input{b}\n\\end{document}",
		"b.tex":    "B \\input{main}",
	}
	got := pickAndJoinTeX(files)
	if !strings.Contains(got, "A") || !strings.Contains(got, "B") {
		t.Fatalf("both files must appear once, got %q", got)
	}
	if strings.Count(got, "B ") != 1 {
		t.Fatalf("cycle must not duplicate content: %q", got)
	}
}

// 429 символов «оглавления» раньше проходили как успешный парс — и чат с
// обзором работали по абстракту, думая, что видят полный текст.
func TestExtractTeXFromEPrintRejectsTinyYield(t *testing.T) {
	src := "\\documentclass{article}\\begin{document}\\input{sections/intro}\\input{sections/results}\\section{Acknowledgement}Thanks.\\end{document}"
	res, ok := ExtractTeXFromEPrint([]byte(src))
	if ok {
		t.Fatalf("a wrapper without its sections must not count as extracted text: %q", res.PlainText)
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "too short") {
		t.Fatalf("expected a too-short warning, got %v", res.Warnings)
	}
}
