package handler

import (
	"testing"
	"time"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	catmodel "api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
)

func TestObjectEntityTypeRoundTrip(t *testing.T) {
	for _, object := range []string{"work", "release", "character", "credit_name", "person", "company", "tag", "engine"} {
		et, ok := objectEntityType(object)
		if !ok {
			t.Fatalf("%s: missing", object)
		}
		got, ok := entityTypeObject(et)
		if !ok || got != object {
			t.Fatalf("%s: et=%d got=%s", object, et, got)
		}
	}
	if _, ok := objectEntityType("label"); ok {
		t.Fatal("v1 label is not a v2 object")
	}
	if got, _ := entityTypeObject(catmodel.EntityTypeLabel); got != "company" {
		t.Fatal("label maps to company")
	}
}

func TestFeedNoBatch(t *testing.T) {
	p := feedNoBatch("changes")
	if p.Code != problem.CodeInvalidParameter {
		t.Fatalf("%s", p.Code)
	}
}

func TestRedirectCursorRoundTrip(t *testing.T) {
	in := repository.RedirectCursor{
		MergedAt: time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC), EntityType: catmodel.EntityTypeWork, OldID: 99,
	}
	out, err := decodeRedirectInner(encodeRedirectInner(in))
	if err != nil {
		t.Fatal(err)
	}
	if out.EntityType != in.EntityType || out.OldID != in.OldID || !out.MergedAt.Equal(in.MergedAt) {
		t.Fatalf("%+v vs %+v", out, in)
	}
	if _, err := decodeRedirectInner("not-b64"); err == nil {
		t.Fatal("bad inner")
	}
}

func TestListChangesUnbound(t *testing.T) {
	_, err := (*Catalog)(nil).ListChanges(t.Context(), collect.Query{})
	var p *problem.Problem
	if err == nil {
		t.Fatal("expected error")
	}
	if !errorAsProblem(err, &p) || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}

func errorAsProblem(err error, dest **problem.Problem) bool {
	p, ok := err.(*problem.Problem)
	if ok {
		*dest = p
	}
	return ok
}

func TestChangeMapping(t *testing.T) {
	ch := repr.Change{Object: "change", TargetObject: "work", ID: "1", UpdatedAt: "2026-01-01T00:00:00Z"}
	if ch.Object != "change" || ch.ID != "1" {
		t.Fatalf("%+v", ch)
	}
}
