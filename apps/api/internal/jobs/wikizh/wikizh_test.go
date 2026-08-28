package wikizh

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	if dsn, ok := dbtest.DSN(); ok {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "SKIP db tests: cannot connect: %v\n", err)
		default:
			if err := migrate.Run(db); err != nil {
				fmt.Fprintf(os.Stderr, "SKIP db tests: catalog migrate failed: %v\n", err)
			} else if err := seed.Run(db); err != nil {
				fmt.Fprintf(os.Stderr, "SKIP db tests: catalog seed failed: %v\n", err)
			} else {
				testDB = db
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "SKIP db tests: TEST_DATABASE_DSN is unset")
	}
	os.Exit(m.Run())
}

func requireDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		dbtest.Skipf(t, "the catalog test database is unavailable")
	}
}

func ensureSnapshot(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec(`CREATE SCHEMA IF NOT EXISTS src_wiki`).Error)
	require.NoError(t, testDB.Exec(`CREATE TABLE IF NOT EXISTS src_wiki.intro_snapshot (
		work_id bigint PRIMARY KEY, galgame_id bigint NOT NULL, site text, claim_state smallint,
		published boolean NOT NULL,
		wiki_zh_cn text NOT NULL DEFAULT '', wiki_zh_tw text NOT NULL DEFAULT '',
		wiki_ja text NOT NULL DEFAULT '', wiki_en text NOT NULL DEFAULT '',
		catalog_ja text NOT NULL DEFAULT '', catalog_en text NOT NULL DEFAULT '',
		catalog_zh_source text NOT NULL DEFAULT '', catalog_zh_mt text NOT NULL DEFAULT '',
		captured_at timestamptz NOT NULL DEFAULT now())`).Error)
	require.NoError(t, testDB.Exec(`TRUNCATE src_wiki.intro_snapshot`).Error)
}

func clean(t *testing.T) {
	t.Helper()
	requireDB(t)
	for _, tbl := range []string{"catalog_work_intro", "catalog_work"} {
		require.NoError(t, testDB.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
	ensureSnapshot(t)
}

var nextProductID int64 = 700000

func mkWork(t *testing.T, published bool) int64 {
	t.Helper()
	var medium int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_medium WHERE key='galgame'`).Scan(&medium).Error)
	site := "kungal"
	nextProductID++
	pid := nextProductID
	w := model.CatalogWork{MediumID: medium, OLang: "ja", DisplayName: "w", Site: &site, ProductWorkID: &pid}
	if !published {
		draft := model.ClaimStateDraft
		w.ClaimState = &draft
	}
	require.NoError(t, testDB.Create(&w).Error)
	return w.ID
}

func mkSnapshot(t *testing.T, workID int64, published bool, wikiZh, catalogJa, catalogMT, catalogHuman string) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO src_wiki.intro_snapshot
		(work_id, galgame_id, site, published, wiki_zh_cn, catalog_ja, catalog_zh_mt, catalog_zh_source)
		VALUES (?,?,?,?,?,?,?,?)`,
		workID, workID, "kungal", published, wikiZh, catalogJa, catalogMT, catalogHuman).Error)
}

func TestBucketsAreDisjointAndScoped(t *testing.T) {
	clean(t)
	ctx := context.Background()

	wNoZh := mkWork(t, true)
	mkSnapshot(t, wNoZh, true, "用户手写的完整中文简介。", "日本語のあらすじ。", "", "")

	wMT := mkWork(t, true)
	mkSnapshot(t, wMT, true, "用户手写版本。", "日本語のあらすじ。", "机器翻译版本。", "")

	wHuman := mkWork(t, true)
	mkSnapshot(t, wHuman, true, "用户手写版本。", "日本語。", "", "后来有人写的中文。")

	wDraft := mkWork(t, false)
	mkSnapshot(t, wDraft, false, "草稿海的中文。", "日本語。", "", "")

	wEmpty := mkWork(t, true)
	mkSnapshot(t, wEmpty, true, "   ", "日本語。", "", "")

	usable, err := LoadCandidates(ctx, testDB, BucketUsable, 0)
	require.NoError(t, err)
	require.Len(t, usable, 1, "only the published, wiki-having, zh-less work")
	assert.Equal(t, wNoZh, usable[0].WorkID)
	assert.Equal(t, "ja", usable[0].SourceLang)
	assert.Equal(t, "日本語のあらすじ。", usable[0].Source)

	compare, err := LoadCandidates(ctx, testDB, BucketCompare, 0)
	require.NoError(t, err)
	require.Len(t, compare, 1, "only the work whose slot a machine row holds")
	assert.Equal(t, wMT, compare[0].WorkID)
	assert.Equal(t, "机器翻译版本。", compare[0].MachineZh)

	assert.NotEqual(t, usable[0].WorkID, compare[0].WorkID)
}

