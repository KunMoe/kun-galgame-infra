package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"api/internal/platform/apiv2/repr"
	"api/internal/platform/catalog/dto"
)

func TestWorkFromListItemBasic(t *testing.T) {
	date := "2011-06-24"
	it := dto.PublicWorkListItem{
		ID:            19658,
		Medium:        "galgame",
		DisplayName:   "紅殻のパンドラ",
		ContentRating: "r18",
		OLang:         "ja",
		ReleaseDate:   &date,
		Updated:       "2026-08-19T02:31:06Z",
		Cover:         "https://img.example/ab/cd/" + strings.Repeat("a", 64) + ".webp",
		ClaimedBy:     &dto.PublicClaimedBy{Site: "touchgal", WorkID: 8812, State: "live", ContentLimit: "sfw"},
	}
	w := workFromListItem(it)
	if w.Object != "work" || w.ID != "19658" || w.ReleaseStatus != "released" {
		t.Fatalf("work %+v", w)
	}
	if w.Claim == nil || w.Claim.SiteWorkID != "8812" {
		t.Fatalf("claim %+v", w.Claim)
	}
	if w.Cover == nil || w.Cover.Hash != strings.Repeat("a", 64) {
		t.Fatalf("cover %+v", w.Cover)
	}
	if w.Cover.Sexual != nil {
		t.Fatalf("url-only cover must not claim sexual: %+v", w.Cover.Sexual)
	}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"id":19658`) || strings.Contains(string(b), `"kind"`) {
		t.Fatalf("wire: %s", b)
	}
}

func TestWorkFromListItemUnknownRelease(t *testing.T) {
	w := workFromListItem(dto.PublicWorkListItem{
		ID: 1, Medium: "galgame", DisplayName: "x", OLang: "ja", ContentRating: "all_ages",
		Updated: "2026-01-01T00:00:00Z",
	})
	if w.ReleaseStatus != "unknown" || w.ReleaseDate != nil || w.ReleaseDatePrecision != nil {
		t.Fatalf("unknown release %+v", w)
	}
}

func TestWorkFromDetailCoverSlots(t *testing.T) {
	hash := strings.Repeat("b", 64)
	rec := dto.PublicCatalogWork{
		ID: 2, Medium: "galgame", DisplayName: "y", OLang: "ja", ContentRating: "sensitive",
		Created: "2024-01-01T00:00:00Z", Updated: "2024-02-01T00:00:00Z",
		CoverSlots: &dto.PublicWorkCoverSlots{
			Portrait: &dto.PublicCoverSlot{
				URL: "https://img.example/" + hash + ".webp", Width: 800, Height: 1131, Sexual: 0, Source: "vndb",
			},
		},
	}
	w := workFromDetail(rec)
	if w.Cover == nil || w.Cover.Hash != hash || w.Cover.Sexual == nil || *w.Cover.Sexual != "safe" {
		t.Fatalf("slot cover %+v", w.Cover)
	}
}

func TestHashFromURL(t *testing.T) {
	h := strings.Repeat("c", 64)
	if got := hashFromURL("https://cdn.example/" + h + ".webp"); got != h {
		t.Fatalf("got %q", got)
	}
	if hashFromURL("https://cdn.example/not-a-hash.png") != "" {
		t.Fatal("non-hash")
	}
}

func TestClaimFromNil(t *testing.T) {
	if claimFrom(nil) != nil {
		t.Fatal("nil claim")
	}
	c := claimFrom(&dto.PublicClaimedBy{Site: "s", WorkID: 1, State: "draft", ContentLimit: "nsfw"})
	if c.SiteWorkID != "1" || c.State != "draft" {
		t.Fatalf("%+v", c)
	}
}

func TestTaxonomyListItemMappers(t *testing.T) {
	c := companyFromListItem(dto.PublicLabelListItem{
		ID: 7, DisplayName: "Alcot", Kind: "game_brand", WorkCount: 3,
		Localized: map[string]dto.PublicLocalizedName{"zh-Hans": {Value: "品牌"}},
	})
	if c.Object != "company" || c.ID != "7" || c.CompanyKind != "game_brand" || c.WorkCount != 3 {
		t.Fatalf("company %+v", c)
	}
	if c.Localized["zh-Hans"].Value != "品牌" {
		t.Fatalf("localized %+v", c.Localized)
	}
	other := companyFromListItem(dto.PublicLabelListItem{ID: 8, DisplayName: "x", Kind: "other"})
	if other.CompanyKind != "group" {
		t.Fatalf("other kind %s", other.CompanyKind)
	}
	tag := tagFromListItem(dto.PublicTagListItem{ID: 2, Name: "nukige", Tier: "core", Kind: "content", WorkCount: 9})
	if tag.DisplayName != "nukige" || tag.TagKind != "content" || tag.ID != "2" {
		t.Fatalf("tag %+v", tag)
	}
	eng := engineFromListItem(dto.PublicEngineListItem{ID: 4, Name: "KiriKiri", WorkCount: 1})
	if eng.DisplayName != "KiriKiri" || eng.ID != "4" {
		t.Fatalf("engine %+v", eng)
	}
	ser := seriesFromDetail(5, "シリーズ")
	if ser.Object != "series" || ser.ID != "5" || ser.DisplayName != "シリーズ" {
		t.Fatalf("series %+v", ser)
	}
}

func TestLocalizedFromEmpty(t *testing.T) {
	if localizedFrom(nil) == nil {
		t.Fatal("nil must become empty map")
	}
	in := map[string]dto.PublicLocalizedName{"zh-Hans": {Value: "名", Machine: true}}
	out := localizedFrom(in)
	if out["zh-Hans"] != (repr.LocalizedText{Value: "名", IsMachine: true}) {
		t.Fatalf("%+v", out)
	}
}
