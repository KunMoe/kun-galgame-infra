package service

import (
	"testing"

	"api/internal/platform/catalog/model"
)

func setOLang(t *testing.T, workID int64, olang string) {
	t.Helper()
	if err := testDB.Exec(`UPDATE catalog_work SET olang = ? WHERE id = ?`, olang, workID).Error; err != nil {
		t.Fatalf("set olang: %v", err)
	}
}

func calIDs(t *testing.T, svc *PublicService, b CalendarBucket, f CalendarFilter) []int64 {
	t.Helper()
	page, err := svc.CalendarPage(t.Context(), b, f, "", 100)
	if err != nil {
		t.Fatalf("CalendarPage %+v: %v", b, err)
	}
	out := make([]int64, len(page.Items))
	for i, it := range page.Items {
		out[i] = it.ID
	}
	return out
}

func june2024() CalendarBucket {
	return CalendarBucket{Kind: CalendarMonthBucket, Year: 2024, Month: 6}
}

func TestCalendarPrecisionToBucket(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()

	day := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "DayPrecision")
	createRelease(t, day.ID, 2024, 6, 14)
	month := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "MonthPrecision")
	createRelease(t, month.ID, 2024, 6, 0)
	year := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "YearPrecision")
	createRelease(t, year.ID, 2024, 0, 0)
	tba := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "TBA")
	createRelease(t, tba.ID, 0, 0, 0)
	unknown := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "NoRelease")

	got := calIDs(t, svc, june2024(), CalendarFilter{})
	if len(got) != 2 || got[0] != month.ID || got[1] != day.ID {
		t.Fatalf("june bucket = %v, want [%d(month-precision) %d(day-precision)]", got, month.ID, day.ID)
	}

	got = calIDs(t, svc, CalendarBucket{Kind: CalendarPendingBucket, Year: 2024}, CalendarFilter{})
	if len(got) != 1 || got[0] != year.ID {
		t.Fatalf("2024 pending = %v, want [%d]", got, year.ID)
	}

	got = calIDs(t, svc, CalendarBucket{Kind: CalendarTBABucket}, CalendarFilter{})
	if len(got) != 1 || got[0] != tba.ID {
		t.Fatalf("tba bucket = %v, want only [%d] (unknown work %d must stay out)", got, tba.ID, unknown.ID)
	}

	for m := 1; m <= 12; m++ {
		for _, id := range calIDs(t, svc, CalendarBucket{Kind: CalendarMonthBucket, Year: 2024, Month: m}, CalendarFilter{}) {
			if id == year.ID {
				t.Fatalf("year-precision work %d leaked into month 2024-%02d", year.ID, m)
			}
		}
	}
	for _, b := range []CalendarBucket{june2024(), {Kind: CalendarPendingBucket, Year: 2024}} {
		for _, id := range calIDs(t, svc, b, CalendarFilter{}) {
			if id == tba.ID || id == unknown.ID {
				t.Fatalf("undated work %d appeared in %+v", id, b)
			}
		}
	}
}

func TestCalendarMonthBoundaries(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()

	first := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "June1")
	createRelease(t, first.ID, 2024, 6, 1)
	last := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "June30")
	createRelease(t, last.ID, 2024, 6, 30)
	prevMonth := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "May31")
	createRelease(t, prevMonth.ID, 2024, 5, 31)
	nextMonth := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "July1")
	createRelease(t, nextMonth.ID, 2024, 7, 1)
	ported := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "Ported")
	createRelease(t, ported.ID, 2024, 6, 20)
	createRelease(t, ported.ID, 2024, 5, 2)

	got := calIDs(t, svc, june2024(), CalendarFilter{})
	if len(got) != 2 || got[0] != first.ID || got[1] != last.ID {
		t.Fatalf("june = %v, want [%d %d] (May 31 / July 1 / the May-anchored port must stay out)", got, first.ID, last.ID)
	}

	may := calIDs(t, svc, CalendarBucket{Kind: CalendarMonthBucket, Year: 2024, Month: 5}, CalendarFilter{})
	if len(may) != 2 || may[0] != ported.ID || may[1] != prevMonth.ID {
		t.Fatalf("may = %v, want [%d(2024-05-02) %d(2024-05-31)]", may, ported.ID, prevMonth.ID)
	}
	page, err := svc.WorksList(t.Context(), WorksListFilter{Sort: "id", IDs: []int64{ported.ID}}, "", 10)
	if err != nil {
		t.Fatalf("WorksList: %v", err)
	}
	if page.Items[0].ReleaseDate == nil || *page.Items[0].ReleaseDate != "2024-05-02" {
		t.Fatalf("ported release_date = %v, want 2024-05-02 (the bucket anchor)", page.Items[0].ReleaseDate)
	}

	if err := testDB.Exec(`UPDATE catalog_release SET deleted_at = now() WHERE work_id = ? AND released_m = 5`, ported.ID).Error; err != nil {
		t.Fatalf("soft delete release: %v", err)
	}
	got = calIDs(t, svc, june2024(), CalendarFilter{})
	if len(got) != 3 {
		t.Fatalf("june after soft delete = %v, want the port to have moved in", got)
	}
}

