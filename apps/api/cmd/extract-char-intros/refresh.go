package main

import (
	"context"
	"fmt"

	"api/internal/platform/catalog/editspec"

	"gorm.io/gorm"
)

// Refresh bucket: a derived row is an EXCERPT of the work intro it was taken
// from, so when that intro is retranslated the excerpt keeps the old wording —
// including the character names the retranslation fixed. src_hash records the
// intro the passage came from; a row whose hash matches none of its works'
// current zh intros is quoting text that no longer exists.
//
// Neither other bucket can reach these rows (fill skips any character that has
// a zh row, panel skips any character that has a derived row), so without this
// query a derived row is written once and never revisited.
var candidateRefreshWorksSQL = `
	WITH zhi AS (
		SELECT DISTINCT ON (work_id) work_id, intro, updated_at
		FROM catalog_work_intro
		WHERE lang = 'zh-Hans' AND length(intro) >= 200
		ORDER BY work_id, source_id
	), der AS (
		SELECT id, character_id, src_hash
		FROM catalog_character_intro
		WHERE lang = 'zh-Hans' AND source_id = 18
	)
	SELECT w.id AS work_id, zhi.intro,
	       wc.character_id, c.display_name,
	       COALESCE((
	         SELECT a.name FROM catalog_character_alias a
	         WHERE a.character_id = c.id AND a.lang IN ('zh-Hans','zh','zh-Hant') AND a.kind IN (0,1)
	         ORDER BY (NOT a.is_primary_for_locale), (a.lang <> 'zh-Hans'), a.id LIMIT 1
	       ), '') AS zh_name,
	       der.id AS derived_id, der.src_hash AS derived_hash
	FROM catalog_work w
	JOIN zhi ON zhi.work_id = w.id
	JOIN catalog_work_character wc ON wc.work_id = w.id
	JOIN catalog_character c ON c.id = wc.character_id AND c.deleted_at IS NULL
	JOIN der ON der.character_id = wc.character_id
	WHERE w.deleted_at IS NULL
	  AND ` + editspec.NotSuppressedRosterSQL("wc") + `
	  AND NOT EXISTS (
	    SELECT 1 FROM catalog_work_character wc2
	    JOIN catalog_work_intro wi2 ON wi2.work_id = wc2.work_id AND wi2.lang = 'zh-Hans'
	    WHERE wc2.character_id = wc.character_id
	      AND encode(sha256(convert_to(wi2.intro,'UTF8')),'hex') = der.src_hash)`

func loadRefreshCandidateWorks(ctx context.Context, db *gorm.DB, o candidateOpts) ([]candidateWork, error) {
	sql := candidateRefreshWorksSQL + sinceClause(o.Since) + `
	ORDER BY w.id, wc.character_id`
	var rows []struct {
		WorkID      int64  `gorm:"column:work_id"`
		Intro       string `gorm:"column:intro"`
		CharacterID int64  `gorm:"column:character_id"`
		DisplayName string `gorm:"column:display_name"`
		ZhName      string `gorm:"column:zh_name"`
		DerivedID   int64  `gorm:"column:derived_id"`
		DerivedHash string `gorm:"column:derived_hash"`
	}
	if err := db.WithContext(ctx).Raw(sql, sinceArgs(o.Since)...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load refresh candidate works: %w", err)
	}
	var out []candidateWork
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].WorkID != r.WorkID {
			out = append(out, candidateWork{WorkID: r.WorkID, Intro: r.Intro})
		}
		cur := &out[len(out)-1]
		cur.Roster = append(cur.Roster, rosterChar{
			CharacterID: r.CharacterID, Name: r.DisplayName, ZhName: r.ZhName,
			DerivedID: r.DerivedID, DerivedHash: r.DerivedHash,
		})
	}
	return window(out, o), nil
}
