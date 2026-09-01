// public_display_limit_test.go — A2-R5: the EDITORIAL DISPLAY axis
// (refs/proj/140).
//
// The incident this closes (doc 106 §38): a downstream mapped the catalog's AGE
// axis (content_rating) onto its own DISPLAY gate (content_limit) and hid every
// r18 game — 94.5% of the claimed live population — collapsing its indexable
// surface from 6,117 works to 599. The two axes are different questions, and on
// production they disagree in bulk: 5,568 works are r18 games whose wiki display
// material is editorially sfw, and 50 are all_ages games the wiki marked nsfw.
//
// The cases below pin the axis where it actually runs: the Go projection, its
// SQL twin (cross-checked row-for-row, exactly as the claim_state axis is), the
// three-gate conjunction, and the value that reaches the wire.
//
// The axis's authority is catalog_work.display_nsfw since the W1-pre nativization
// (refs/proj/140 §5b) — it was galgame.content_limit, read out of the wiki body per
// request, and these cases are the same cases re-pointed at the native column.
package service

import (
	"strings"
	"testing"

	"api/internal/platform/catalog/model"
	catsearch "api/internal/platform/catalog/search"
)

func setDisplayNSFW(t *testing.T, workID int64, nsfw bool) {
	t.Helper()
	if err := testDB.Exec(
		`UPDATE catalog_work SET display_nsfw = ? WHERE id = ?`, nsfw, workID).Error; err != nil {
		t.Fatalf("set display_nsfw on work %d: %v", workID, err)
	}
}

func declareDisplayLimit(t *testing.T, productWorkID int64, contentLimit string) {
	t.Helper()
	res := testDB.Exec(
		`UPDATE catalog_work SET display_nsfw = ? WHERE site = ? AND product_work_id = ?`,
		contentLimit == model.WikiContentLimitNSFW, siteGalgameWiki, productWorkID)
	if res.Error != nil {
		t.Fatalf("declare display limit for body %d: %v", productWorkID, res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("declare display limit for body %d touched %d rows, want 1 (is the work claimed?)",
			productWorkID, res.RowsAffected)
	}
}

type displayRow struct {
	name    string
	site    *string
	pwid    *int64
	display bool
	rating  int16
}

func displayLimitFixture(t *testing.T) (byLimit map[string][]int64, all []int64) {
	t.Helper()
	wiki, empty, letmoe := "galgame_wiki", "", "letmoe"
	pw := func(n int64) *int64 { return &n }

	rows := []displayRow{
		{"bodyless all_ages", nil, nil, false, model.ContentRatingAllAges},
		{"bodyless sensitive", nil, nil, false, model.ContentRatingSensitive},
		{"bodyless r18", nil, nil, false, model.ContentRatingR18},
		{"empty site is bodyless", &empty, pw(9401), false, model.ContentRatingR18},
		{"site without a product work id", &wiki, nil, false, model.ContentRatingR18},
		{"claimed r18 game, editorially sfw", &wiki, pw(9405), false, model.ContentRatingR18},
		{"claimed all_ages game, editorially nsfw", &wiki, pw(9406), true, model.ContentRatingAllAges},
		{"claimed r18 game, editorially nsfw", &wiki, pw(9407), true, model.ContentRatingR18},
		{"claimed, nothing declared", &wiki, pw(9408), false, model.ContentRatingR18},
		{"non-wiki claim of an r18 game", &letmoe, pw(9410), false, model.ContentRatingR18},
	}

	byLimit = map[string][]int64{}
	for _, r := range rows {
		w := createWorkX(t, galgameMediumID, r.rating, model.WorkStatusLive, r.name)
		setClaimColumns(t, w.ID, r.site, r.pwid, nil)
		if r.display {
			setDisplayNSFW(t, w.ID, true)
		}
		key := model.DisplayLimitKey(r.site, r.pwid, r.display, r.rating)
		byLimit[key] = append(byLimit[key], w.ID)
		all = append(all, w.ID)
	}
	return byLimit, all
}

func TestDisplayLimitWhereMatchesProjection(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)
	byLimit, all := displayLimitFixture(t)

	for _, lim := range []string{model.DisplayLimitKeySFW, model.DisplayLimitKeyNSFW} {
		if len(byLimit[lim]) == 0 {
			t.Fatalf("fixture covers no %q row", lim)
		}
	}

	seen := map[int64]int{}
	for lim, want := range byLimit {
		got := idSet(listIDs(t, WorksListFilter{Sort: "id", NSFW: true, DisplayLimits: []string{lim}}))
		if len(got) != len(want) {
			t.Fatalf("content_limit=%s selected %d rows, want %d (%v vs %v)", lim, len(got), len(want), got, want)
		}
		for _, id := range want {
			if !got[id] {
				t.Fatalf("content_limit=%s must select work %d (model.DisplayLimitKey says %s)", lim, id, lim)
			}
			seen[id]++
		}
	}
	for _, id := range all {
		if seen[id] != 1 {
			t.Fatalf("work %d matched %d display limits, want exactly 1", id, seen[id])
		}
	}
	if n := len(idSet(listIDs(t, WorksListFilter{
		Sort: "id", NSFW: true,
		DisplayLimits: []string{model.DisplayLimitKeySFW, model.DisplayLimitKeyNSFW},
	}))); n != len(all) {
		t.Fatalf("both values selected %d rows, want the whole set of %d", n, len(all))
	}
}

