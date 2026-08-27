package main

import (
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBgmLaneFillsOnlyWorksWithNoChineseTitle(t *testing.T) {
	clean(t)
	ctx := t.Context()

	empty := mkWork(t, "空")
	mkTitle(t, empty, "ja", "からっぽ", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkAnchor(t, empty, "bangumi", "100")
	mkSubject(t, 100, "からっぽ", "空空如也")

	hasHant := mkWork(t, "已有繁體")
	mkTitle(t, hasHant, "zh-Hant", "已有繁體", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkAnchor(t, hasHant, "bangumi", "101")
	mkSubject(t, 101, "ある", "已有简体")

	noNameCN := mkWork(t, "无中文名")
	mkAnchor(t, noNameCN, "bangumi", "102")
	mkSubject(t, 102, "なまえ", "")

	rows, err := loadBgmSupply(ctx, testDB, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the work with no Chinese title at all is supplied")
	assert.Equal(t, empty, rows[0].WorkID)
	assert.Equal(t, langZhHans, rows[0].Lang)
	assert.Equal(t, "空空如也", rows[0].Title)

	require.NoError(t, runSource(ctx, testDB, "bgm", true, 0, 0))
	got := titlesOf(t, empty)
	require.Len(t, got, 2)
	assert.Equal(t, "空空如也", got[1].Title)
	assert.Equal(t, langZhHans, got[1].Lang)
	assert.Equal(t, model.WorkTitleKindOfficial, got[1].Kind)
	assert.Equal(t, model.WorkTitleProvenanceSource, got[1].Provenance)
	assert.Nil(t, got[1].SrcHash, "a source row carries no translation provenance")
	assert.Nil(t, got[1].MTModel)

	// Second run: the work now has a zh title, so it leaves the candidate set
	// entirely and nothing is written.
	rows, err = loadBgmSupply(ctx, testDB, 0)
	require.NoError(t, err)
	assert.Empty(t, rows, "second run must find nothing to do")
	require.NoError(t, runSource(ctx, testDB, "bgm", true, 0, 0))
	assert.Len(t, titlesOf(t, empty), 2)
}

func TestVndbLaneIsPerLangAndPicksTheAgreedTitle(t *testing.T) {
	clean(t)
	ctx := t.Context()

	both := mkWork(t, "两个语种都缺")
	mkAnchor(t, both, "vndb", "v10")
	mkRelease(t, "r3", "v10", langZhHans, "少数派简体", false)
	mkRelease(t, "r4", "v10", langZhHans, "多数派简体", false)
	mkRelease(t, "r5", "v10", langZhHans, "多数派简体", false)
	mkRelease(t, "r6", "v10", langZhHant, "繁體標題", false)
	mkRelease(t, "r7", "v10", langZhHant, "機翻標題", true)

	hantOnly := mkWork(t, "只缺繁體")
	mkTitle(t, hantOnly, "zh-Hans", "已有简体", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkAnchor(t, hantOnly, "vndb", "v11")
	mkRelease(t, "r8", "v11", langZhHans, "另一个简体", false)
	mkRelease(t, "r9", "v11", langZhHant, "另一個繁體", false)

	rows, err := loadVndbSupply(ctx, testDB, 0)
	require.NoError(t, err)
	got := map[string]string{}
	for _, r := range rows {
		got[key(r.WorkID, r.Lang)] = r.Title
	}
	assert.Equal(t, map[string]string{
		key(both, langZhHans):     "多数派简体",
		key(both, langZhHant):     "繁體標題",
		key(hantOnly, langZhHant): "另一個繁體",
	}, got, "mtl titles are excluded, the majority title wins, and a filled slot is left alone")

	require.NoError(t, runSource(ctx, testDB, "vndb", true, 0, 0))
	assert.Len(t, titlesOf(t, both), 2)
	assert.Len(t, titlesOf(t, hantOnly), 2)

	rows, err = loadVndbSupply(ctx, testDB, 0)
	require.NoError(t, err)
	assert.Empty(t, rows, "second run must find nothing to do")
	require.NoError(t, runSource(ctx, testDB, "vndb", true, 0, 0))
	assert.Len(t, titlesOf(t, both), 2)
}

func TestSourceLaneSupersedesMachineRows(t *testing.T) {
	clean(t)
	ctx := t.Context()

	// The 605-forum-set shape: the only zh row is a machine translation, and
	// bangumi publishes a different human name_cn. The machine row must not
	// block the gate, and apply must replace it, not sit beside it.
	mtOnly := mkWork(t, "机翻占位")
	mkTitle(t, mtOnly, "zh-Hans", "机翻标题", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine)
	mkAnchor(t, mtOnly, "bangumi", "400")
	mkSubject(t, 400, "げんさく", "人工中文名")

	blocked := mkWork(t, "已有源行")
	mkTitle(t, blocked, "zh-Hans", "出版社中文名", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkAnchor(t, blocked, "bangumi", "401")
	mkSubject(t, 401, "べつ", "别的中文名")

	rows, err := loadBgmSupply(ctx, testDB, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1, "machine rows do not block, source rows do")
	assert.Equal(t, mtOnly, rows[0].WorkID)

	require.NoError(t, runSource(ctx, testDB, "bgm", true, 0, 0))
	got := titlesOf(t, mtOnly)
	require.Len(t, got, 1, "the machine row is replaced, not kept beside the source row")
	assert.Equal(t, "人工中文名", got[0].Title)
	assert.Equal(t, model.WorkTitleProvenanceSource, got[0].Provenance)

	rows, err = loadBgmSupply(ctx, testDB, 0)
	require.NoError(t, err)
	assert.Empty(t, rows, "second run must find nothing to do")
}

func TestVndbLaneSupersedesMachineOnlyInItsOwnSlot(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "繁體供給")
	mkTitle(t, w, "zh-Hans", "机翻简体", model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine)
	mkAnchor(t, w, "vndb", "v30")
	mkRelease(t, "r300", "v30", langZhHant, "人工繁體", false)

	rows, err := loadVndbSupply(ctx, testDB, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, langZhHant, rows[0].Lang)

	require.NoError(t, runSource(ctx, testDB, "vndb", true, 0, 0))
	got := titlesOf(t, w)
	require.Len(t, got, 2, "the machine zh-Hans row survives a zh-Hant-only supply")
	assert.Equal(t, "机翻简体", got[0].Title)
	assert.Equal(t, model.WorkTitleProvenanceMachine, got[0].Provenance)
	assert.Equal(t, "人工繁體", got[1].Title)
	assert.Equal(t, model.WorkTitleProvenanceSource, got[1].Provenance)
}

func TestVndbLaneSkipsReleaseDecoratedTitles(t *testing.T) {
	clean(t)
	ctx := t.Context()

	// The decorated string outnumbers the clean one; the filter must beat the
	// agree count, not merely tie-break it.
	rescued := mkWork(t, "有干净兄弟")
	mkAnchor(t, rescued, "vndb", "v40")
	mkRelease(t, "r400", "v40", langZhHant, "魔女的夜宴 - FHD Edition", false)
	mkRelease(t, "r401", "v40", langZhHant, "魔女的夜宴 - FHD Edition", false)
	mkRelease(t, "r402", "v40", langZhHant, "魔女的夜宴", false)

	skipped := mkWork(t, "只有修饰名")
	mkAnchor(t, skipped, "vndb", "v41")
	mkRelease(t, "r410", "v41", langZhHant, "悠之空 - 18+ 補丁", false)

	rows, err := loadVndbSupply(ctx, testDB, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the decorated-only work is skipped entirely")
	assert.Equal(t, rescued, rows[0].WorkID)
	assert.Equal(t, "魔女的夜宴", rows[0].Title)
}

func TestVndbLaneRequiresHanAndSingleVNReleases(t *testing.T) {
	clean(t)
	ctx := t.Context()

	// vndb zh release rows also carry romanizations and plain Latin strings;
	// neither is a Chinese name. And a release spanning two VNs carries the
	// bundle's title, not either VN's — prod grew "告别回憶 歷代回憶錄 限量版"
	// on nine works that way before the guard existed.
	latinOnly := mkWork(t, "拉丁行")
	mkAnchor(t, latinOnly, "vndb", "v80")
	mkRelease(t, "r800", "v80", langZhHans, "Rewrite", false)
	mkRelease(t, "r801", "v80", langZhHans, "Gangtie Nüyou 2nd", false)

	bundled := mkWork(t, "被合集碟盖名")
	mkAnchor(t, bundled, "vndb", "v81")
	other := mkWork(t, "合集碟另一部")
	mkAnchor(t, other, "vndb", "v82")
	for _, rid := range []string{"r810", "r812"} {
		mkRelease(t, rid, "v81", langZhHans, "两部曲精选", false)
		require.NoError(t, testDB.Exec(
			`INSERT INTO src_vndb.releases_vn (id, vid, rtype) VALUES (?, 'v82', 'complete')`, rid).Error)
	}
	mkRelease(t, "r811", "v81", langZhHans, "第一部真名", false)

	rows, err := loadVndbSupply(ctx, testDB, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1, "no Han, no supply; a bundle title never counts even when it out-agrees")
	assert.Equal(t, bundled, rows[0].WorkID)
	assert.Equal(t, "第一部真名", rows[0].Title)
}

func TestVndbLaneBreaksTiesOnTheEarliestRelease(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "平手")
	mkAnchor(t, w, "vndb", "v20")
	mkRelease(t, "r200", "v20", langZhHans, "后来的写法", false)
	mkRelease(t, "r30", "v20", langZhHans, "最早的写法", false)

	rows, err := loadVndbSupply(ctx, testDB, 0)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "最早的写法", rows[0].Title, "r30 sorts before r200 numerically, not lexically")
}

func TestSourceLaneDryRunWritesNothing(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "干跑")
	mkAnchor(t, w, "bangumi", "300")
	mkSubject(t, 300, "からっぽ", "干跑")

	require.NoError(t, runSource(ctx, testDB, "bgm", false, 0, 0))
	assert.Empty(t, titlesOf(t, w))
}

func key(id int64, lang string) string {
	return lang + ":" + itoa(id)
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
