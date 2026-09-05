package hihyou

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"api/internal/platform/news/model"
	"api/internal/platform/news/newstest"
	"api/internal/testsupport/dbtest"
	"api/pkg/config"
	"api/pkg/imageclient"

	"gorm.io/gorm"
)

func writeCorpus(t *testing.T, dir string, arts ...*Article) {
	t.Helper()
	c := Corpus{Dir: dir}
	if err := c.Mkdirs(); err != nil {
		t.Fatal(err)
	}
	for _, a := range arts {
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Write(c.ArticlePath(a.Data.ID), b); err != nil {
			t.Fatal(err)
		}
	}
}

// healthy is four items under two sections. The third heading carries only a
// picture, which is real upstream shape and must be dropped per item rather than
// stored as a row with an empty preview.
func healthy(cv int64) *Article {
	a := article("【Gal周报200期】x",
		text(17, true, "新闻日期：2026年8月1日~2026年8月8日"),
		text(17, false, "新作资讯"),
		text(17, true, "1.《A》情报公开"), text(17, false, body),
		picture("http://i0.hdslb.com/bfs/new_dyn/a.png"),
		text(17, true, "2.《B》发售日决定"), text(17, false, body),
		text(17, false, "汉化情报"),
		text(17, true, "3.《C》汉化发布"), text(17, false, body),
		text(17, true, "4.《D》图集"), picture("//i0.hdslb.com/bfs/article/d.png"),
	)
	a.Data.ID = cv
	a.Data.PublishTime = 1786257779
	return a
}

func openTestDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	db, release, ok := newstest.Open()
	if !ok {
		dbtest.Skipf(t, "news test database unavailable")
	}
	t.Cleanup(release)
	if err := newstest.Truncate(db); err != nil {
		t.Fatal(err)
	}
	dsn, _ := dbtest.DSN()
	return db, dsn
}