func TestCalendarPopulationGates(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()

	ja := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "JaWork")
	createRelease(t, ja.ID, 2024, 6, 5)
	zhHans := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "ZhHansWork")
	setOLang(t, zhHans.ID, "zh-Hans")
	createRelease(t, zhHans.ID, 2024, 6, 6)
	zhHant := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "ZhHantWork")
	setOLang(t, zhHant.ID, "zh-Hant")
	createRelease(t, zhHant.ID, 2024, 6, 7)
	en := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "EnWork")
	setOLang(t, en.ID, "en")
	createRelease(t, en.ID, 2024, 6, 8)
	r18 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "R18Work")
	createRelease(t, r18.ID, 2024, 6, 9)
	stub := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusStub, "StubWork")
	createRelease(t, stub.ID, 2024, 6, 10)
	asmr := createWorkX(t, 5, model.ContentRatingAllAges, model.WorkStatusLive, "AsmrWork")
	createRelease(t, asmr.ID, 2024, 6, 11)

	got := calIDs(t, svc, june2024(), CalendarFilter{})
	if len(got) != 3 || got[0] != ja.ID || got[1] != zhHans.ID || got[2] != zhHant.ID {
		t.Fatalf("default population = %v, want [ja %d, zh-Hans %d, zh-Hant %d]", got, ja.ID, zhHans.ID, zhHant.ID)
	}

	got = calIDs(t, svc, june2024(), CalendarFilter{NSFW: true})
	if len(got) != 4 || got[3] != r18.ID {
		t.Fatalf("nsfw population = %v, want the r18 work %d to join", got, r18.ID)
	}

	got = calIDs(t, svc, june2024(), CalendarFilter{OLang: PublicOLang{All: true}})
	if len(got) != 4 || got[3] != en.ID {
		t.Fatalf("olang=all = %v, want the en work %d to join and r18 to stay out", got, en.ID)
	}

	got = calIDs(t, svc, june2024(), CalendarFilter{OLang: PublicOLang{Values: []string{"en"}}})
	if len(got) != 1 || got[0] != en.ID {
		t.Fatalf("olang=en = %v, want only %d", got, en.ID)
	}
	got = calIDs(t, svc, june2024(), CalendarFilter{OLang: PublicOLang{Values: []string{"ja", "en"}}})
	if len(got) != 2 || got[0] != ja.ID || got[1] != en.ID {
		t.Fatalf("olang=ja,en = %v, want [%d %d]", got, ja.ID, en.ID)
	}
	if got = calIDs(t, svc, june2024(), CalendarFilter{OLang: PublicOLang{Values: []string{"nope"}}}); len(got) != 0 {
		t.Fatalf("unknown olang = %v, want an empty bucket", got)
	}

	if err := testDB.Exec(`UPDATE catalog_work SET deleted_at = now() WHERE id = ?`, ja.ID).Error; err != nil {
		t.Fatalf("soft delete work: %v", err)
	}
	for _, id := range calIDs(t, svc, june2024(), CalendarFilter{OLang: PublicOLang{All: true}}) {
		if id == ja.ID {
			t.Fatalf("soft-deleted work %d still in the bucket", ja.ID)
		}
	}
}