func TestWorksListDisplayLimitIsNotTheAgeAxis(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)

	safeR18 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "成人ゲーム・素材は安全")
	spicySFW := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "全年齢ゲーム・素材は成人")
	bodylessR18 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "無認領・成人")
	for i, id := range []int64{safeR18.ID, spicySFW.ID} {
		claimWork(t, id, "galgame_wiki", int64(9500+i))
	}
	declareDisplayLimit(t, 9500, "sfw")
	declareDisplayLimit(t, 9501, "nsfw")

	sfw := idSet(listIDs(t, WorksListFilter{
		Sort: "id", NSFW: true, DisplayLimits: []string{model.DisplayLimitKeySFW},
	}))
	if !sfw[safeR18.ID] {
		t.Fatalf("content_limit=sfw dropped the claimed r18 work with safe material — the exact 5,568-row incident")
	}
	if sfw[spicySFW.ID] {
		t.Fatalf("content_limit=sfw served an all_ages work the wiki marked nsfw — the reverse leak")
	}
	if sfw[bodylessR18.ID] {
		t.Fatalf("content_limit=sfw served a BODYLESS r18 work: with no editorial flag the rating is the only signal")
	}

	nsfw := idSet(listIDs(t, WorksListFilter{
		Sort: "id", NSFW: true, DisplayLimits: []string{model.DisplayLimitKeyNSFW},
	}))
	if !nsfw[spicySFW.ID] || !nsfw[bodylessR18.ID] || nsfw[safeR18.ID] {
		t.Fatalf("content_limit=nsfw = %v, want exactly the wiki-nsfw work and the bodyless r18 one", nsfw)
	}

	if n := len(idSet(listIDs(t, WorksListFilter{Sort: "id", NSFW: true}))); n != 3 {
		t.Fatalf("no content_limit selected %d rows, want all 3", n)
	}
}

