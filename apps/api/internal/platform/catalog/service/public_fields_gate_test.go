package service

import (
	"encoding/json"
	"testing"
)

// The whole point of the wave: ?fields= gates the LOAD, not the serialization.
// The retired galgame face trimmed marshalled bytes only, so a caller asking
// for two keys still paid for every query the full detail runs.
var detailBlockMarkers = map[string]string{
	"titles":      "catalog_work_title",
	"releases":    "catalog_release",
	"labels":      "catalog_work_label",
	"refs":        "catalog_external_ref",
	"characters":  "catalog_work_character",
	"intros":      "catalog_work_intro",
	"covers":      "catalog_work_cover",
	"screenshots": "catalog_work_screenshot",
	"ratings":     "catalog_work_rating",
	"tags":        "catalog_work_tag",
	"popularity":  "catalog_work_popularity",
	"playtimes":   "catalog_work_playtime",
	"platforms":   "catalog_work_platform",
	"claimed_by":  "display_nsfw",
}

func topLevelKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	members, err := decodeJSONObject(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	keys := make([]string, 0, len(members))
	for _, m := range members {
		keys = append(keys, m.key)
	}
	return keys
}

func projectedKeys(t *testing.T, v any, sel PublicFields) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	trimmed, err := sel.ProjectObject(raw)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	members, err := decodeJSONObject(trimmed)
	if err != nil {
		t.Fatalf("decode projected: %v", err)
	}
	keys := make([]string, 0, len(members))
	for _, m := range members {
		keys = append(keys, m.key)
	}
	return keys
}

func TestWorkDetailFieldsGatesTheLoad(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()
	id := seedRichWork(t)

	var stmts []string
	svc := recordingPublicSvc(&stmts)

	stmts = stmts[:0]
	full, found, err := svc.WorkDetail(ctx, id, PublicInclude{}, false, 0, PublicFields{})
	if err != nil || !found {
		t.Fatalf("unprojected detail: found=%v err=%v", found, err)
	}
	baseline := len(stmts)
	if baseline < 10 {
		t.Fatalf("unprojected detail ran %d statements — the fixture is not rich enough to prove anything", baseline)
	}

	stmts = stmts[:0]
	lean, found, err := svc.WorkDetail(ctx, id, PublicInclude{}, false, 0, ParsePublicFields("display_name,id"))
	if err != nil || !found {
		t.Fatalf("fields=display_name,id: found=%v err=%v", found, err)
	}
	if n := len(stmts); n != 1 {
		t.Fatalf("fields=display_name,id ran %d statements (want 1, the work row itself); baseline is %d:\n%v",
			n, baseline, stmts)
	}
	t.Logf("work detail statements: %d unprojected, %d under fields=display_name,id", baseline, len(stmts))
	for block, marker := range detailBlockMarkers {
		if n := countMatching(stmts, marker); n != 0 {
			t.Fatalf("fields=display_name,id still ran the %s loader (%s) %d time(s)", block, marker, n)
		}
	}
	if lean.DisplayName != full.DisplayName || lean.ID != full.ID {
		t.Fatalf("identity keys drifted under fields=: %+v vs %+v", lean, full)
	}
	if got := projectedKeys(t, lean, ParsePublicFields("display_name,id")); len(got) != 2 {
		t.Fatalf("projected keys = %v (want exactly id + display_name)", got)
	}
	// The blocks whose loaders were skipped must not reach the wire as an
	// empty-but-truthful-looking answer; the projection is what guarantees it.
	if len(topLevelKeys(t, lean)) != len(topLevelKeys(t, full)) {
		t.Fatal("the DTO itself changed shape — fields= must trim on the way out, not restructure the record")
	}
}

func TestWorkDetailFieldsLoadsADerivedKeysDependency(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()
	id := seedRichWork(t)

	var stmts []string
	svc := recordingPublicSvc(&stmts)

	stmts = stmts[:0]
	rec, found, err := svc.WorkDetail(ctx, id, PublicInclude{}, false, 0, ParsePublicFields("release_date"))
	if err != nil || !found {
		t.Fatalf("fields=release_date: found=%v err=%v", found, err)
	}
	if n := countMatching(stmts, "catalog_release"); n == 0 {
		t.Fatal("fields=release_date did not load the releases it is derived from")
	}
	for _, marker := range []string{"catalog_work_cover", "catalog_work_tag", "catalog_work_character", "display_nsfw"} {
		if n := countMatching(stmts, marker); n != 0 {
			t.Fatalf("fields=release_date still ran %s %d time(s)", marker, n)
		}
	}
	if rec.ReleaseDate == nil || *rec.ReleaseDate != "2021-06-04" {
		t.Fatalf("release_date = %v (want the derived 2021-06-04)", rec.ReleaseDate)
	}
	// The dependency loads, but its own key stays behind the selection.
	got := projectedKeys(t, rec, ParsePublicFields("release_date"))
	if len(got) != 2 || got[0] != "id" || got[1] != "release_date" {
		t.Fatalf("projected keys = %v (want id + release_date only — releases must not ride along)", got)
	}
}

