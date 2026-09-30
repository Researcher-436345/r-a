//go:build integration

// Package integration walks the product's user flows against a running test
// stand (make stand-up) through the gateway only. See docs/testing/TEST_PLAN.md
// and docs/testing/flows/ for the scenarios each file implements.
package integration

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
)

var stand client.Stand

func TestMain(m *testing.M) {
	stand = client.FromEnv()
	if err := stand.WaitReady(3 * time.Minute); err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		fmt.Fprintln(os.Stderr, "start the stand with `make stand-up` (or set STAND_* env for a remote one)")
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// guest returns a fresh anonymous session for the test.
func guest(t *testing.T) *client.Session { return stand.Guest(t) }

// user registers, verifies and logs in a fresh account.
func user(t *testing.T, prefix string) *client.User { return stand.RegisterAndVerify(t, prefix) }

// onlyCall returns the single provider call whose messages contain match and
// fails when the fake recorded none or several.
func onlyCall(t *testing.T, match string) client.LLMRequest {
	t.Helper()
	calls := stand.LLM().Requests(t, match)
	if len(calls) != 1 {
		t.Fatalf("expected exactly one provider call containing %q, got %d", match, len(calls))
	}
	return calls[0]
}
