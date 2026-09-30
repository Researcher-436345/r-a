package client

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Compose runs a docker compose command against the stand's project. The
// degradation scenarios (F12) stop and start containers with it; everything
// else stays on the HTTP API. STAND_COMPOSE_DIR points at the repo root
// (default: two levels above the integration package).
func (s Stand) Compose(t testing.TB, args ...string) string {
	t.Helper()
	dir := os.Getenv("STAND_COMPOSE_DIR")
	if dir == "" {
		dir = "../.."
	}
	project := os.Getenv("STAND_COMPOSE_PROJECT")
	if project == "" {
		project = "r-a-test"
	}
	full := append([]string{"compose", "-p", project, "-f", "docker-compose.yml", "-f", "docker-compose.test.yml"}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker compose %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// StopService stops one container of the stand and restores it when the test ends.
func (s Stand) StopService(t testing.TB, name string) {
	t.Helper()
	s.Compose(t, "stop", name)
	t.Cleanup(func() {
		s.Compose(t, "start", name)
		_ = s.WaitReady(2 * time.Minute)
	})
}

// RestartService restarts one container of the stand.
func (s Stand) RestartService(t testing.TB, name string) {
	t.Helper()
	s.Compose(t, "restart", name)
}
