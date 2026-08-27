package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/model"
)

func addWorkTitleLatin(t *testing.T, workID int64, lang, title, latin string, kind, provenance int16) {
	t.Helper()
	row := &model.CatalogWorkTitle{
		WorkID: workID, Lang: lang, Title: title, Kind: kind, Provenance: provenance,
	}
	if latin != "" {
		row.Latin = &latin
	}
	if err := testDB.Create(row).Error; err != nil {
		t.Fatalf("create work title %s/%s: %v", lang, title, err)
	}
}

func localizedValues(m map[string]dto.PublicLocalizedName) map[string]string {
	out := make(map[string]string, len(m))
	for locale, name := range m {
		out[locale] = name.Value
	}
	return out
}

func listItemByID(t *testing.T, svc *PublicService, id int64) dto.PublicWorkListItem {
	t.Helper()
	page, err := svc.WorksList(t.Context(),
		WorksListFilter{Sort: "id", Include: ParseWorksListInclude("names")}, "", 50)
	if err != nil {
		t.Fatalf("WorksList include=names: %v", err)
	}
	for _, it := range page.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("work %d missing from the list page", id)
	return dto.PublicWorkListItem{}
}

func TestPublicWorkLocalizedKeysEveryTaggedLocale(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitle(t, w.ID, "ja", "こころ", model.WorkTitleKindOfficial)
	addWorkTitle(t, w.ID, "ko", "코코로", model.WorkTitleKindOfficial)
	addWorkTitle(t, w.ID, "ru", "Кокоро", model.WorkTitleKindOfficial)
	addWorkTitle(t, w.ID, "", "Kokoro", model.WorkTitleKindAlias)

	item := listItemByID(t, svc, w.ID)
	want := map[string]string{"ja": "こころ", "ko": "코코로", "ru": "Кокоро"}
	if got := localizedValues(item.Localized); !reflect.DeepEqual(got, want) {
		t.Fatalf("localized = %+v, want %+v (an untagged row answers no locale)", got, want)
	}
}

// The election is first-row-wins over rows nativeWorkTitles ordered by
// (provenance, kind, id). This pins that the SQL order really puts source ahead
// of machine: a machine `official` (kind 0) must still lose to a source `alias`
// (kind 1), exactly as the four slots behave.
func TestPublicWorkLocalizedSourceBeatsMachine(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	filled := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitle(t, filled.ID, "ja", "こころ", model.WorkTitleKindOfficial)
	addWorkTitleP(t, filled.ID, "zh-Hans", "心", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine)

	sourced := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "そら")
	addWorkTitle(t, sourced.ID, "ja", "そら", model.WorkTitleKindOfficial)
	addWorkTitleP(t, sourced.ID, "zh-Hans", "机翻天空", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine)
	addWorkTitleP(t, sourced.ID, "zh-Hans", "天空", model.WorkTitleKindAlias, model.WorkTitleProvenanceSource)

	f := listItemByID(t, svc, filled.ID).Localized
	if f["zh-Hans"].Value != "心" || !f["zh-Hans"].Machine {
		t.Fatalf("localized[zh-Hans] = %+v, want the machine row filling the empty locale, flagged", f["zh-Hans"])
	}
	if f["ja"].Value != "こころ" || f["ja"].Machine {
		t.Fatalf("localized[ja] = %+v, want the source row unflagged", f["ja"])
	}

	s := listItemByID(t, svc, sourced.ID).Localized
	if s["zh-Hans"].Value != "天空" || s["zh-Hans"].Machine || s["zh-Hans"].Kind != "alias" {
		t.Fatalf("localized[zh-Hans] = %+v, a source alias must beat a machine official", s["zh-Hans"])
	}
}

// The detail face loads titles withHints=true — one extra row kind and one more
// ORDER BY level (lang) than the list face. Both must elect the same winner,
// and the search_hint row must never take a locale: here it sorts AHEAD of the
// real title (source hint kind 3 before machine official kind 0), which is the
// only arrangement that can expose a slot-before-kind mistake.
func TestPublicWorkDetailLocalizedEqualsListElection(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitle(t, w.ID, "ja", "こころ", model.WorkTitleKindOfficial)
	addWorkTitle(t, w.ID, "zh-Hans", "kokoro 心 检索用", model.WorkTitleKindSearchHint)
	addWorkTitleP(t, w.ID, "zh-Hans", "心", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine)
	addWorkTitle(t, w.ID, "en", "Kokoro", model.WorkTitleKindAlias)

	rec, found, err := svc.WorkDetail(t.Context(), w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail: found=%v err=%v", found, err)
	}
	list := listItemByID(t, svc, w.ID)
	if !reflect.DeepEqual(rec.Localized, list.Localized) {
		t.Fatalf("detail localized = %+v, list localized = %+v — the two paths must elect the same winner",
			rec.Localized, list.Localized)
	}
	zh := rec.Localized["zh-Hans"]
	if zh.Value != "心" || !zh.Machine {
		t.Fatalf("localized[zh-Hans] = %+v, want the official row behind the search hint", zh)
	}
	for _, name := range rec.Localized {
		if name.Kind == "" {
			t.Fatalf("localized = %+v, a search_hint row leaked into the map", rec.Localized)
		}
	}
}