func TestApplyIsPurelyAdditive(t *testing.T) {
	clean(t)
	ctx := context.Background()
	var curated, bangumi int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key='curated'`).Scan(&curated).Error)
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key='bangumi'`).Scan(&bangumi).Error)

	w := mkWork(t, true)
	mkSnapshot(t, w, true, "用户手写的完整中文简介。", "日本語のあらすじ。", "机器翻译版本。", "")
	require.NoError(t, testDB.Create(&model.CatalogWorkIntro{
		WorkID: w, Lang: "zh-Hans", Intro: "机器翻译版本。", SourceID: bangumi,
		Provenance: 1, MTModel: "glm", SrcHash: "abc"}).Error)

	vs := []Verdict{{Key: fmt.Sprintf("w%d", w), WorkID: w, Bucket: BucketCompare,
		Verdict: VerdictABetter, Confidence: 0.95}}

	st, err := Apply(ctx, testDB, vs, false)
	require.NoError(t, err)
	assert.Equal(t, 1, st.Restores)
	assert.Zero(t, st.Written)

	st, err = Apply(ctx, testDB, vs, true)
	require.NoError(t, err)
	assert.Equal(t, 1, st.Written)
	assert.Len(t, st.ReceiptIDs, 1, "receipts identify exactly the row written")

	var mt model.CatalogWorkIntro
	require.NoError(t, testDB.Where("work_id=? AND provenance=1", w).First(&mt).Error)
	assert.Equal(t, "机器翻译版本。", mt.Intro)
	assert.Equal(t, "glm", mt.MTModel)
	var human model.CatalogWorkIntro
	require.NoError(t, testDB.Where("work_id=? AND provenance=0", w).First(&human).Error)
	assert.Equal(t, "用户手写的完整中文简介。", human.Intro)
	assert.Equal(t, curated, human.SourceID)

	st, err = Apply(ctx, testDB, vs, true)
	require.NoError(t, err)
	assert.Zero(t, st.Written)
	assert.Equal(t, 1, st.Skipped, "a human row now exists, so the work is no longer eligible")
}

func TestGateAndVocabulary(t *testing.T) {
	clean(t)
	ctx := context.Background()

	wLow := mkWork(t, true)
	mkSnapshot(t, wLow, true, "边缘案例的中文。", "日本語。", "", "")
	wBogus := mkWork(t, true)
	mkSnapshot(t, wBogus, true, "另一段中文。", "日本語。", "", "")
	wNo := mkWork(t, true)
	mkSnapshot(t, wNo, true, "残片", "日本語。", "", "")

	vs := []Verdict{
		{WorkID: wLow, Bucket: BucketUsable, Verdict: VerdictUsable, Confidence: 0.85},
		{WorkID: wBogus, Bucket: BucketUsable, Verdict: "definitely_keep", Confidence: 0.99},
		{WorkID: wNo, Bucket: BucketUsable, Verdict: VerdictUnusable, Confidence: 0.97},
	}
	st, err := Apply(ctx, testDB, vs, true)
	require.NoError(t, err)
	assert.Zero(t, st.Written, "nothing here may be written")
	assert.Equal(t, 1, st.BelowGate, "low confidence is held for human review")
	assert.Equal(t, 1, st.Invalid, "an invented verdict cannot cause a write")
	assert.Equal(t, 1, st.Restores, "only the low-confidence one even asked to restore")

	var n int64
	require.NoError(t, testDB.Raw(`SELECT count(*) FROM catalog_work_intro`).Scan(&n).Error)
	assert.EqualValues(t, 0, n)
}

func TestSnapshotMissingIsAPrecondition(t *testing.T) {
	requireDB(t)
	require.NoError(t, testDB.Exec(`DROP TABLE IF EXISTS src_wiki.intro_snapshot`).Error)
	_, err := LoadCandidates(context.Background(), testDB, BucketUsable, 0)
	require.Error(t, err)
	assert.IsType(t, SnapshotMissingError{}, err)
	ensureSnapshot(t)
}

