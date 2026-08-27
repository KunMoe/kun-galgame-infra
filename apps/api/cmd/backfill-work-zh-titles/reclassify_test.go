package main

import (
	"os"
	"path/filepath"
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeIDsFile(t *testing.T, ids ...int64) string {
	t.Helper()
	body := "# forum galgame ids\n"
	for _, id := range ids {
		body += itoa(id) + "\n"
	}
	path := filepath.Join(t.TempDir(), "ids.txt")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestReclassifyKeepsSourceBackedTitles(t *testing.T) {
	clean(t)
	ctx := t.Context()

	// bangumi publishes this exact string, so the row is not a translation
	// the forum account made — it stays source.
	backedByBgm := mkClaimedWork(t, "被 bgm 背书", 4695)
	mkTitle(t, backedByBgm, "ja", "そら", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	bgmKept := mkTitle(t, backedByBgm, "zh-Hans", "天空", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkAnchor(t, backedByBgm, "bangumi", "500")
	mkSubject(t, 500, "そら", "天空")

	// vndb publishes a different Chinese title for this work, so the forum's
	// own string is unbacked and becomes machine.
	backedByVndb := mkClaimedWork(t, "被 vndb 部分背书", 4696)
	vndbKept := mkTitle(t, backedByVndb, "zh-Hant", "繁體標題", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	vndbFlagged := mkTitle(t, backedByVndb, "zh-Hans", "论坛自己的写法", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkAnchor(t, backedByVndb, "vndb", "v50")
	mkRelease(t, "r50", "v50", langZhHant, "繁體標題", false)

	// No anchor at all: nothing backs it, so it is machine.
	unbacked := mkClaimedWork(t, "无锚", 4700)
	unbackedFlagged := mkTitle(t, unbacked, "zh-Hans", "无来源中文", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	// Not in the id list — must not be touched.
	outsider := mkClaimedWork(t, "名单之外", 9999)
	outsiderRow := mkTitle(t, outsider, "zh-Hans", "别动我", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	ids := writeIDsFile(t, 4695, 4696, 4700)

	require.NoError(t, runReclassify(ctx, testDB, ids, false, 0))
	assert.Equal(t, model.WorkTitleProvenanceSource, provenanceOf(t, unbackedFlagged), "dry run writes nothing")

	require.NoError(t, runReclassify(ctx, testDB, ids, true, 0))
	assert.Equal(t, model.WorkTitleProvenanceSource, provenanceOf(t, bgmKept))
	assert.Equal(t, model.WorkTitleProvenanceSource, provenanceOf(t, vndbKept))
	assert.Equal(t, model.WorkTitleProvenanceMachine, provenanceOf(t, vndbFlagged))
	assert.Equal(t, model.WorkTitleProvenanceMachine, provenanceOf(t, unbackedFlagged))
	assert.Equal(t, model.WorkTitleProvenanceSource, provenanceOf(t, outsiderRow), "a work outside the id list is untouched")

	// Second run: everything is already decided, so nothing is updated.
	rows, err := loadReclassifyRows(ctx, testDB, []int64{4695, 4696, 4700})
	require.NoError(t, err)
	for _, r := range rows {
		if !r.SourceBacked {
			assert.Equal(t, model.WorkTitleProvenanceMachine, r.Provenance)
		}
	}
	require.NoError(t, runReclassify(ctx, testDB, ids, true, 0))
	assert.Equal(t, model.WorkTitleProvenanceSource, provenanceOf(t, bgmKept))
}

// A work with no ja title keeps its (now machine-flagged) Chinese title: it
// never enters the MT population, so nothing will replace it.
func TestReclassifiedWorkWithoutJaTitleStaysOutOfTheMTPopulation(t *testing.T) {
	clean(t)
	ctx := t.Context()

	noJa := mkClaimedWork(t, "没有日文原名", 4695)
	row := mkTitle(t, noJa, "zh-Hans", "只有中文", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	withJa := mkClaimedWork(t, "有日文原名", 4696)
	mkTitle(t, withJa, "ja", "そら", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkTitle(t, withJa, "zh-Hans", "论坛中文", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	require.NoError(t, runReclassify(ctx, testDB, writeIDsFile(t, 4695, 4696), true, 0))
	assert.Equal(t, model.WorkTitleProvenanceMachine, provenanceOf(t, row))

	cands, err := loadMTCandidates(ctx, testDB, 0)
	require.NoError(t, err)
	ids := map[int64]bool{}
	for _, c := range cands {
		ids[c.WorkID] = true
	}
	assert.False(t, ids[noJa], "no ja title, nothing to re-translate from")
	require.True(t, ids[withJa])

	var reflagged mtCandidate
	for _, c := range cands {
		if c.WorkID == withJa {
			reflagged = c
		}
	}
	dec, _ := decide(reflagged)
	assert.Equal(t, decRetrans, dec, "the reclassified row carries no src_hash, so it is due for re-translation")
}

func provenanceOf(t *testing.T, titleID int64) int16 {
	t.Helper()
	var p int16
	require.NoError(t, testDB.Raw(`SELECT provenance FROM catalog_work_title WHERE id = ?`, titleID).Scan(&p).Error)
	return p
}