func TestPublicWorkDetailLocalizedIsAlwaysEmitted(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "無題")
	rec, found, err := svc.WorkDetail(t.Context(), w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail: found=%v err=%v", found, err)
	}
	if rec.Localized == nil || len(rec.Localized) != 0 {
		t.Fatalf("localized = %#v, want an allocated empty map on a title-less work", rec.Localized)
	}
	raw, err := json.Marshal(rec.Localized)
	if err != nil {
		t.Fatalf("marshal localized: %v", err)
	}
	if string(raw) != "{}" {
		t.Fatalf("localized wire shape = %s, want {}", raw)
	}
	if rec.Latin != "" {
		t.Fatalf("latin = %q, want empty with no title rows", rec.Latin)
	}
}

func TestPublicWorkLatinComesFromTheDisplayTitleRow(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitleLatin(t, w.ID, "ja", "こころ", "Kokoro", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	addWorkTitleLatin(t, w.ID, "en", "Heart", "Heart", model.WorkTitleKindAlias, model.WorkTitleProvenanceSource)

	rec, found, err := svc.WorkDetail(t.Context(), w.ID, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("WorkDetail: found=%v err=%v", found, err)
	}
	if rec.Latin != "Kokoro" {
		t.Fatalf("detail latin = %q, want the romanisation of the display row", rec.Latin)
	}
	if got := listItemByID(t, svc, w.ID).Latin; got != "Kokoro" {
		t.Fatalf("list latin = %q, want the romanisation of the display row", got)
	}
}

// latin and localized ride the include=names block: without it they must be
// absent from the wire, not present-and-empty. An emitted `"localized":{}` on a
// list that never loaded titles would read as "this work has no localized
// title", which is a different claim from "you did not ask".
func TestPublicWorkListNamePrimitivesAreIncludeGated(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitleLatin(t, w.ID, "ja", "こころ", "Kokoro", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	page, err := svc.WorksList(t.Context(), WorksListFilter{Sort: "id"}, "", 50)
	if err != nil {
		t.Fatalf("WorksList: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page = %d items, want 1", len(page.Items))
	}
	raw, err := json.Marshal(page.Items[0])
	if err != nil {
		t.Fatalf("marshal item: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal item: %v", err)
	}
	for _, key := range []string{"latin", "localized"} {
		if _, present := wire[key]; present {
			t.Fatalf("item without include=names carries %q: %s", key, raw)
		}
	}

	item := listItemByID(t, svc, w.ID)
	if item.Latin != "Kokoro" || item.Localized["ja"].Value != "こころ" {
		t.Fatalf("include=names item = %+v, want both primitives filled", item)
	}
}

func TestPublicWorkBriefCarriesNamePrimitives(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitleLatin(t, w.ID, "ja", "こころ", "Kokoro", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	addWorkTitle(t, w.ID, "ko", "코코로", model.WorkTitleKindOfficial)
	addExternalRef(t, model.EntityTypeWork, w.ID, srcVNDB, "v4242", model.LinkKindExact)

	data, found, err := svc.Lookup(t.Context(), "vndb", "v4242", false)
	if err != nil || !found || data.Work == nil {
		t.Fatalf("Lookup: found=%v err=%v work=%+v", found, err, data.Work)
	}
	if data.Work.Latin != "Kokoro" {
		t.Fatalf("brief latin = %q, want Kokoro", data.Work.Latin)
	}
	want := map[string]string{"ja": "こころ", "ko": "코코로"}
	if got := localizedValues(data.Work.Localized); !reflect.DeepEqual(got, want) {
		t.Fatalf("brief localized = %+v, want %+v", got, want)
	}
}

func TestPublicWorkBriefLocalizedIsAlwaysEmitted(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "無題")
	addExternalRef(t, model.EntityTypeWork, w.ID, srcVNDB, "v4243", model.LinkKindExact)

	data, found, err := svc.Lookup(t.Context(), "vndb", "v4243", false)
	if err != nil || !found || data.Work == nil {
		t.Fatalf("Lookup: found=%v err=%v", found, err)
	}
	raw, err := json.Marshal(data.Work)
	if err != nil {
		t.Fatalf("marshal brief: %v", err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal brief: %v", err)
	}
	loc, present := wire["localized"]
	if !present {
		t.Fatalf("brief has no localized key: %s", raw)
	}
	if m, ok := loc.(map[string]any); !ok || len(m) != 0 {
		t.Fatalf("brief localized = %#v, want {}", loc)
	}
}

func TestWorkNamesByIDCarriesLocalized(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "こころ")
	addWorkTitle(t, w.ID, "ja", "こころ", model.WorkTitleKindOfficial)
	addWorkTitle(t, w.ID, "ko", "코코로", model.WorkTitleKindOfficial)
	bare := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "無題")

	blocks, err := svc.WorkNamesByID(t.Context(), []int64{w.ID, bare.ID})
	if err != nil {
		t.Fatalf("WorkNamesByID: %v", err)
	}
	want := map[string]string{"ja": "こころ", "ko": "코코로"}
	if values := localizedValues(blocks[w.ID].Localized); !reflect.DeepEqual(values, want) {
		t.Fatalf("localized = %+v, want %+v", values, want)
	}
	if len(blocks[bare.ID].Localized) != 0 {
		t.Fatalf("a work with no titles must carry no localized names, got %+v", blocks[bare.ID].Localized)
	}
}
