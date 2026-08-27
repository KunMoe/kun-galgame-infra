package handler

import (
	"testing"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	catsearch "api/internal/platform/catalog/search"
)

func TestSearchIndexMapping(t *testing.T) {
	uid, v1 := searchIndex("credit_name")
	if uid != catsearch.IndexCreditNames || v1 != "name" {
		t.Fatalf("credit_name %s %s", uid, v1)
	}
	uid, v1 = searchIndex("company")
	if uid != catsearch.IndexLabels || v1 != "label" {
		t.Fatalf("company %s %s", uid, v1)
	}
	uid, v1 = searchIndex("work")
	if uid != catsearch.IndexWorks || v1 != "work" {
		t.Fatalf("work %s %s", uid, v1)
	}
}

func TestStripSearchID(t *testing.T) {
	n, ok := stripSearchID("w19658")
	if !ok || n != 19658 {
		t.Fatalf("%d %v", n, ok)
	}
	if _, ok := stripSearchID("x"); ok {
		t.Fatal("short")
	}
}

func TestSearchUnboundAndCursor(t *testing.T) {
	_, err := (*Catalog)(nil).Search(t.Context(), collect.Query{}, "work", "foo", "")
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
	c := &Catalog{Searcher: &catsearch.Indexer{}}
	_, err = c.Search(t.Context(), collect.Query{Cursor: "x"}, "work", "foo", "")
	p, ok = err.(*problem.Problem)
	if !ok || p.Code != problem.CodeInvalidParameter {
		t.Fatalf("cursor %v", err)
	}
	_, err = c.Search(t.Context(), collect.Query{}, "", "foo", "")
	p, ok = err.(*problem.Problem)
	if !ok || p.Code != problem.CodeInvalidParameter {
		t.Fatalf("object %v", err)
	}
	_, err = c.Search(t.Context(), collect.Query{}, "person", "foo", "")
	p, ok = err.(*problem.Problem)
	if !ok || p.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("person %v", err)
	}
}