func TestCalendarKeysetAndCursorLanes(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()

	a := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "CalA")
	createRelease(t, a.ID, 2024, 6, 3)
	b := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "CalB")
	createRelease(t, b.ID, 2024, 6, 3)
	c := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "CalC")
	createRelease(t, c.ID, 2024, 6, 20)
	pendA := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "PendA")
	createRelease(t, pendA.ID, 2024, 0, 0)
	pendB := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "PendB")
	createRelease(t, pendB.ID, 2024, 0, 0)
	tbaA := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "TbaA")
	createRelease(t, tbaA.ID, 0, 0, 0)
	tbaB := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "TbaB")
	createRelease(t, tbaB.ID, 0, 0, 0)

	var walked []int64
	cursor := ""
	for range 4 {
		page, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{}, cursor, 1)
		if err != nil {
			t.Fatalf("month page: %v", err)
		}
		for _, it := range page.Items {
			walked = append(walked, it.ID)
		}
		if page.NextCursor == nil {
			break
		}
		cursor = *page.NextCursor
	}
	if len(walked) != 3 || walked[0] != a.ID || walked[1] != b.ID || walked[2] != c.ID {
		t.Fatalf("month keyset walk = %v, want [%d %d %d]", walked, a.ID, b.ID, c.ID)
	}

	p1, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{}, "", 1)
	if err != nil || p1.NextCursor == nil {
		t.Fatalf("month p1: cursor=%v err=%v", p1.NextCursor, err)
	}
	p2, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{}, *p1.NextCursor, 1)
	if err != nil || len(p2.Items) != 1 || p2.Items[0].ID != b.ID {
		t.Fatalf("month p2 = %+v err=%v, want the same-date sibling %d", p2.Items, err, b.ID)
	}

	pendPage, err := svc.CalendarPage(t.Context(), CalendarBucket{Kind: CalendarPendingBucket, Year: 2024}, CalendarFilter{}, "", 1)
	if err != nil || pendPage.NextCursor == nil {
		t.Fatalf("pending p1: cursor=%v err=%v", pendPage.NextCursor, err)
	}
	tbaPage, err := svc.CalendarPage(t.Context(), CalendarBucket{Kind: CalendarTBABucket}, CalendarFilter{}, "", 1)
	if err != nil || tbaPage.NextCursor == nil {
		t.Fatalf("tba p1: cursor=%v err=%v", tbaPage.NextCursor, err)
	}

	if _, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{}, "!!!not-base64!!!", 20); err != ErrBadCursor {
		t.Fatalf("malformed calendar cursor = %v, want ErrBadCursor", err)
	}
	cross := []struct {
		name   string
		bucket CalendarBucket
		cursor string
	}{
		{"month cursor on pending", CalendarBucket{Kind: CalendarPendingBucket, Year: 2024}, *p1.NextCursor},
		{"month cursor on tba", CalendarBucket{Kind: CalendarTBABucket}, *p1.NextCursor},
		{"pending cursor on month", june2024(), *pendPage.NextCursor},
		{"tba cursor on month", june2024(), *tbaPage.NextCursor},
	}
	for _, tc := range cross {
		if _, err := svc.CalendarPage(t.Context(), tc.bucket, CalendarFilter{}, tc.cursor, 20); err != ErrBadCursor {
			t.Fatalf("%s = %v, want ErrBadCursor", tc.name, err)
		}
	}
	worksPage, err := svc.WorksList(t.Context(), WorksListFilter{Sort: "id"}, "", 1)
	if err != nil || worksPage.NextCursor == nil {
		t.Fatalf("works p1: %v", err)
	}
	if _, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{}, *worksPage.NextCursor, 20); err != ErrBadCursor {
		t.Fatalf("works cursor on the calendar lane = %v, want ErrBadCursor", err)
	}
	if _, err := svc.WorksList(t.Context(), WorksListFilter{Sort: "id"}, *p1.NextCursor, 20); err != ErrBadCursor {
		t.Fatalf("calendar cursor on the works lane = %v, want ErrBadCursor", err)
	}
	if _, err := svc.EnginesList(t.Context(), EnginesListFilter{}, *p1.NextCursor, 20); err != ErrBadCursor {
		t.Fatalf("calendar cursor on the engines lane = %v, want ErrBadCursor", err)
	}
}