func TestWorkDetailFieldsIntersectsInclude(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	w := createWork(t, "交差本編")
	other := createWork(t, "交差関連作")
	createWorkRelation(t, w.ID, other.ID)

	var stmts []string
	svc := recordingPublicSvc(&stmts)

	stmts = stmts[:0]
	rec, found, err := svc.WorkDetail(ctx, w.ID, PublicInclude{Relations: true}, false, 0, ParsePublicFields("id,display_name"))
	if err != nil || !found {
		t.Fatalf("include=relations + fields without it: found=%v err=%v", found, err)
	}
	if n := countMatching(stmts, relationQueryMarker); n != 0 {
		t.Fatalf("a fields= selection that drops relations still ran the relation query %d time(s)", n)
	}
	if len(rec.Relations) != 0 {
		t.Fatalf("relations = %v (want empty: fields= narrows include=, never widens it)", rec.Relations)
	}

	// fields= alone must NOT expand an include-gated block.
	stmts = stmts[:0]
	rec, found, err = svc.WorkDetail(ctx, w.ID, PublicInclude{}, false, 0, ParsePublicFields("id,relations"))
	if err != nil || !found {
		t.Fatalf("fields=relations without include: found=%v err=%v", found, err)
	}
	if n := countMatching(stmts, relationQueryMarker); n != 0 {
		t.Fatalf("fields=relations without include=relations expanded the block (%d queries)", n)
	}
	if got := projectedKeys(t, rec, ParsePublicFields("id,relations")); len(got) != 1 || got[0] != "id" {
		t.Fatalf("projected keys = %v (want id only — relations is omitempty and was never loaded)", got)
	}

	stmts = stmts[:0]
	rec, found, err = svc.WorkDetail(ctx, w.ID, PublicInclude{Relations: true}, false, 0, ParsePublicFields("id,relations"))
	if err != nil || !found {
		t.Fatalf("include + fields both naming relations: found=%v err=%v", found, err)
	}
	if n := countMatching(stmts, relationQueryMarker); n != 1 {
		t.Fatalf("include=relations & fields=relations ran the relation query %d time(s) (want 1)", n)
	}
	if len(rec.Relations) != 1 || rec.Relations[0].Work.ID != other.ID {
		t.Fatalf("relations = %v (want the one edge to %d)", rec.Relations, other.ID)
	}
}

func TestWorksListFieldsGatesTheLoad(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()
	seedRichWork(t)

	var stmts []string
	svc := recordingPublicSvc(&stmts)

	stmts = stmts[:0]
	if _, err := svc.WorksList(ctx, WorksListFilter{Sort: "id"}, "", 50); err != nil {
		t.Fatalf("unprojected list: %v", err)
	}
	baseline := len(stmts)

	stmts = stmts[:0]
	page, err := svc.WorksList(ctx, WorksListFilter{Sort: "id", Fields: ParsePublicFields("display_name,id")}, "", 50)
	if err != nil {
		t.Fatalf("projected list: %v", err)
	}
	if n := len(stmts); n != 1 {
		t.Fatalf("fields=display_name,id on the list ran %d statements (want 1, the page query); baseline %d:\n%v",
			n, baseline, stmts)
	}
	t.Logf("works list statements: %d unprojected, %d under fields=display_name,id", baseline, len(stmts))
	// The three loads the list runs unconditionally today.
	for _, marker := range []string{"catalog_work_cover", "display_nsfw", "released_y"} {
		if n := countMatching(stmts, marker); n != 0 {
			t.Fatalf("projected list still ran %s %d time(s)", marker, n)
		}
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %d (want the one seeded work)", len(page.Items))
	}

	// include ∩ fields: the include token alone no longer buys the query.
	stmts = stmts[:0]
	if _, err = svc.WorksList(ctx, WorksListFilter{
		Sort:    "id",
		Include: WorksListInclude{Ratings: true, Covers: true},
		Fields:  ParsePublicFields("display_name,id"),
	}, "", 50); err != nil {
		t.Fatalf("include+fields list: %v", err)
	}
	for _, marker := range []string{"catalog_work_rating", "catalog_work_cover"} {
		if n := countMatching(stmts, marker); n != 0 {
			t.Fatalf("an include token the fields selection drops still ran %s %d time(s)", marker, n)
		}
	}
}