func TestUserPacketShape(t *testing.T) {
	c := Candidate{WorkID: 42, Bucket: BucketCompare, SourceLang: "ja",
		Source: "原文", WikiZh: "甲", MachineZh: "乙"}
	p := UserPacket(c)
	assert.Contains(t, p, "### key: w42", "the key must round-trip so a chunked reply can be reassembled")
	assert.Contains(t, p, "A(用户手写中文)")
	assert.Contains(t, p, "B(机器翻译中文)")

	p = UserPacket(Candidate{WorkID: 7, Bucket: BucketUsable, Source: "原文", WikiZh: "甲"})
	assert.NotContains(t, p, "B(机器翻译中文)")
}

func TestConsensusRequiresUnanimity(t *testing.T) {
	r := func(id int64, v string, c float64) Verdict {
		return Verdict{Key: fmt.Sprintf("w%d", id), WorkID: id, Bucket: BucketCompare, Verdict: v, Confidence: c}
	}
	rounds := [][]Verdict{
		{r(1, VerdictABetter, 0.95), r(2, VerdictABetter, 0.95), r(3, VerdictABetter, 0.95), r(4, VerdictABetter, 0.95)},
		{r(1, VerdictABetter, 0.92), r(2, VerdictBBetter, 0.90), r(3, VerdictABetter, 0.80), r(4, VerdictABetter, 0.95)},
		{r(1, VerdictABetter, 0.99), r(2, VerdictABetter, 0.95), r(3, VerdictABetter, 0.95)},
	}
	got, st := Consensus(rounds)
	assert.Equal(t, 3, st.Rounds)
	assert.Equal(t, 4, st.Works)
	assert.Equal(t, 2, st.Unanimous, "works 1 and 3 agreed in every round")
	assert.Equal(t, 1, st.Contested, "work 2 flipped")
	assert.Equal(t, 1, st.Incomplete, "work 4 is missing a round")

	by := map[int64]Verdict{}
	for _, v := range got {
		by[v.WorkID] = v
	}
	assert.Equal(t, VerdictABetter, by[1].Verdict)
	assert.InDelta(t, 0.92, by[1].Confidence, 0.001)
	assert.Equal(t, VerdictABetter, by[3].Verdict)
	assert.InDelta(t, 0.80, by[3].Confidence, 0.001, "the 0.80 round drags it under the gate")

	for _, id := range []int64{2, 4} {
		assert.Equal(t, VerdictUnsure, by[id].Verdict)
		assert.Zero(t, by[id].Confidence)
		assert.NotEmpty(t, by[id].Reason, "the review pile needs to know WHY it landed there")
	}

	clean(t)
	for _, id := range []int64{1, 2, 3, 4} {
		w := mkWork(t, true)
		by[id] = Verdict{WorkID: w, Bucket: by[id].Bucket, Verdict: by[id].Verdict, Confidence: by[id].Confidence}
		mkSnapshot(t, w, true, "候选中文。", "日本語。", "机翻。", "")
	}
	var folded []Verdict
	for _, id := range []int64{2, 3, 4} {
		folded = append(folded, by[id])
	}
	stApply, err := Apply(context.Background(), testDB, folded, true)
	require.NoError(t, err)
	assert.Zero(t, stApply.Written, "disagreement, incompleteness and a sub-gate fold all block the write")
}

