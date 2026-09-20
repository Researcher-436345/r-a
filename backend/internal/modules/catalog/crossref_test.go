package catalog

import (
	"context"
	"errors"
	"testing"
)

func TestIsContainerCrossrefType(t *testing.T) {
	for _, typ := range []string{"proceedings", "Proceedings-Series", "book-series", "journal", "journal-volume", "journal-issue", "report-series", "component"} {
		if !isContainerCrossrefType(typ) {
			t.Errorf("%s must be treated as a container", typ)
		}
	}
	for _, typ := range []string{"journal-article", "proceedings-article", "book", "book-chapter", "monograph", "posted-content", "dissertation", ""} {
		if isContainerCrossrefType(typ) {
			t.Errorf("%s is a real work", typ)
		}
	}
}

func TestFetchCrossrefMetadataReadsType(t *testing.T) {
	fakeIndexes{crossrefJSON: `{"status":"ok","message":{"DOI":"10.52202/075280","type":"proceedings",
		"title":["Advances in Neural Information Processing Systems 36"],
		"abstract":"<jats:p>Abstract <jats:italic>Deep</jats:italic> nets.</jats:p>",
		"published-print":{"date-parts":[[2023]]}}}`}.install(t)
	got, err := FetchCrossrefMetadata(context.Background(), "10.52202/075280")
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "proceedings" || !isContainerCrossrefType(got.Type) {
		t.Fatalf("type not read: %+v", got)
	}
	if got.Abstract == nil || *got.Abstract != "Deep nets." {
		t.Fatalf("abstract markup not stripped: %v", got.Abstract)
	}
	if got.Year == nil || *got.Year != 2023 {
		t.Fatalf("year=%v", got.Year)
	}
}

func TestFetchCrossrefMetadataNotFound(t *testing.T) {
	fakeIndexes{crossrefCode: 404}.install(t)
	if _, err := FetchCrossrefMetadata(context.Background(), "10.1/missing"); !errors.Is(err, errUnsupportedArticleURL) {
		t.Fatalf("a missing DOI must read as not found, got %v", err)
	}
}
