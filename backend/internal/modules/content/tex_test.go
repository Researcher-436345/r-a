package content

import "testing"

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