func TestWorksListThreeGatesAreOrthogonal(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)

	safeLive := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "成人・安全・公開")
	safeDraft := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "成人・安全・下書き")
	spicyLive := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "成人・成人素材・公開")
	sfwLive := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "全年齢・安全・公開")
	for i, id := range []int64{safeLive.ID, safeDraft.ID, spicyLive.ID, sfwLive.ID} {
		claimWork(t, id, "galgame_wiki", int64(9600+i))
	}
	for id, limit := range map[int64]string{9600: "sfw", 9601: "sfw", 9602: "nsfw", 9603: "sfw"} {
		declareDisplayLimit(t, id, limit)
	}
	setClaimState(t, safeLive.ID, i16(model.ClaimStateLive))
	setClaimState(t, safeDraft.ID, i16(model.ClaimStateDraft))
	setClaimState(t, spicyLive.ID, i16(model.ClaimStateLive))
	setClaimState(t, sfwLive.ID, i16(model.ClaimStateLive))

	live := []string{model.ClaimStateKeyLive}
	sfwOnly := []string{model.DisplayLimitKeySFW}
	for _, tc := range []struct {
		name   string
		nsfw   bool
		limits []string
		states []string
		want   []int64
	}{
		{"no gate at all", true, nil, nil, []int64{safeLive.ID, safeDraft.ID, spicyLive.ID, sfwLive.ID}},
		{"age gate alone drops every r18 game", false, nil, nil, []int64{sfwLive.ID}},
		{"display gate alone keeps the safe r18 games", true, sfwOnly, nil, []int64{safeLive.ID, safeDraft.ID, sfwLive.ID}},
		{"display + claim gate", true, sfwOnly, live, []int64{safeLive.ID, sfwLive.ID}},
		{"the display gate never reopens the age gate", false, sfwOnly, nil, []int64{sfwLive.ID}},
		{"the claim gate never reopens the display gate", true, sfwOnly, live, []int64{safeLive.ID, sfwLive.ID}},
		{"nsfw display + live claim", true, []string{model.DisplayLimitKeyNSFW}, live, []int64{spicyLive.ID}},
	} {
		got := idSet(listIDs(t, WorksListFilter{
			Sort: "id", NSFW: tc.nsfw, DisplayLimits: tc.limits, ClaimStates: tc.states,
		}))
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %d rows %v, want %d %v", tc.name, len(got), got, len(tc.want), tc.want)
		}
		for _, id := range tc.want {
			if !got[id] {
				t.Fatalf("%s: work %d missing", tc.name, id)
			}
		}
	}

	f := WorksListFilter{Sort: "id", NSFW: true, DisplayLimits: sfwOnly, ClaimStates: live}
	walked := idSet(listIDs(t, f))
	onePage, err := newPublicSvc().WorksList(t.Context(), f, "", 100)
	if err != nil {
		t.Fatalf("WorksList single page: %v", err)
	}
	if len(onePage.Items) != len(walked) {
		t.Fatalf("single page served %d rows but the keyset walk served %d — the gate must not depend on paging",
			len(onePage.Items), len(walked))
	}
}

