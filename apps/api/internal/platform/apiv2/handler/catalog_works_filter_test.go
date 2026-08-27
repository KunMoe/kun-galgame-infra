package handler

import (
	"testing"

	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	catmodel "api/internal/platform/catalog/model"
	catsvc "api/internal/platform/catalog/service"
)

func TestParseWorksFilterClosedVocab(t *testing.T) {
	_, err := parseWorksFilter(&listWorksInput{ContentRating: "r17"})
	if err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("content_rating: %v", err)
	}
	_, err = parseWorksFilter(&listWorksInput{ClaimState: "live,nope"})
	if err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("claim_state: %v", err)
	}
	_, err = parseWorksFilter(&listWorksInput{ReleasedAfter: "2024-13-01"})
	if err == nil {
		t.Fatal("bad date")
	}
	f, err := parseWorksFilter(&listWorksInput{
		Q: "千恋", ContentRating: "r18", CompanyID: "12", TagID: "1,2",
		ReleasedAfter: "2020-01-01", OLang: "ja", Claimed: "true",
		ClaimState: "live", ContentLimit: "sfw", CompanyRollup: "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.Q != "千恋" || f.CompanyID != 12 || len(f.TagIDs) != 2 || f.ReleasedAfter != 20200101 {
		t.Fatalf("%+v", f)
	}
	if f.ContentRating == nil || *f.ContentRating != catmodel.ContentRatingR18 || !f.CompanyRollup {
		t.Fatalf("%+v", f)
	}
	if len(f.OLang.Values) != 1 || f.OLang.Values[0] != "ja" {
		t.Fatalf("olang %+v", f.OLang)
	}
}

func TestParseWorksFilterOLangDefaultAll(t *testing.T) {
	f, err := parseWorksFilter(&listWorksInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !f.OLang.All {
		t.Fatalf("default olang %+v", f.OLang)
	}
}

func TestSearchWorksRequested(t *testing.T) {
	if searchWorksRequested(collect.Query{}, worksFilter{}) {
		t.Fatal("browse")
	}
	if !searchWorksRequested(collect.Query{}, worksFilter{Q: "x"}) {
		t.Fatal("q")
	}
	if !searchWorksRequested(collect.Query{Sort: "relevance"}, worksFilter{}) {
		t.Fatal("relevance")
	}
	if !searchWorksRequested(collect.Query{Facets: []string{"tag_id"}}, worksFilter{}) {
		t.Fatal("facets")
	}
}

func TestListWorksIncludeMapping(t *testing.T) {
	inc := listWorksInclude([]string{"titles", "companies", "characters"})
	if !inc.Names || !inc.Labels || inc.Intros {
		t.Fatalf("%+v", inc)
	}
}

func TestListWorksFilteredUnbound(t *testing.T) {
	_, err := (*Catalog)(nil).ListWorksFiltered(t.Context(), collect.Query{}, worksFilter{})
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeServiceUnavailable {
		t.Fatalf("%v", err)
	}
}

func TestListWorksFilteredRelevanceNeedsQ(t *testing.T) {
	c := &Catalog{Public: &catsvc.PublicService{}}
	_, err := c.ListWorksFiltered(t.Context(), collect.Query{Sort: "relevance"}, worksFilter{})
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeInvalidParameter {
		t.Fatalf("%v", err)
	}
}

func TestListWorksFilteredQAndIDsExclusive(t *testing.T) {
	c := &Catalog{Public: &catsvc.PublicService{}}
	_, err := c.ListWorksFiltered(t.Context(), collect.Query{Batch: true, IDs: []string{"1"}}, worksFilter{Q: "x"})
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeMutuallyExclusiveParameters {
		t.Fatalf("%v", err)
	}
}
