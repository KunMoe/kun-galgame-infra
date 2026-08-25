package bangumicovers

import (
	"context"
	"fmt"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

// Matched_by tags of the EXACT work→Bangumi anchors this backfill trusts:
//   - rule:bgm-title-year — minted by internal/jobs/doujinbangumi (BGM 竖封轨
//     Phase 1, refs/proj/56a): title + a corroborating ±1 release year.
//   - rule:bgm-type4-gated — the structural birth anchor minted by
//     internal/platform/catalog/importer (bgmtype4gated): the work was born
//     FROM that Bangumi subject, so the anchor is exact by construction. These
//     works were born cover-less (metadata-only import) — 11.4k of them had no
//     cover at all until refs/proj/56d widened this filter to include them.
//
// Probable (rule:bgm-title-only) anchors stay in the confirm bucket and are
// deliberately excluded. The strings are duplicated (not imported) to keep the
// job boundary clean; they are persisted contract values and must stay
// byte-identical to doujinbangumi.ruleTitleYear / importer.ruleBgmType4Gated.
const (
	ruleBgmTitleYear  = "rule:bgm-title-year"
	ruleBgmType4Gated = "rule:bgm-type4-gated"
)

var trustedRules = []string{ruleBgmTitleYear, ruleBgmType4Gated}

type registry struct {
	galgameMedium int16
	bangumiSource int16
}

func resolveRegistry(ctx context.Context, db *gorm.DB) (registry, error) {
	var r registry
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_medium WHERE key = 'galgame'`).Scan(&r.galgameMedium).Error; err != nil {
		return r, fmt.Errorf("resolve galgame medium: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'bangumi'`).Scan(&r.bangumiSource).Error; err != nil {
		return r, fmt.Errorf("resolve bangumi source: %w", err)
	}
	if r.galgameMedium == 0 || r.bangumiSource == 0 {
		return r, fmt.Errorf("registry not seeded (galgame medium=%d, bangumi source=%d)", r.galgameMedium, r.bangumiSource)
	}
	return r, nil
}

type candidate struct {
	WorkID    int64   `gorm:"column:work_id"`
	SubjectID string  `gorm:"column:subject_id"`
	Site      *string `gorm:"column:site"`
}

func loadCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]candidate, error) {
	q := db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (w.id) w.id AS work_id, r.external_id AS subject_id, w.site AS site
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ? AND r.matched_by IN ?
			WHERE w.medium_id = ? AND (w.site IS NULL OR w.site = '') AND w.deleted_at IS NULL
			ORDER BY w.id, r.external_id`,
			model.EntityTypeWork, reg.bangumiSource, model.LinkKindExact, trustedRules, reg.galgameMedium)
	var out []candidate
	if err := q.Scan(&out).Error; err != nil {
		return nil, err
	}
	if offset > 0 {
		if offset >= len(out) {
			return nil, nil
		}
		out = out[offset:]
	}
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func preloadExistingCovers(ctx context.Context, db *gorm.DB, workIDs []int64, bangumiSource int16) (map[int64]bool, error) {
	have := map[int64]bool{}
	if len(workIDs) == 0 {
		return have, nil
	}
	var ids []int64
	if err := db.WithContext(ctx).Raw(
		`SELECT DISTINCT work_id FROM catalog_work_cover WHERE source_id = ? AND work_id IN ?`,
		bangumiSource, workIDs).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("preload bangumi covers: %w", err)
	}
	for _, id := range ids {
		have[id] = true
	}
	return have, nil
}

func preloadPinnedCovers(ctx context.Context, db *gorm.DB, workIDs []int64) (map[int64]bool, error) {
	have := map[int64]bool{}
	if len(workIDs) == 0 {
		return have, nil
	}
	var ids []int64
	if err := db.WithContext(ctx).Raw(
		`SELECT DISTINCT work_id FROM catalog_work_cover WHERE portrait_pinned AND work_id IN ?`,
		workIDs).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("preload pinned covers: %w", err)
	}
	for _, id := range ids {
		have[id] = true
	}
	return have, nil
}
