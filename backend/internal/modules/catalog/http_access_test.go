package catalog

import (
	"github.com/centraluniversity/researcher/internal/modules/identity"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddToLibraryDefaultsToTrue(t *testing.T) {
	if !addToLibrary(nil) {
		t.Fatal("omitted add_to_library must preserve the existing add behavior")
	}
	value := false
	if addToLibrary(&value) {
		t.Fatal("explicit false must open a paper without adding it to the library")
	}
	value = true
	if !addToLibrary(&value) {
		t.Fatal("explicit true must add the paper to the library")
	}
}

func TestGuestCannotReadUploadedVersion(t *testing.T) {
	for _, source := range []string{"upload", "", "unknown"} {
		if publicVersion(Version{Source: source}) {
			t.Fatalf("guest can access %q", source)
		}
	}
	for _, source := range []string{"arxiv", "doi", "web_pdf", "openalex"} {
		if !publicVersion(Version{Source: source}) {
			t.Fatalf("guest cannot read public source %q", source)
		}
	}
}

func TestGuestResolveRateSubject(t *testing.T) {
	for _, tc := range []struct{ remote, forwarded, want string }{
		{"192.0.2.1:1234", "", "guest:192.0.2.1"},
		{"192.0.2.2:5678", "", "guest:192.0.2.2"},
		{"127.0.0.1:1234", "spoofed, 192.0.2.1", "guest:192.0.2.1"},
		{"[2001:db8::1]:1234", "", "guest:2001:db8::1"},
	} {
		r := httptest.NewRequest("POST", "/papers/arxiv/open", nil)
		r.RemoteAddr = tc.remote
		r.Header.Set("X-Forwarded-For", tc.forwarded)
		h := identity.GuestOrAuthenticatedFromGateway(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := resolveRateSubject(r); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		}))
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
}