func TestCalendarMetaTracksTheBucket(t *testing.T) {
	cleanTables(t)
	svc := newPublicSvc()

	w1 := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "MetaA")
	createRelease(t, w1.ID, 2024, 6, 4)
	w2 := createWorkX(t, galgameMediumID, model.ContentRatingR18, model.WorkStatusLive, "MetaR18")
	createRelease(t, w2.ID, 2024, 6, 5)

	count, maxUpdated, err := svc.CalendarMeta(t.Context(), june2024(), CalendarFilter{})
	if err != nil {
		t.Fatalf("CalendarMeta: %v", err)
	}
	if count != 1 {
		t.Fatalf("sfw meta count = %d, want 1 (r18 excluded)", count)
	}
	if maxUpdated.IsZero() {
		t.Fatal("meta max_updated must carry the bucket's newest stamp")
	}
	nsfwCount, _, err := svc.CalendarMeta(t.Context(), june2024(), CalendarFilter{NSFW: true})
	if err != nil {
		t.Fatalf("CalendarMeta nsfw: %v", err)
	}
	if nsfwCount != 2 {
		t.Fatalf("nsfw meta count = %d, want 2", nsfwCount)
	}

	page, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{NSFW: true}, "", 1)
	if err != nil {
		t.Fatalf("CalendarPage: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page = %d items, want 1", len(page.Items))
	}

	emptyCount, emptyMax, err := svc.CalendarMeta(t.Context(), CalendarBucket{Kind: CalendarMonthBucket, Year: 1990, Month: 1}, CalendarFilter{})
	if err != nil {
		t.Fatalf("CalendarMeta empty: %v", err)
	}
	if emptyCount != 0 || emptyMax.Unix() != 0 {
		t.Fatalf("empty bucket meta = (%d, %v), want (0, epoch)", emptyCount, emptyMax)
	}

	before := CalendarETag("month-2024-06", CalendarFilter{}.PopulationKey(), count, maxUpdated)
	if err := testDB.Exec(`UPDATE catalog_work SET updated_at = now() + interval '1 hour' WHERE id = ?`, w1.ID).Error; err != nil {
		t.Fatalf("touch work: %v", err)
	}
	count2, max2, err := svc.CalendarMeta(t.Context(), june2024(), CalendarFilter{})
	if err != nil {
		t.Fatalf("CalendarMeta after touch: %v", err)
	}
	if after := CalendarETag("month-2024-06", CalendarFilter{}.PopulationKey(), count2, max2); after == before {
		t.Fatalf("etag did not move after a member work was touched: %s", after)
	}
}

func TestCalendarItemsAreWorksListItems(t *testing.T) {
	cleanTables(t)
	cleanTagTables(t)
	cleanTaxonomyTables(t)
	svc := newPublicSvcCDN()

	w := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "CalRich")
	createRelease(t, w.ID, 2024, 6, 12)
	addWorkTitle(t, w.ID, "ja", "六月の作品", 0)
	addWorkIntro(t, w.ID, "ja", "紹介文", srcVNDB, 0)
	addWorkCover(t, w.ID, hash64("ca11"), 0, "main", true, 0, srcVNDB)
	addWorkRating(t, w.ID, srcVNDB, 8.1, 300)
	addWorkLabel(t, w.ID, "CalBrand", model.LabelKindGameBrand, model.WorkLabelKindBrand)

	page, err := svc.CalendarPage(t.Context(), june2024(), CalendarFilter{}, "", 20)
	if err != nil {
		t.Fatalf("CalendarPage: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page = %d items, want 1", len(page.Items))
	}
	item := page.Items[0]
	if item.DisplayName != "CalRich" || item.OLang != "ja" || item.ContentRating != "all_ages" || item.Medium == "" {
		t.Fatalf("calendar item is not a works-list item: %+v", item)
	}
	if item.ReleaseDate == nil || *item.ReleaseDate != "2024-06-12" {
		t.Fatalf("release_date = %v, want the bucket anchor 2024-06-12", item.ReleaseDate)
	}
	if item.Cover == "" {
		t.Fatal("calendar item must carry the representative cover the works list carries")
	}
	if item.Localized != nil || item.Intros != nil || item.Labels != nil || item.Ratings != nil || item.Covers != nil {
		t.Fatalf("no include= must mean no blocks: %+v", item)
	}

	inc := ParseWorksListInclude("names,intros,labels,ratings,covers")
	page, err = svc.CalendarPage(t.Context(), june2024(), CalendarFilter{Include: inc}, "", 20)
	if err != nil {
		t.Fatalf("CalendarPage include: %v", err)
	}
	item = page.Items[0]
	if item.Localized["ja"].Value != "六月の作品" {
		t.Fatalf("include=names on the calendar: %+v", item.Localized)
	}
	if len(item.Intros) != 1 || item.Intros[0].Lang != "ja" || item.Intros[0].Intro != "紹介文" {
		t.Fatalf("include=intros on the calendar: %+v", item.Intros)
	}
	if len(item.Labels) != 1 || item.Labels[0].DisplayName != "CalBrand" {
		t.Fatalf("include=labels on the calendar: %+v", item.Labels)
	}
	if len(item.Ratings) != 1 {
		t.Fatalf("include=ratings on the calendar: %+v", item.Ratings)
	}
	if item.Covers == nil || item.Covers.Portrait == nil {
		t.Fatalf("include=covers on the calendar: %+v", item.Covers)
	}
}