func TestClaimedByContentLimitOnEveryFace(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()
	ctx := t.Context()

	safeR18 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "LimitSafeR18")
	spicySFW := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "LimitSpicySFW")
	bodyless := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "LimitBodyless")
	claimWork(t, safeR18.ID, "galgame_wiki", 9700)
	claimWork(t, spicySFW.ID, "galgame_wiki", 9701)
	declareDisplayLimit(t, 9700, "sfw")
	declareDisplayLimit(t, 9701, "nsfw")
	setClaimState(t, spicySFW.ID, i16(model.ClaimStateDraft))

	want := map[int64]string{safeR18.ID: "sfw", spicySFW.ID: "nsfw"}

	page, err := svc.WorksList(ctx, WorksListFilter{Sort: "id", NSFW: true}, "", 50)
	if err != nil {
		t.Fatalf("WorksList: %v", err)
	}
	seen := 0
	for _, it := range page.Items {
		if it.ID == bodyless.ID {
			if it.ClaimedBy != nil {
				t.Fatalf("bodyless work got claimed_by %+v, want null", it.ClaimedBy)
			}
			continue
		}
		if it.ClaimedBy == nil || it.ClaimedBy.ContentLimit != want[it.ID] {
			t.Fatalf("list work %d claimed_by = %+v, want content_limit %q", it.ID, it.ClaimedBy, want[it.ID])
		}
		seen++
	}
	if seen != len(want) {
		t.Fatalf("list covered %d claimed works, want %d", seen, len(want))
	}

	for id, limit := range want {
		rec, found, err := svc.WorkDetail(ctx, id, PublicInclude{}, true, 0, PublicFields{})
		if err != nil || !found {
			t.Fatalf("WorkDetail %d: found=%v err=%v", id, found, err)
		}
		if rec.ClaimedBy == nil || rec.ClaimedBy.ContentLimit != limit {
			t.Fatalf("detail work %d claimed_by = %+v, want content_limit %q", id, rec.ClaimedBy, limit)
		}
	}

	addExternalRef(t, model.EntityTypeWork, safeR18.ID, srcVNDB, "v70501", model.LinkKindExact)
	single, found, err := svc.Lookup(ctx, "vndb", "v70501", true)
	if err != nil || !found {
		t.Fatalf("Lookup: found=%v err=%v", found, err)
	}
	if single.ClaimedBy == nil || single.ClaimedBy.ContentLimit != "sfw" {
		t.Fatalf("lookup claimed_by = %+v, want content_limit sfw", single.ClaimedBy)
	}
	if single.Work == nil || single.Work.ClaimedBy == nil || single.Work.ClaimedBy.ContentLimit != "sfw" {
		t.Fatalf("lookup brief claimed_by = %+v, want content_limit sfw", single.Work)
	}

	claims, err := svc.claimedByFor(ctx, []int64{safeR18.ID, spicySFW.ID, bodyless.ID})
	if err != nil {
		t.Fatalf("claimedByFor: %v", err)
	}
	if len(claims) != 2 {
		t.Fatalf("claimedByFor returned %d claims, want 2 (the bodyless row has none)", len(claims))
	}
	for id, limit := range want {
		if claims[id] == nil || claims[id].ContentLimit != limit {
			t.Fatalf("batch claim %d = %+v, want content_limit %q", id, claims[id], limit)
		}
	}
}