func TestConsensusFoldsOnDirection(t *testing.T) {
	r := func(id int64, v string, c float64) Verdict {
		return Verdict{Key: fmt.Sprintf("w%d", id), WorkID: id, Bucket: BucketCompare, Verdict: v, Confidence: c}
	}
	rounds := [][]Verdict{
		{r(1, VerdictABetter, 0.95), r(2, VerdictABetter, 0.95), r(3, VerdictABetter, 0.95), r(4, VerdictBBetter, 0.95), r(5, VerdictUnsure, 0.5)},
		{r(1, VerdictABetter, 0.93), r(2, VerdictEquivalent, 0.9), r(3, VerdictBBetter, 0.95), r(4, VerdictEquivalent, 0.9), r(5, VerdictEquivalent, 0.4)},
		{r(1, VerdictEquivalent, 0.10), r(2, VerdictUnsure, 0.5), r(3, VerdictABetter, 0.95), r(4, VerdictBBetter, 0.95), r(5, VerdictUnsure, 0.5)},
	}
	got, st := Consensus(rounds)
	assert.Equal(t, 1, st.Leaning, "work 1: a majority wanted it, one abstained, none objected")
	assert.Equal(t, 1, st.Contested, "only work 3 has rounds pointing opposite ways")
	assert.Equal(t, 1, st.Declined, "work 4: nobody wanted it — no write, and no human either")
	assert.Equal(t, 2, st.Abstained, "works 2 and 5")
	assert.Zero(t, st.Unanimous)

	by := map[int64]Verdict{}
	for _, v := range got {
		by[v.WorkID] = v
	}
	assert.Equal(t, VerdictABetter, by[1].Verdict)
	assert.InDelta(t, 0.93, by[1].Confidence, 0.001)
	assert.Equal(t, VerdictUnsure, by[2].Verdict)
	assert.Zero(t, by[2].Confidence)
	assert.Equal(t, VerdictBBetter, by[4].Verdict)
	assert.False(t, restores(BucketCompare, by[4].Verdict))
}

func TestSwapVerdictIsAnInvolution(t *testing.T) {
	for _, v := range []string{VerdictABetter, VerdictBBetter, VerdictEquivalent, VerdictUnsure} {
		assert.Equal(t, v, SwapVerdict(SwapVerdict(v)), v)
	}
	assert.Equal(t, VerdictBBetter, SwapVerdict(VerdictABetter))
	assert.Equal(t, VerdictABetter, SwapVerdict(VerdictBBetter))
	assert.Equal(t, VerdictEquivalent, SwapVerdict(VerdictEquivalent))
	assert.Equal(t, VerdictUnsure, SwapVerdict(VerdictUnsure))
}

func TestAdversarialPacketSwapsOnlyCompare(t *testing.T) {
	c := Candidate{WorkID: 1, Bucket: BucketCompare, Source: "原文", WikiZh: "USERTEXT", MachineZh: "MACHINETEXT"}
	swapped := AdversarialPacket(c)
	a := strings.Index(swapped, "MACHINETEXT")
	b := strings.Index(swapped, "USERTEXT")
	assert.Positive(t, a)
	assert.Positive(t, b)
	assert.Less(t, a, b, "under swap the machine text must be presented first")

	u := Candidate{WorkID: 2, Bucket: BucketUsable, Source: "原文", WikiZh: "USERTEXT"}
	assert.Equal(t, UserPacket(u), AdversarialPacket(u))
}

func TestTiebreakIsScoped(t *testing.T) {
	v := func(id int64, verdict string, c float64) Verdict {
		return Verdict{Key: fmt.Sprintf("w%d", id), WorkID: id, Bucket: BucketCompare, Verdict: verdict, Confidence: c}
	}
	folded := []Verdict{
		v(1, VerdictABetter, 0.95),
		v(2, VerdictUnsure, 0),
		v(3, VerdictUnsure, 0),
		v(4, VerdictUnsure, 0),
		v(5, VerdictUnsure, 0),
	}
	tie := []Verdict{
		v(1, VerdictBBetter, 0.99),
		v(2, VerdictABetter, 0.93),
		v(3, VerdictBBetter, 0.91),
		v(4, VerdictUnsure, 0.5),
	}
	got, st := Tiebreak(folded, tie)
	by := map[int64]Verdict{}
	for _, g := range got {
		by[g.WorkID] = g
	}
	assert.Equal(t, VerdictABetter, by[1].Verdict, "a settled work is not the tiebreak's to reopen")
	assert.InDelta(t, 0.95, by[1].Confidence, 0.001)
	assert.Equal(t, VerdictABetter, by[2].Verdict)
	assert.Equal(t, VerdictBBetter, by[3].Verdict)
	assert.Equal(t, VerdictUnsure, by[4].Verdict)
	assert.Equal(t, VerdictUnsure, by[5].Verdict)

	assert.Equal(t, 4, st.Eligible)
	assert.Equal(t, 1, st.ResolvedFor)
	assert.Equal(t, 1, st.ResolvedAgainst)
	assert.Equal(t, 1, st.StillUnsure)
	assert.Equal(t, 1, st.NoTiebreak)
}
