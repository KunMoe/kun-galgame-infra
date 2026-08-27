package main

import (
	"os"
	"path/filepath"
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMachineTitleInsertsAndReplaces(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "こころ")
	mkTitle(t, w, "ja", "こころ", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	res, err := writeMachineTitle(ctx, testDB, w, "心", "hash-1", "glm-test")
	require.NoError(t, err)
	assert.Equal(t, writeInserted, res)
	rows := titlesOf(t, w)
	require.Len(t, rows, 2)
	assert.Equal(t, "心", rows[1].Title)
	assert.Equal(t, langZhHans, rows[1].Lang)
	assert.Equal(t, model.WorkTitleProvenanceMachine, rows[1].Provenance)
	require.NotNil(t, rows[1].SrcHash)
	assert.Equal(t, "hash-1", *rows[1].SrcHash)
	require.NotNil(t, rows[1].MTModel)
	assert.Equal(t, "glm-test", *rows[1].MTModel)

	// Same title, same hash: the lane has nothing to do.
	res, err = writeMachineTitle(ctx, testDB, w, "心", "hash-1", "glm-test")
	require.NoError(t, err)
	assert.Equal(t, writeUnchanged, res)
	assert.Len(t, titlesOf(t, w), 2)

	// The ja title changed under the row, so the stored translation is of a
	// title that is no longer there — replace it rather than accumulate.
	res, err = writeMachineTitle(ctx, testDB, w, "心之所向", "hash-2", "glm-test")
	require.NoError(t, err)
	assert.Equal(t, writeReplaced, res)
	rows = titlesOf(t, w)
	require.Len(t, rows, 2, "a re-translation replaces the machine row, it does not add one")
	assert.Equal(t, "心之所向", rows[1].Title)
}

func TestWriteMachineTitleRefusesWhenASourceTitleExists(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "そら")
	mkTitle(t, w, "ja", "そら", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkTitle(t, w, "zh-Hans", "天空", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	res, err := writeMachineTitle(ctx, testDB, w, "苍穹", "hash-1", "glm-test")
	require.NoError(t, err)
	assert.Equal(t, writeSkippedSource, res)
	assert.Len(t, titlesOf(t, w), 2, "a source title is never displaced by a machine one")
}

func TestRetranslationTriggersOnSrcHashDrift(t *testing.T) {
	clean(t)
	ctx := t.Context()

	stale := mkWork(t, "旧")
	mkTitle(t, stale, "ja", "ふるい", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	_, err := writeMachineTitle(ctx, testDB, stale, "旧的", "stale-hash", "glm-test")
	require.NoError(t, err)

	fresh := mkWork(t, "新")
	mkTitle(t, fresh, "ja", "あたらしい", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	_, err = writeMachineTitle(ctx, testDB, fresh, "新的", hashSource("あたらしい"), "glm-test")
	require.NoError(t, err)

	cands, err := loadMTCandidates(ctx, testDB, 0)
	require.NoError(t, err)
	byWork := map[int64]mtCandidate{}
	for _, c := range cands {
		byWork[c.WorkID] = c
	}
	require.Contains(t, byWork, stale)
	require.Contains(t, byWork, fresh)

	dec, hash := decide(byWork[stale])
	assert.Equal(t, decRetrans, dec, "a machine row whose src_hash no longer matches the ja title is stale")
	assert.Equal(t, hashSource("ふるい"), hash)

	dec, _ = decide(byWork[fresh])
	assert.Equal(t, decUnchanged, dec)
}

func TestLoadMTCandidatesExcludesSourceZhAndClassifies(t *testing.T) {
	clean(t)
	ctx := t.Context()

	kana := mkWork(t, "かな")
	mkTitle(t, kana, "ja", "そらのおと", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	kanji := mkWork(t, "漢字")
	mkTitle(t, kanji, "ja", "月姫", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	latin := mkWork(t, "latin")
	mkTitle(t, latin, "ja", "AIR", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	done := mkWork(t, "已译")
	mkTitle(t, done, "ja", "うみ", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)
	mkTitle(t, done, "zh-Hans", "海", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	hintOnly := mkWork(t, "只有检索提示")
	mkTitle(t, hintOnly, "ja", "けんさく", model.WorkTitleKindSearchHint, model.WorkTitleProvenanceSource)

	cands, err := loadMTCandidates(ctx, testDB, 0)
	require.NoError(t, err)
	classOf := map[int64]gateClass{}
	for _, c := range cands {
		classOf[c.WorkID] = c.Class
	}
	assert.Equal(t, map[int64]gateClass{
		kana:  classNeedsMT,
		kanji: classKanjiOnly,
		latin: classSkipLatin,
	}, classOf, "a work with a source zh title is out; a search-hint-only ja title is not a source title")

	c := censusOf(cands)
	assert.Equal(t, gateCensus{Total: 3, KanjiOnly: 1, NeedsMT: 1, SkipLatin: 1, Insert: 3}, c)
}

func TestApplyCSVIsIdempotent(t *testing.T) {
	clean(t)
	ctx := t.Context()

	w := mkWork(t, "こころ")
	mkTitle(t, w, "ja", "こころ", model.WorkTitleKindOfficial, model.WorkTitleProvenanceSource)

	path := filepath.Join(t.TempDir(), "accepted.csv")
	body := "work_id,ja_title,src_hash,decision,pop,proposed_zh,model,verdict,candidate_zh,rederived_zh,detail\n" +
		itoa(w) + ",こころ," + hashSource("こころ") + ",insert,0,心,glm-test,accept,心,心,\n" +
		itoa(w+9000) + ",ない,x,insert,0,,glm-test,reject_kana_left,そら,,\n"
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	require.NoError(t, runApplyCSV(ctx, testDB, path, false, 0, 0))
	assert.Len(t, titlesOf(t, w), 1, "dry run writes nothing")

	require.NoError(t, runApplyCSV(ctx, testDB, path, true, 0, 0))
	rows := titlesOf(t, w)
	require.Len(t, rows, 2)
	assert.Equal(t, "心", rows[1].Title)

	require.NoError(t, runApplyCSV(ctx, testDB, path, true, 0, 0))
	assert.Len(t, titlesOf(t, w), 2, "second run must write nothing")
}
