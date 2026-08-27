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

// claim_state used to admit all six states to any catalog:read key; pending,
// declined and hidden are moderation-workflow states and moved behind
// /v2/moderation/claims.
func TestParseWorksFilterClaimStateIsPublicOnly(t *testing.T) {
	for _, state := range []string{"pending", "declined", "hidden"} {
		_, err := parseWorksFilter(&listWorksInput{ClaimState: state})
		if err == nil || err.Code != problem.CodeUnknownEnumValue {
			t.Fatalf("claim_state=%s: %v", state, err)
		}
	}
	f, err := parseWorksFilter(&listWorksInput{ClaimState: "none,live,draft"})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.ClaimStates) != 3 {
		t.Fatalf("claim_states %+v", f.ClaimStates)
	}
}

func TestParseWorksFilterOwnerUID(t *testing.T) {
	_, err := parseWorksFilter(&listWorksInput{OwnerUID: "7"})
	if err == nil || err.Code != problem.CodeValidationFailed {
		t.Fatalf("owner_uid without site: %v", err)
	}
	if len(err.Errors) != 1 || err.Errors[0].Reason != problem.ReasonInconsistentWith {
		t.Fatalf("owner_uid field error %+v", err.Errors)
	}
	_, err = parseWorksFilter(&listWorksInput{OwnerUID: "nope", Site: "kungal"})
	if err == nil || err.Code != problem.CodeInvalidParameter {
		t.Fatalf("owner_uid non-numeric: %v", err)
	}
	f, err := parseWorksFilter(&listWorksInput{OwnerUID: "7", Site: "kungal"})
	if err != nil {
		t.Fatal(err)
	}
	if f.OwnerUID != 7 || f.Site != "kungal" {
		t.Fatalf("%+v", f)
	}
}

func TestListWorksFilteredRefusesOwnerUIDOnSearch(t *testing.T) {
	cat := &Catalog{Public: &catsvc.PublicService{}, Searcher: nil}
	f, perr := parseWorksFilter(&listWorksInput{OwnerUID: "7", Site: "kungal", Q: "kanon"})
	if perr != nil {
		t.Fatal(perr)
	}
	_, err := cat.ListWorksFiltered(t.Context(), collect.Query{}, f)
	p, ok := err.(*problem.Problem)
	if !ok || p.Code != problem.CodeMutuallyExclusiveParameters {
		t.Fatalf("owner_uid + q: %v", err)
	}
}

func TestParseMyClaimFilter(t *testing.T) {
	f, err := parseMyClaimFilter("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Kind != "submitted" {
		t.Fatalf("default kind %q", f.Kind)
	}
	// The me face is the bearer's own history, so every state is readable here
	// even though the public works lane only admits none/live/draft.
	f, err = parseMyClaimFilter("pending,declined,hidden,none,live,draft", "audited", " kungal ")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.ClaimStates) != 6 || f.Kind != "audited" || f.Site != "kungal" {
		t.Fatalf("%+v", f)
	}
	if f, err = parseMyClaimFilter("", "all", ""); err != nil || f.Kind != "" {
		t.Fatalf("kind=all maps to the service's empty Kind: %+v %v", f, err)
	}
	if _, err = parseMyClaimFilter("", "everything", ""); err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("kind=everything: %v", err)
	}
	if _, err = parseMyClaimFilter("nope", "", ""); err == nil || err.Code != problem.CodeUnknownEnumValue {
		t.Fatalf("claim_state=nope: %v", err)
	}
}