func TestCalendarDisplayLimitGateAndETag(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()
	ctx := t.Context()

	safe := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "CalSafe")
	spicy := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "CalSpicy")
	claimWork(t, safe.ID, "galgame_wiki", 9800)
	claimWork(t, spicy.ID, "galgame_wiki", 9801)
	declareDisplayLimit(t, 9800, "sfw")
	declareDisplayLimit(t, 9801, "nsfw")
	createRelease(t, safe.ID, 2024, 6, 14)
	createRelease(t, spicy.ID, 2025, 6, 14)

	june2024 := CalendarBucket{Kind: CalendarMonthBucket, Year: 2024, Month: 6}
	june2025 := CalendarBucket{Kind: CalendarMonthBucket, Year: 2025, Month: 6}
	ungated := CalendarFilter{NSFW: true}
	sfwOnly := CalendarFilter{NSFW: true, DisplayLimits: []string{model.DisplayLimitKeySFW}}

	for _, tc := range []struct {
		name   string
		bucket CalendarBucket
		f      CalendarFilter
		want   int64
	}{
		{"ungated june 2024", june2024, ungated, 1},
		{"ungated june 2025", june2025, ungated, 1},
		{"sfw-gated june 2024", june2024, sfwOnly, 1},
		{"sfw-gated june 2025", june2025, sfwOnly, 0},
	} {
		count, _, err := svc.CalendarMeta(ctx, tc.bucket, tc.f)
		if err != nil {
			t.Fatalf("%s: CalendarMeta: %v", tc.name, err)
		}
		if count != tc.want {
			t.Fatalf("%s: count = %d, want %d", tc.name, count, tc.want)
		}
		page, err := svc.CalendarPage(ctx, tc.bucket, tc.f, "", 50)
		if err != nil {
			t.Fatalf("%s: CalendarPage: %v", tc.name, err)
		}
		if int64(len(page.Items)) != tc.want {
			t.Fatalf("%s: page carries %d rows but the count says %d — one gate, two queries",
				tc.name, len(page.Items), tc.want)
		}
	}

	_, maxOrd, found, err := svc.CalendarBounds(ctx, sfwOnly)
	if err != nil || !found {
		t.Fatalf("CalendarBounds: found=%v err=%v", found, err)
	}
	if maxOrd != 202406_00 {
		t.Fatalf("sfw-gated max month = %d, want 202406_00 (the nsfw 2025 work is outside this population)", maxOrd)
	}

	ungatedCount, ungatedMax, err := svc.CalendarMeta(ctx, june2024, ungated)
	if err != nil {
		t.Fatalf("CalendarMeta ungated: %v", err)
	}
	gatedCount, gatedMax, err := svc.CalendarMeta(ctx, june2024, sfwOnly)
	if err != nil {
		t.Fatalf("CalendarMeta gated: %v", err)
	}
	if ungatedCount != gatedCount {
		t.Fatalf("the two populations must have EQUAL counts here (%d vs %d) — that is what makes the key load-bearing",
			ungatedCount, gatedCount)
	}
	a := CalendarETag("month-2024-06", ungated.PopulationKey(), ungatedCount, ungatedMax)
	b := CalendarETag("month-2024-06", sfwOnly.PopulationKey(), gatedCount, gatedMax)
	if a == b {
		t.Fatalf("ungated and content_limit=sfw share the ETag %s — a cross-gate cache smear", a)
	}
	nsfwOnly := CalendarFilter{NSFW: true, DisplayLimits: []string{model.DisplayLimitKeyNSFW}}
	if sfwOnly.PopulationKey() == nsfwOnly.PopulationKey() {
		t.Fatalf("sfw and nsfw share the population key %q", sfwOnly.PopulationKey())
	}
}

func TestDisplayLimitVocabularyIsClosed(t *testing.T) {
	for _, ok := range []string{"sfw", "nsfw"} {
		if !IsDisplayLimit(ok) {
			t.Fatalf("%q must be a legal content_limit token", ok)
		}
	}
	for _, bad := range []string{"", "all", "SFW", "NSFW", "r18", "safe", "true"} {
		if IsDisplayLimit(bad) {
			t.Fatalf("%q must NOT be a legal content_limit token", bad)
		}
	}
}

func TestDisplayLimitFilterCompilation(t *testing.T) {
	if got := catsearch.MeiliFilter((WorksSearchFilter{}).worksFilter("")); strings.Contains(got, "content_limit") {
		t.Fatalf("no content_limit param must emit no clause: %q", got)
	}

	one := catsearch.MeiliFilter(WorksSearchFilter{DisplayLimits: []string{model.DisplayLimitKeySFW}}.worksFilter(""))
	if !strings.Contains(one, "(content_limit = 'sfw')") {
		t.Fatalf("single content_limit clause = %q", one)
	}
	if !strings.Contains(one, "content_rating != 2") {
		t.Fatalf("content_limit must not replace the other clauses: %q", one)
	}

	both := catsearch.MeiliFilter(WorksSearchFilter{
		DisplayLimits: []string{model.DisplayLimitKeySFW, model.DisplayLimitKeyNSFW},
	}.worksFilter(""))
	if !strings.Contains(both, "(content_limit = 'sfw' OR content_limit = 'nsfw')") {
		t.Fatalf("multi content_limit clause = %q", both)
	}
	all := catsearch.MeiliFilter(WorksSearchFilter{
		DisplayLimits: []string{model.DisplayLimitKeySFW},
		ClaimStates:   []string{model.ClaimStateKeyLive},
	}.worksFilter(""))
	for _, want := range []string{"content_rating != 2", "(claim_state = 'live')", "(content_limit = 'sfw')"} {
		if !strings.Contains(all, want) {
			t.Fatalf("three-gate expression %q is missing %q", all, want)
		}
	}
}