func TestImportWritesPendingItemsAndIsIdempotent(t *testing.T) {
	db, dsn := openTestDB(t)
	dir := t.TempDir()
	// 期201 holds a single item, which the gate refuses. It must contribute no
	// rows at all — a mis-segmented issue is quarantined whole, never partially
	// ingested, because its rows look plausible one at a time.
	broken := article("【Gal周报201期】x", text(17, false, "新作资讯"),
		text(17, true, "《只有一条》"), text(17, false, body))
	broken.Data.ID = 999
	writeCorpus(t, dir, healthy(1001), broken)

	opts := Opts{Dir: dir, NoImages: true, DSN: dsn}
	cfg := &config.Config{}
	ctx := context.Background()

	dry, err := Import(ctx, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if dry.WouldCreate != 3 || dry.Created != 0 {
		t.Fatalf("dry run: would_create=%d created=%d, want 3/0", dry.WouldCreate, dry.Created)
	}
	if dry.Passing != 1 || len(dry.Quarantined) != 1 {
		t.Fatalf("dry run: passing=%d quarantined=%d, want 1/1", dry.Passing, len(dry.Quarantined))
	}
	var n int64
	db.Model(&model.NewsItem{}).Count(&n)
	if n != 0 {
		t.Fatalf("dry run wrote %d rows", n)
	}

	opts.Apply = true
	applied, err := Import(ctx, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Created != 3 || applied.DroppedNoBody != 1 {
		t.Fatalf("apply: created=%d dropped=%d, want 3/1", applied.Created, applied.DroppedNoBody)
	}

	var rows []model.NewsItem
	if err := db.Where("source_key = ?", model.SourceKeyHihyou).
		Order("external_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("stored %d rows, want 3", len(rows))
	}
	for _, r := range rows {
		if r.Status != model.StatusPending {
			t.Errorf("%s: status %d — the backfill must not bypass the gate", r.ExternalID, r.Status)
		}
		if r.SourceURL != SourceURL(1001) {
			t.Errorf("%s: source_url %q", r.ExternalID, r.SourceURL)
		}
		if r.Lane != model.LaneNews {
			t.Errorf("%s: lane %q — an extracted item is not the column itself", r.ExternalID, r.Lane)
		}
	}
	if rows[0].ExternalID != ExternalID(1001, 1) {
		t.Errorf("external_id = %q", rows[0].ExternalID)
	}
	// The serial prefix is stripped: a published item's "1." has no referent.
	if rows[0].Title != "《A》情报公开" || rows[0].UpstreamCategory != "新作资讯" {
		t.Errorf("first row = %q / %q", rows[0].Title, rows[0].UpstreamCategory)
	}
	if rows[2].UpstreamCategory != "汉化情报" {
		t.Errorf("third row section = %q", rows[2].UpstreamCategory)
	}

	// Re-running must recognise every row. The identity is (cv, ordinal), so a
	// second import that created anything would mean the segmentation drifted.
	again, err := Import(ctx, cfg, opts)
	if err != nil {
		t.Fatal(err)
	}
	if again.Created != 0 || again.Updated != 0 || again.Unchanged != 3 {
		t.Fatalf("re-import: created=%d updated=%d unchanged=%d, want 0/0/3",
			again.Created, again.Updated, again.Unchanged)
	}
}

func TestPublishActorReleasesPendingRowsAndSparesYmgal(t *testing.T) {
	db, dsn := openTestDB(t)
	// The ymgal source row is seeded by the migration and survives Truncate, so
	// only the guard item is created here.
	guard := model.NewsItem{SourceKey: model.SourceKeyYmgal, Lane: model.LaneNews,
		ExternalID: "t1", Title: "t", Preview: "p", SourceURL: "https://www.ymgal.games/t1",
		PublishedAt: time.Unix(1786257779, 0).UTC(), Status: model.StatusPending}
	if err := db.Create(&guard).Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	writeCorpus(t, dir, healthy(1005))
	sum, err := Import(context.Background(), &config.Config{},
		Opts{Dir: dir, NoImages: true, Apply: true, DSN: dsn, PublishActor: 30})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Created != 3 || sum.Released != 3 {
		t.Fatalf("created=%d released=%d, want 3/3", sum.Created, sum.Released)
	}

	var n int64
	db.Model(&model.NewsItem{}).
		Where("source_key = ? AND status = ?", model.SourceKeyHihyou, model.StatusPublished).Count(&n)
	if n != 3 {
		t.Errorf("published hihyou rows = %d, want 3", n)
	}
	var decisions []model.NewsModerationDecision
	if err := db.Find(&decisions).Error; err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 3 {
		t.Fatalf("decision rows = %d, want one per released item", len(decisions))
	}
	for _, d := range decisions {
		if d.ActorUID != 30 || d.FromStatus != model.StatusPending || d.ToStatus != model.StatusPublished || d.Reason == "" {
			t.Errorf("decision = %+v", d)
		}
	}
	// The release is scoped to this source: ymgal's queue answers a different
	// promise and a wholesale sweep here would silently bypass it.
	if err := db.Take(&guard, guard.ID).Error; err != nil {
		t.Fatal(err)
	}
	if guard.Status != model.StatusPending {
		t.Errorf("ymgal row status = %d — the standing release swept another source", guard.Status)
	}
}

// A row that already exists with the right text is the case 04c actually meets:
// the whole 04b backfill ran with --no-images. If pictures only ever arrive on
// create or on a text change, that run leaves 4,425 rows without a banner
// forever and reports success while uploading nothing.
func TestPicturesArriveForRowsWhoseTextDidNotChange(t *testing.T) {
	db, _ := openTestDB(t)
	// Three pictures under one heading: a banner plus a gallery, which is the
	// shape that makes the two storage paths distinguishable.
	a := article("【Gal周报202期】x",
		text(17, false, "新作资讯"),
		text(17, true, "1.《A》情报公开"), text(17, false, body),
		picture("http://i0.hdslb.com/bfs/new_dyn/a.png"),
		picture("//i0.hdslb.com/bfs/article/b.png"),
		picture("https://i0.hdslb.com/bfs/article/c.png"),
		text(17, true, "2.《B》发售日决定"), text(17, false, body),
		text(17, true, "3.《C》汉化发布"), text(17, false, body),
	)
	a.Data.ID = 1003
	a.Data.PublishTime = 1786257779
	seg := Segment(a)
	it := seg.Items[0]
	if len(it.Pictures) != 3 {
		t.Fatalf("fixture yields %d pictures", len(it.Pictures))
	}
	published := time.Unix(1786257779, 0).UTC()
	ctx := context.Background()

	dry := &writer{db: db, opts: Opts{Apply: true, NoImages: true}, uploaded: map[string]string{}}
	if err := dry.applyItem(ctx, 1003, published, it, &stats{}); err != nil {
		t.Fatal(err)
	}
	var row model.NewsItem
	if err := db.Where("external_id = ?", ExternalID(1003, it.Ordinal)).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.BannerHash != "" {
		t.Fatalf("--no-images stored a banner %q", row.BannerHash)
	}

	// A pre-seeded upload cache stands in for the image host: picture() consults
	// it before the client, so nothing here reaches the network.
	hashes := map[string]string{}
	for _, u := range it.Pictures {
		hashes[u] = "hash-" + path.Base(u)
	}
	st := &stats{}
	live := &writer{db: db, opts: Opts{Apply: true}, uploaded: hashes,
		images: &imageclient.Client{}}
	if err := live.applyItem(ctx, 1003, published, it, st); err != nil {
		t.Fatal(err)
	}
	if st.unchanged != 1 || st.updated != 0 {
		t.Fatalf("unchanged=%d updated=%d — the text must not have changed", st.unchanged, st.updated)
	}
	if err := db.Where("external_id = ?", ExternalID(1003, it.Ordinal)).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.BannerHash != hashes[it.Pictures[0]] {
		t.Errorf("banner_hash = %q, want the first picture's hash", row.BannerHash)
	}
	var n int64
	db.Model(&model.NewsItemImage{}).Where("item_id = ?", row.ID).Count(&n)
	if want := int64(len(it.Pictures) - 1); n != want {
		t.Errorf("stored %d gallery rows, want %d", n, want)
	}

	// And a third run must not fetch them again: origin URLs already on disk are
	// what decides the work, so an empty cache would fail on a network attempt.
	blank := &writer{db: db, opts: Opts{Apply: true}, uploaded: map[string]string{},
		images: &imageclient.Client{}}
	if err := blank.applyItem(ctx, 1003, published, it, &stats{}); err != nil {
		t.Fatal(err)
	}
	db.Model(&model.NewsItemImage{}).Where("item_id = ?", row.ID).Count(&n)
	if want := int64(len(it.Pictures) - 1); n != want {
		t.Errorf("after re-run: %d gallery rows, want %d", n, want)
	}
}

// warm is what makes the 8,080-picture pass concurrent, so it is also what
// would re-download the whole corpus on a second run. Its first job is to seed
// the cache from what is already stored: every URL resolved here is a fetch
// that does not happen, and the writer holds no image client, so a fetch would
// fail the test rather than quietly succeed.
func TestWarmResolvesStoredPicturesWithoutFetching(t *testing.T) {
	db, _ := openTestDB(t)
	a := article("【Gal周报203期】x",
		text(17, false, "新作资讯"),
		text(17, true, "1.《A》情报公开"), text(17, false, body),
		picture("https://i0.hdslb.com/bfs/article/w1.png"),
		picture("https://i0.hdslb.com/bfs/article/w2.png"),
		text(17, true, "2.《B》发售日决定"), text(17, false, body),
		text(17, true, "3.《C》汉化发布"), text(17, false, body),
	)
	a.Data.ID = 1004
	a.Data.PublishTime = 1786257779
	it := Segment(a).Items[0]
	ctx := context.Background()

	seeded := map[string]string{it.Pictures[0]: "hash-w1", it.Pictures[1]: "hash-w2"}
	first := &writer{db: db, opts: Opts{Apply: true}, uploaded: seeded,
		failed: map[string]bool{}, images: &imageclient.Client{}}
	if err := first.applyItem(ctx, 1004, time.Unix(1786257779, 0).UTC(), it, &stats{}); err != nil {
		t.Fatal(err)
	}

	second := &writer{db: db, opts: Opts{Apply: true, Concurrency: 4},
		uploaded: map[string]string{}, failed: map[string]bool{}, images: &imageclient.Client{}}
	st := &stats{}
	second.warm(ctx, it.Pictures, st)
	if st.imagesUp != 0 || st.imagesFail != 0 {
		t.Errorf("warm fetched %d / failed %d — everything was already stored", st.imagesUp, st.imagesFail)
	}
	for _, u := range it.Pictures {
		if second.uploaded[u] != seeded[u] {
			t.Errorf("%s resolved to %q, want %q", u, second.uploaded[u], seeded[u])
		}
	}
}

func TestImportSeedsSourceRow(t *testing.T) {
	// One newstest.Open per test and no more: the suite lock is a session-level
	// advisory lock, so a second handle inside the same test waits on the first
	// one forever rather than failing.
	db, dsn := openTestDB(t)
	dir := t.TempDir()
	writeCorpus(t, dir, healthy(1002))

	if _, err := Import(context.Background(), &config.Config{},
		Opts{Dir: dir, NoImages: true, Apply: true, DSN: dsn}); err != nil {
		t.Fatal(err)
	}
	var src model.NewsSource
	if err := db.Where("key = ?", model.SourceKeyHihyou).Take(&src).Error; err != nil {
		t.Fatal(err)
	}
	// 「注明出处就行」 is the whole permission: a row published without the
	// attribution and the column link breaks the only condition she set.
	if src.Attribution == "" || src.ColumnURL == "" || src.PublisherUID != 115235 {
		t.Errorf("source row = %+v", src)
	}
}

func TestCorpusTreatsRateLimitedFileAsAbsent(t *testing.T) {
	dir := t.TempDir()
	c := Corpus{Dir: dir}
	if err := c.Mkdirs(); err != nil {
		t.Fatal(err)
	}
	// A stored -509 envelope parses fine and would otherwise count as harvested,
	// which is what turns a second-chance pass into a no-op that reports success.
	if err := os.WriteFile(filepath.Join(dir, "article", "cv7.json"),
		[]byte(`{"code":-509,"message":"请求过于频繁","data":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if c.Has(7) {
		t.Error("a rate-limited response was accepted as a harvested article")
	}
	entries, bad, err := c.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 || len(bad) != 1 {
		t.Errorf("load: %d entries, %d incomplete", len(entries), len(bad))
	}
}
