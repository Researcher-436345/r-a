package assistant

import "testing"

func TestEstimateTokens(t *testing.T) {
	if EstimateTokens("") != 0 {
		t.Fatalf("empty")
	}
	if EstimateTokens("abcd") != 1 {
		t.Fatalf("got %d", EstimateTokens("abcd"))
	}
}

func TestTruncateToTokensKeepsMarker(t *testing.T) {
	long := stringsRepeat("word ", 5000)
	out := truncateToTokens(long, 50)
	if EstimateTokens(out) > 80 {
		t.Fatalf("too long: %d", EstimateTokens(out))
	}
	if !contains(out, "truncated") {
		t.Fatalf("missing truncation marker")
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
