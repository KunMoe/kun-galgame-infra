package entityintromt

import (
	"context"
	"strings"

	"gorm.io/gorm"
)

type SourceLang string

const (
	SourceJa SourceLang = "ja"
	SourceEn SourceLang = "en"
)

type candidate struct {
	EntityID   int64    `gorm:"column:entity_id"`
	SrcLang    string   `gorm:"column:src_lang"`
	SourceID   int16    `gorm:"column:source_id"`
	Text       string   `gorm:"column:src_text"`
	MZhID      *int64   `gorm:"column:mzh_id"`
	MZhSrcHash *string  `gorm:"column:mzh_src_hash"`
	Gloss      Glossary `gorm:"-"`
}

func loadCandidates(ctx context.Context, db *gorm.DB, lane laneDef, limit, offset int, entityIDs []int64) ([]candidate, error) {
	t, id := lane.introTable, lane.idCol
	idGate, args := ``, []any{}
	if len(entityIDs) > 0 {
		idGate = `
		  AND src.entity_id IN (?)`
		args = append(args, entityIDs)
	}
	q := `
		WITH src AS (
			SELECT DISTINCT ON (` + id + `) ` + id + ` AS entity_id, lang, source_id, intro
			FROM ` + t + `
			WHERE lang IN ('ja','en') AND provenance = 0
			ORDER BY ` + id + `, (lang <> 'ja'), source_id
		),
		has_zh_source AS (
			SELECT DISTINCT ` + id + ` AS entity_id FROM ` + t + `
			WHERE lang IN ('zh-Hans','zh-Hant') AND provenance = 0
		),
		mzh AS (
			SELECT DISTINCT ON (` + id + `) ` + id + ` AS entity_id, id AS mzh_id, src_hash AS mzh_src_hash
			FROM ` + t + ` WHERE lang = 'zh-Hans' AND provenance = 1
			ORDER BY ` + id + `, source_id
		)
		SELECT src.entity_id, src.lang AS src_lang, src.source_id, src.intro AS src_text,
			mzh.mzh_id, mzh.mzh_src_hash
		FROM src
		JOIN ` + lane.entityTable + ` e ON e.id = src.entity_id AND e.deleted_at IS NULL
		LEFT JOIN has_zh_source hz ON hz.entity_id = src.entity_id
		LEFT JOIN mzh ON mzh.entity_id = src.entity_id
		WHERE hz.entity_id IS NULL` + idGate + `
		ORDER BY src.entity_id ASC`
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	if offset > 0 {
		q += ` OFFSET ?`
		args = append(args, offset)
	}
	var out []candidate
	if err := db.WithContext(ctx).Raw(q, args...).Scan(&out).Error; err != nil {
		return nil, err
	}
	kept := out[:0]
	for _, c := range out {
		if strings.TrimSpace(c.Text) != "" {
			kept = append(kept, c)
		}
	}
	return kept, nil
}