func TestWorksSearchDisplayLimitGate(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)
	cleanTaxonomyTables(t)
	idx := worksSearchIndexer(t)
	svc := newPublicSvc().WithWorksSearch(idx)

	safeR18 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "検索・成人・安全素材")
	spicySFW := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "検索・全年齢・成人素材")
	bodylessR18 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "検索・無認領・成人")
	claimWork(t, safeR18.ID, "galgame_wiki", 9900)
	claimWork(t, spicySFW.ID, "galgame_wiki", 9901)
	declareDisplayLimit(t, 9900, "sfw")
	declareDisplayLimit(t, 9901, "nsfw")

	docs := make([]catsearch.WorkDocInput, 0, 3)
	for _, w := range []struct {
		id   int64
		name string
	}{
		{safeR18.ID, "検索・成人・安全素材"}, {spicySFW.ID, "検索・全年齢・成人素材"},
		{bodylessR18.ID, "検索・無認領・成人"},
	} {
		var row struct {
			Site          *string `gorm:"column:site"`
			ProductWorkID *int64  `gorm:"column:product_work_id"`
			ClaimState    *int16  `gorm:"column:claim_state"`
			ContentRating int16   `gorm:"column:content_rating"`
			DisplayNSFW   bool    `gorm:"column:display_nsfw"`
		}
		if err := testDB.Raw(`
			SELECT w.site, w.product_work_id, w.claim_state, w.content_rating, w.display_nsfw
			FROM catalog_work w WHERE w.id = ?`, w.id).Scan(&row).Error; err != nil {
			t.Fatalf("read claim columns: %v", err)
		}
		docs = append(docs, catsearch.WorkDocInput{
			ID: w.id, DisplayName: w.name, OLang: "ja",
			ContentRating: row.ContentRating,
			Claimed:       row.Site != nil && *row.Site != "",
			ClaimState:    model.ClaimStateKey(row.Site, row.ProductWorkID, row.ClaimState),
			ContentLimit:  model.DisplayLimitKey(row.Site, row.ProductWorkID, row.DisplayNSFW, row.ContentRating),
			UpdatedTS:     1700000000,
		})
	}
	indexWorks(t, idx, docs)

	for _, tc := range []struct {
		limits []string
		want   []int64
	}{
		{nil, []int64{safeR18.ID, spicySFW.ID, bodylessR18.ID}},
		{[]string{model.DisplayLimitKeySFW}, []int64{safeR18.ID}},
		{[]string{model.DisplayLimitKeyNSFW}, []int64{spicySFW.ID, bodylessR18.ID}},
		{[]string{model.DisplayLimitKeySFW, model.DisplayLimitKeyNSFW}, []int64{safeR18.ID, spicySFW.ID, bodylessR18.ID}},
	} {
		data, err := svc.WorksSearch(t.Context(), WorksSearchFilter{
			Sort: "id", NSFW: true, DisplayLimits: tc.limits, Limit: 50,
		})
		if err != nil {
			t.Fatalf("content_limit=%v: WorksSearch: %v", tc.limits, err)
		}
		got := make(map[int64]bool, len(data.Items))
		for _, it := range data.Items {
			got[it.ID] = true
		}
		if len(got) != len(tc.want) {
			t.Fatalf("content_limit=%v: got %d items %v, want %d", tc.limits, len(got), got, len(tc.want))
		}
		for _, id := range tc.want {
			if !got[id] {
				t.Fatalf("content_limit=%v: work %d missing from %v", tc.limits, id, got)
			}
		}
		if int(data.Total) != len(data.Items) {
			t.Fatalf("content_limit=%v: total=%d but page carries %d rows — total and items must share one filter",
				tc.limits, data.Total, len(data.Items))
		}
	}
}
