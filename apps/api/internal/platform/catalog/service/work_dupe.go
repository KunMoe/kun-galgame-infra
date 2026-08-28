package service

import (
	"strconv"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WorkDupeMinRunes is the shortest folded norm the dedup lanes compare on;
// shorter strings ("PC版", "同人誌") collide by genre, not identity.
const WorkDupeMinRunes = 4

// WorkTitleFoldSQL folds an already NFKC-lowered SQL text expression for
// duplicate comparison by removing every whitespace rune. The 2026-07 bgm
// import wave minted ~4.9k duplicate works past a gate that compared raw
// title_norm equality; whitespace variants of the same title were one of the
// two holes, so every dup detector and mint guard compares folded norms
// through this one definition.
func WorkTitleFoldSQL(expr string) string {
	return `regexp_replace(` + expr + `, '[[:space:]　]', '', 'g')`
}

// WorkDupeCorpusSQL selects every live work's folded identity norms as
// (work_id, n): official titles plus display_name. display_name must be in
// the corpus — wiki-claimed works were born with zero title rows, which is
// the other hole the 2026-07 wave slipped through.
func WorkDupeCorpusSQL() string {
	minLen := strconv.Itoa(WorkDupeMinRunes)
	return `SELECT t.work_id, ` + WorkTitleFoldSQL("t.title_norm") + ` AS n
		FROM catalog_work_title t
		JOIN catalog_work tw ON tw.id = t.work_id AND tw.deleted_at IS NULL
		WHERE t.kind = 0 AND length(t.title_norm) >= ` + minLen + `
		  AND ` + editspec.NotSuppressedWorkTitleSQL("t") + `
		UNION
		SELECT w.id, ` + WorkTitleFoldSQL("lower(normalize(w.display_name, NFKC))") + `
		FROM catalog_work w
		WHERE w.deleted_at IS NULL AND length(w.display_name) >= ` + minLen
}

// recordWorkDupeSuspects files a pending match candidate for every live work
// sharing a folded identity norm with the freshly minted work. It never
// blocks the mint — same-name distinct works are real, so the receipt goes
// to the reconciliation queue instead of the submitter.
func recordWorkDupeSuspects(tx *gorm.DB, workID int64) error {
	var hits []int64
	err := tx.Raw(`
		WITH corpus AS (`+WorkDupeCorpusSQL()+`)
		SELECT DISTINCT o.work_id
		FROM corpus mine
		JOIN corpus o ON o.n = mine.n AND o.work_id <> mine.work_id
		WHERE mine.work_id = ? AND length(mine.n) >= ?
		ORDER BY o.work_id`, workID, WorkDupeMinRunes).Scan(&hits).Error
	if err != nil || len(hits) == 0 {
		return err
	}
	rows := make([]model.CatalogMatchCandidate, 0, len(hits))
	for _, hit := range hits {
		a, b := workID, hit
		if b < a {
			a, b = b, a
		}
		rows = append(rows, model.CatalogMatchCandidate{
			EntityType: model.EntityTypeWork, AID: a, BID: b,
			Reason: model.CandidateReasonNameNormEqual,
			Status: model.CandidateStatusPending,
		})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}
