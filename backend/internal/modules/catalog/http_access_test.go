package catalog

import "testing"

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
	for _, source := range []string{"arxiv", "doi", "web_pdf"} {
		if !publicVersion(Version{Source: source}) {
			t.Fatalf("guest cannot read public source %q", source)
		}
	}
}
