package releasemeta

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

type registry struct {
	bangumiSource int16
	dlsiteSource  int16
	egSource      int16
	vndbSource    int16
}

func resolveRegistry(ctx context.Context, db *gorm.DB) (registry, error) {
	var r registry
	for _, s := range []struct {
		key string
		dst *int16
	}{
		{"bangumi", &r.bangumiSource},
		{"dlsite", &r.dlsiteSource},
		{"erogamescape", &r.egSource},
		{"vndb", &r.vndbSource},
	} {
		if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = ?`, s.key).Scan(s.dst).Error; err != nil {
			return r, fmt.Errorf("resolve %s source: %w", s.key, err)
		}
	}
	if r.bangumiSource == 0 || r.dlsiteSource == 0 || r.egSource == 0 || r.vndbSource == 0 {
		return r, fmt.Errorf("registry not seeded (bangumi=%d, dlsite=%d, erogamescape=%d, vndb=%d)",
			r.bangumiSource, r.dlsiteSource, r.egSource, r.vndbSource)
	}
	return r, nil
}

const emptyDate = `(rel.released_y IS NULL OR rel.released_y = 0)`

const singleRelease = `(SELECT count(*) FROM catalog_release r2 WHERE r2.work_id = w.id AND r2.deleted_at IS NULL) = 1`

type dlDateCandidate struct {
	ReleaseID int64  `gorm:"column:release_id"`
	WorkID    int64  `gorm:"column:work_id"`
	Workno    string `gorm:"column:workno"`
}

func loadDlDateCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]dlDateCandidate, error) {
	var out []dlDateCandidate
	if err := db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (rel.id) rel.id AS release_id, rel.work_id AS work_id, r.external_id AS workno
			FROM catalog_release rel
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = rel.id
				AND r.source_id = ? AND r.link_kind = ?
			WHERE rel.deleted_at IS NULL AND `+emptyDate+`
			ORDER BY rel.id, r.external_id`,
			model.EntityTypeRelease, reg.dlsiteSource, model.LinkKindExact).
		Scan(&out).Error; err != nil {
		return nil, err
	}
	return window(out, limit, offset), nil
}

type egDateCandidate struct {
	WorkID    int64
	ReleaseID int64
	EgIDs     []int64
}

func loadEgDateCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]egDateCandidate, error) {
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		ReleaseID  int64  `gorm:"column:release_id"`
		ExternalID string `gorm:"column:external_id"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT w.id AS work_id, rel.id AS release_id, r.external_id AS external_id
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ?
			JOIN catalog_release rel ON rel.work_id = w.id AND rel.deleted_at IS NULL
			WHERE (w.site IS NULL OR w.site = '') AND w.deleted_at IS NULL
				AND `+emptyDate+` AND `+singleRelease+`
			ORDER BY w.id, r.external_id`,
			model.EntityTypeWork, reg.egSource, model.LinkKindExact).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	byWork := map[int64]*egDateCandidate{}
	var order []int64
	for _, r := range rows {
		egID, err := strconv.ParseInt(r.ExternalID, 10, 64)
		if err != nil {
			egID = -1
		}
		c := byWork[r.WorkID]
		if c == nil {
			c = &egDateCandidate{WorkID: r.WorkID, ReleaseID: r.ReleaseID}
			byWork[r.WorkID] = c
			order = append(order, r.WorkID)
		}
		c.EgIDs = append(c.EgIDs, egID)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]egDateCandidate, 0, len(order))
	for _, id := range order {
		sort.Slice(byWork[id].EgIDs, func(i, j int) bool { return byWork[id].EgIDs[i] < byWork[id].EgIDs[j] })
		out = append(out, *byWork[id])
	}
	return window(out, limit, offset), nil
}

type bgmDateCandidate struct {
	WorkID    int64  `gorm:"column:work_id"`
	ReleaseID int64  `gorm:"column:release_id"`
	SubjectID int64  `gorm:"column:subject_id"`
	Date      string `gorm:"column:date"`
}

func loadBgmDateCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]bgmDateCandidate, error) {
	var out []bgmDateCandidate
	if err := db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (w.id) w.id AS work_id, rel.id AS release_id,
				sub.id AS subject_id, sub.date AS date
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ?
			JOIN src_bangumi.subject sub ON sub.id = r.external_id::bigint
			JOIN catalog_release rel ON rel.work_id = w.id AND rel.deleted_at IS NULL
			WHERE w.deleted_at IS NULL AND `+emptyDate+` AND `+singleRelease+`
			ORDER BY w.id, sub.id`,
			model.EntityTypeWork, reg.bangumiSource, model.LinkKindExact).
		Scan(&out).Error; err != nil {
		return nil, err
	}
	return window(out, limit, offset), nil
}

type ratingCandidate struct {
	WorkID        int64   `gorm:"column:work_id"`
	Site          *string `gorm:"column:site"`
	ProductWorkID *int64  `gorm:"column:product_work_id"`
}

func loadRatingCandidates(ctx context.Context, db *gorm.DB, limit, offset int) ([]ratingCandidate, error) {
	var out []ratingCandidate
	if err := db.WithContext(ctx).
		Raw(`SELECT id AS work_id, site AS site, product_work_id AS product_work_id
			FROM catalog_work
			WHERE deleted_at IS NULL AND content_rating = 0
			ORDER BY id`).
		Scan(&out).Error; err != nil {
		return nil, err
	}
	return window(out, limit, offset), nil
}

func loadRatingDlsiteAnchors(ctx context.Context, db *gorm.DB, reg registry) (map[int64]string, error) {
	var rows []struct {
		WorkID int64  `gorm:"column:work_id"`
		Workno string `gorm:"column:workno"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (w.id) w.id AS work_id, r.external_id AS workno
			FROM catalog_work w
			JOIN catalog_release rel ON rel.work_id = w.id AND rel.deleted_at IS NULL
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = rel.id
				AND r.source_id = ? AND r.link_kind = ?
			WHERE w.deleted_at IS NULL AND w.content_rating = 0
			ORDER BY w.id, r.external_id`,
			model.EntityTypeRelease, reg.dlsiteSource, model.LinkKindExact).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		out[r.WorkID] = r.Workno
	}
	return out, nil
}

// The nsfw flag alone is close to blind on the doujin tail: of 492 unclaimed
// works whose only covers were explicit, ZERO carried nsfw=true while 117
// carried the wiki-curated "R18" meta_tag (2026-08 census). Both signals are
// positive-only; neither absent asserts all-ages.
func loadRatingBgmR18(ctx context.Context, db *gorm.DB, reg registry) (map[int64]bool, error) {
	var rows []struct {
		WorkID int64 `gorm:"column:work_id"`
		R18    bool  `gorm:"column:r18"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT w.id AS work_id,
				bool_or(sub.nsfw OR sub.meta_tags @> '["R18"]'::jsonb) AS r18
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ?
			JOIN src_bangumi.subject sub ON sub.id = r.external_id::bigint
			WHERE w.deleted_at IS NULL AND w.content_rating = 0
			GROUP BY w.id`,
			model.EntityTypeWork, reg.bangumiSource, model.LinkKindExact).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(rows))
	for _, r := range rows {
		out[r.WorkID] = r.R18
	}
	return out, nil
}

// Same shape as buildVNDBWorkPoolQuery's r18 column: releases carry the age,
// the vn row does not, and patch releases are excluded because an 18+ patch
// for an all-ages VN is exactly the case where the release's rating is not
// the work's.
func loadRatingVndbR18(ctx context.Context, db *gorm.DB, reg registry) (map[int64]bool, error) {
	var rows []struct {
		WorkID int64 `gorm:"column:work_id"`
		R18    bool  `gorm:"column:r18"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT w.id AS work_id, bool_or(EXISTS (
				SELECT 1 FROM src_vndb.releases_vn rv
				JOIN src_vndb.releases rel ON rel.id = rv.id
				WHERE rv.vid = r.external_id AND rel.patch = false
					AND (rel.minage >= 18 OR rel.has_ero))) AS r18
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ?
			WHERE w.deleted_at IS NULL AND w.content_rating = 0
			GROUP BY w.id`,
			model.EntityTypeWork, reg.vndbSource, model.LinkKindExact).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(rows))
	for _, r := range rows {
		out[r.WorkID] = r.R18
	}
	return out, nil
}

func loadRatingEgAnchors(ctx context.Context, db *gorm.DB, reg registry) (map[int64][]int64, error) {
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		ExternalID string `gorm:"column:external_id"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT w.id AS work_id, r.external_id AS external_id
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ?
			WHERE w.deleted_at IS NULL AND w.content_rating = 0
			ORDER BY w.id, r.external_id`,
			model.EntityTypeWork, reg.egSource, model.LinkKindExact).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64][]int64, len(rows))
	for _, r := range rows {
		id, err := strconv.ParseInt(r.ExternalID, 10, 64)
		if err != nil {
			continue
		}
		out[r.WorkID] = append(out[r.WorkID], id)
	}
	return out, nil
}

// erogame=false is not an all-ages assertion: EG rosters 全年齢版 re-releases
// and plain non-ero games as false, so only true carries a verdict.
func loadEgErogame(ctx context.Context, egDB *gorm.DB, ids []int64) (map[int64]bool, error) {
	out := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID      int64 `gorm:"column:id"`
		Erogame bool  `gorm:"column:erogame"`
	}
	if err := egDB.WithContext(ctx).
		Raw(`SELECT id, erogame FROM games WHERE id IN ? AND erogame IS NOT NULL`, ids).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = r.Erogame
	}
	return out, nil
}
