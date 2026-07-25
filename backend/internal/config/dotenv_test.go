package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvLine(t *testing.T) {
	cases := []struct {
		in      string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"KEY=value", "KEY", "value", true},
		{"  KEY = value ", "KEY", "value", true},
		{"export KEY=value", "KEY", "value", true},
		{`KEY="quoted value"`, "KEY", "quoted value", true},
		{"KEY='single'", "KEY", "single", true},
		{"KEY=", "KEY", "", true},
		{"# comment", "", "", false},
		{"", "", "", false},
		{"noequals", "", "", false},
	}
	for _, c := range cases {
		k, v, ok := parseDotEnvLine(c.in)
		if ok != c.wantOK || k != c.wantKey || v != c.wantVal {
			t.Errorf("parseDotEnvLine(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.in, k, v, ok, c.wantKey, c.wantVal, c.wantOK)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "FOO=from_file\nBAR=bar_val\n# comment\nPRESET=should_not_override\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PRESET", "from_env") // already set -> must win
	t.Setenv("FOO", "")            // ensure clean; will be unset below
	os.Unsetenv("FOO")
	os.Unsetenv("BAR")

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}

	if got := os.Getenv("FOO"); got != "from_file" {
		t.Errorf("FOO = %q, want from_file", got)
	}
	if got := os.Getenv("BAR"); got != "bar_val" {
		t.Errorf("BAR = %q, want bar_val", got)
	}
	if got := os.Getenv("PRESET"); got != "from_env" {
		t.Errorf("PRESET = %q, want from_env (env should win over .env)", got)
	}
}

func TestLoadDotEnvMissingFileOK(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Errorf("missing .env should not error, got %v", err)
	}
}
