package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sayaPassage = "主人公的青梅竹马,性格开朗,总是照顾身边的每一个人。"

func seedDerivedRow(t *testing.T, characterID int64, intro, srcHash string) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_character_intro
		(character_id, lang, intro, source_id, provenance, src_hash, mt_model, created_at, updated_at)
		VALUES (?, 'zh-Hans', ?, 18, 1, ?, 'glm-5.2', now(), now())`,
		characterID, intro, srcHash).Error)
}

func TestRefreshBucketHoldsOnlyStaleRows(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID := seedWorkWithIntro(t, longIntro)
	fresh := seedRosterChar(t, workID, "沙耶")
	stale := seedRosterChar(t, workID, "玲")
	seedDerivedRow(t, fresh, sayaPassage, hashText(longIntro))
	seedDerivedRow(t, stale, "转校生,沉默寡言。", hashText("这是被替换掉的旧简介。"))

	cands, err := loadRefreshCandidateWorks(context.Background(), testDB, candidateOpts{})
	require.NoError(t, err)
	require.Len(t, cands, 1)
	require.Len(t, cands[0].Roster, 1, "a row whose hash still matches the current intro is not stale")
	assert.Equal(t, stale, cands[0].Roster[0].CharacterID)
	assert.Equal(t, hashText("这是被替换掉的旧简介。"), cands[0].Roster[0].DerivedHash)
}

// A character on two works is stale only when NO work of theirs still carries
// the intro the excerpt came from — otherwise the row is live and the other
// work would rewrite it with a passage from a different game.
func TestRefreshBucketSkipsRowsLiveOnAnotherWork(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	other := "另一部作品的简介。" + longIntro
	workA := seedWorkWithIntro(t, longIntro)
	workB := seedWorkWithIntro(t, other)
	saya := seedRosterChar(t, workA, "沙耶")
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_work_character
		(work_id, character_id, kind, spoiler, matched_by, created_at, updated_at)
		VALUES (?, ?, 0, 0, 'test', now(), now())`, workB, saya).Error)
	seedDerivedRow(t, saya, sayaPassage, hashText(other))

	cands, err := loadRefreshCandidateWorks(context.Background(), testDB, candidateOpts{})
	require.NoError(t, err)
	assert.Empty(t, cands)
}

func TestRunRefreshRewritesInPlace(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID := seedWorkWithIntro(t, longIntro)
	saya := seedRosterChar(t, workID, "沙耶")
	seedDerivedRow(t, saya, "主人公的青梅竹马、Saya。", hashText("旧简介"))

	ex := fakeExtractor{out: map[string]string{"沙耶": sayaPassage}}
	require.NoError(t, run(context.Background(), testDB, ex, nil, opts{Apply: true, Refresh: true}))

	var rows []struct {
		Intro   string `gorm:"column:intro"`
		SrcHash string `gorm:"column:src_hash"`
	}
	require.NoError(t, testDB.Raw(`SELECT intro, src_hash FROM catalog_character_intro
		WHERE character_id = ?`, saya).Scan(&rows).Error)
	require.Len(t, rows, 1, "the stale row is rewritten, not duplicated")
	assert.Equal(t, sayaPassage, rows[0].Intro)
	assert.Equal(t, hashText(longIntro), rows[0].SrcHash)

	cands, err := loadRefreshCandidateWorks(context.Background(), testDB, candidateOpts{})
	require.NoError(t, err)
	assert.Empty(t, cands, "the refreshed row leaves the bucket")
}

func TestRunRefreshKeepsTheRowWhenNothingIsExtractable(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID := seedWorkWithIntro(t, longIntro)
	saya := seedRosterChar(t, workID, "沙耶")
	seedDerivedRow(t, saya, "旧的摘录。", hashText("旧简介"))

	// Nothing extractable, and an invented passage: neither may erase the row.
	invented := "其实是来自未来的魔法使,为了拯救这座城市而隐藏身份潜入学园。"
	for _, out := range []map[string]string{{}, {"沙耶": invented}} {
		require.NoError(t, run(context.Background(), testDB, fakeExtractor{out: out}, nil,
			opts{Apply: true, Refresh: true}))
		var intro string
		require.NoError(t, testDB.Raw(`SELECT intro FROM catalog_character_intro
			WHERE character_id = ?`, saya).Scan(&intro).Error)
		assert.Equal(t, "旧的摘录。", intro, "a character never loses its intro to a failed refresh")
	}
}

func TestSinceFiltersOnTheElectedIntro(t *testing.T) {
	require.NoError(t, testDB.Exec(`TRUNCATE catalog_work, catalog_character RESTART IDENTITY CASCADE`).Error)
	workID := seedWorkWithIntro(t, longIntro)
	seedRosterChar(t, workID, "沙耶")
	require.NoError(t, testDB.Exec(`UPDATE catalog_work_intro SET updated_at = '2026-01-02T03:04:05Z'
		WHERE work_id = ?`, workID).Error)

	cands, err := loadCandidateWorks(context.Background(), testDB, candidateOpts{Since: "2026-01-01T00:00:00Z"})
	require.NoError(t, err)
	assert.Len(t, cands, 1)

	cands, err = loadCandidateWorks(context.Background(), testDB, candidateOpts{Since: "2026-02-01T00:00:00Z"})
	require.NoError(t, err)
	assert.Empty(t, cands, "an intro that has not changed since the cutoff is not re-scanned")
}
