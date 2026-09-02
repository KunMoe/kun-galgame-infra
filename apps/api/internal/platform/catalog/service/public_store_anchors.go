package service

import (
	"context"

	"api/internal/platform/catalog/model"
)

type StoreAnchor struct{ Source, ExternalID string }

func (s *PublicService) StoreAnchorsFor(ctx context.Context, workIDs []int64) (anchors map[int64][]StoreAnchor, visible map[int64]bool, err error) {
	anchors = map[int64][]StoreAnchor{}
	visible = map[int64]bool{}
	if len(workIDs) == 0 {
		return anchors, visible, nil
	}
	var ids []int64
	if err := s.db.WithContext(ctx).Raw(
		`SELECT id FROM catalog_work WHERE id IN ? AND deleted_at IS NULL AND medium_id = ? AND status = ?`,
		workIDs, galgameMediumID, model.WorkStatusLive,
	).Scan(&ids).Error; err != nil {
		return nil, nil, err
	}
	for _, id := range ids {
		visible[id] = true
		anchors[id] = []StoreAnchor{}
	}
	if len(ids) == 0 {
		return anchors, visible, nil
	}
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		Source     string `gorm:"column:source"`
		ExternalID string `gorm:"column:external_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT rel.work_id, src.key AS source, r.external_id
		FROM catalog_external_ref r
		JOIN catalog_release rel ON rel.id = r.entity_id AND rel.deleted_at IS NULL
		JOIN catalog_source src ON src.id = r.source_id
		WHERE r.entity_type = ? AND r.link_kind = ? AND r.dead_at IS NULL
			AND rel.work_id IN ? AND src.key IN ('dlsite','steam')
		ORDER BY rel.work_id, src.key, r.external_id`,
		model.EntityTypeRelease, model.LinkKindExact, ids,
	).Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		anchors[r.WorkID] = append(anchors[r.WorkID], StoreAnchor{Source: r.Source, ExternalID: r.ExternalID})
	}
	return anchors, visible, nil
}
