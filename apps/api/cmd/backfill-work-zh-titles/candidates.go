package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

// mtCandidate is FLAT on purpose: Raw().Scan() silently zero-fills anonymous
// embedded structs, so a batch row struct never embeds one.
type mtCandidate struct {
	WorkID     int64   `gorm:"column:work_id"`
	JaTitle    string  `gorm:"column:ja_title"`
	MZhID      *int64  `gorm:"column:mzh_id"`
	MZhTitle   *string `gorm:"column:mzh_title"`
	MZhSrcHash *string `gorm:"column:mzh_src_hash"`
	PopScore   int64   `gorm:"column:pop_score"`

	Class   gateClass `gorm:"-"`
	Context []string  `gorm:"-"`
}

func hashSource(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

type decision string

const (
	decInsert    decision = "insert"
	decRetrans   decision = "retranslate"
	decUnchanged decision = "unchanged"
)

// decide answers whether this work still needs the model. A stored machine row
// whose src_hash no longer matches the current ja title is stale: the original
// changed under it and the translation is of a title that is no longer there.
func decide(c mtCandidate) (decision, string) {
	hash := hashSource(cleanTitle(c.JaTitle))
	switch {
	case c.MZhID == nil:
		return decInsert, hash
	case c.MZhSrcHash != nil && *c.MZhSrcHash == hash:
		return decUnchanged, hash
	default:
		return decRetrans, hash
	}
}

// loadMTCandidates lists every live work with a ja title and no SOURCE Chinese
// title. A machine row does not exclude a work — that is what makes
// re-translation reachable — so the exclusion tests provenance, not language.
func loadMTCandidates(ctx context.Context, db *gorm.DB, limit int) ([]mtCandidate, error) {
	live := editspec.NotSuppressedWorkTitleSQL("t")
	q := `
	WITH ja AS (
		SELECT DISTINCT ON (t.work_id) t.work_id, t.title AS ja_title
		FROM catalog_work_title t
		WHERE (t.lang = 'ja' OR t.lang LIKE 'ja-%') AND t.kind <= ? AND ` + live + `
		ORDER BY t.work_id, t.kind, t.id
	),
	src_zh AS (
		SELECT DISTINCT t.work_id FROM catalog_work_title t
		WHERE t.lang LIKE 'zh%' AND t.provenance = ?
	),
	mzh AS (
		SELECT DISTINCT ON (t.work_id) t.work_id, t.id AS mzh_id,
		       t.title AS mzh_title, t.src_hash AS mzh_src_hash
		FROM catalog_work_title t
		WHERE t.lang LIKE 'zh%' AND t.provenance = ?
		ORDER BY t.work_id, t.kind, t.id
	),
	pop AS (
		SELECT work_id,
			max(value) FILTER (WHERE metric = ?) AS dl,
			max(value) FILTER (WHERE metric = ?) AS wl
		FROM catalog_work_popularity GROUP BY work_id
	)
	SELECT w.id AS work_id, ja.ja_title, mzh.mzh_id, mzh.mzh_title, mzh.mzh_src_hash,
	       COALESCE(pop.dl, pop.wl, 0) AS pop_score
	FROM catalog_work w
	JOIN ja ON ja.work_id = w.id
	LEFT JOIN src_zh ON src_zh.work_id = w.id
	LEFT JOIN mzh ON mzh.work_id = w.id
	LEFT JOIN pop ON pop.work_id = w.id
	WHERE w.deleted_at IS NULL AND src_zh.work_id IS NULL
	ORDER BY COALESCE(pop.dl, pop.wl, 0) DESC, w.id`
	var rows []mtCandidate
	err := db.WithContext(ctx).Raw(q,
		model.WorkTitleKindAlias,
		model.WorkTitleProvenanceSource,
		model.WorkTitleProvenanceMachine,
		model.PopularityMetricDownloads, model.PopularityMetricWishlist,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load mt candidates: %w", err)
	}
	for i := range rows {
		rows[i].Class = classify(rows[i].JaTitle)
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	return rows, nil
}

type gateCensus struct {
	Total, KanjiOnly, NeedsMT, SkipLatin int
	Insert, Retranslate, Unchanged       int
}

func censusOf(cands []mtCandidate) gateCensus {
	var c gateCensus
	c.Total = len(cands)
	for _, cand := range cands {
		switch cand.Class {
		case classKanjiOnly:
			c.KanjiOnly++
		case classNeedsMT:
			c.NeedsMT++
		case classSkipLatin:
			c.SkipLatin++
		}
		switch dec, _ := decide(cand); dec {
		case decInsert:
			c.Insert++
		case decRetrans:
			c.Retranslate++
		case decUnchanged:
			c.Unchanged++
		}
	}
	return c
}

func filterClass(cands []mtCandidate, class gateClass) []mtCandidate {
	out := make([]mtCandidate, 0, len(cands))
	for _, c := range cands {
		if c.Class == class {
			out = append(out, c)
		}
	}
	return out
}
