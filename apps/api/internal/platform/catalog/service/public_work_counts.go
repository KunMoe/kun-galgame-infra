package service

import (
	"context"

	"api/internal/platform/catalog/dto"
)

func (s *PublicService) attachWorkChipCounts(ctx context.Context, rec *dto.PublicCatalogWork, nsfw bool) error {
	labelBlocks := make([][]dto.PublicWorkLabel, 0, 1+len(rec.Releases))
	labelBlocks = append(labelBlocks, rec.Labels)
	for i := range rec.Releases {
		labelBlocks = append(labelBlocks, rec.Releases[i].Labels)
	}
	if err := s.fillWorkLabelCounts(ctx, labelBlocks, nsfw); err != nil {
		return err
	}

	tagIDs := make([]int64, 0, len(rec.Tags))
	for _, t := range rec.Tags {
		if t.CanonicalID != 0 {
			tagIDs = append(tagIDs, t.CanonicalID)
		}
	}
	tagCounts, err := s.workCountsFor(ctx, tagWorkEdge, tagIDs, nsfw)
	if err != nil {
		return err
	}
	for i := range rec.Tags {
		if rec.Tags[i].CanonicalID == 0 {
			continue
		}
		n := tagCounts[rec.Tags[i].CanonicalID]
		rec.Tags[i].WorkCount = &n
	}

	engineIDs := make([]int64, 0, len(rec.Engines))
	for _, e := range rec.Engines {
		engineIDs = append(engineIDs, e.ID)
	}
	engineCounts, err := s.workCountsFor(ctx, engineWorkEdge, engineIDs, nsfw)
	if err != nil {
		return err
	}
	for i := range rec.Engines {
		rec.Engines[i].WorkCount = engineCounts[rec.Engines[i].ID]
	}
	return nil
}

func (s *PublicService) fillWorkLabelCounts(ctx context.Context, blocks [][]dto.PublicWorkLabel, nsfw bool) error {
	var ids []int64
	for _, b := range blocks {
		for _, l := range b {
			ids = append(ids, l.ID)
		}
	}
	// A NIL id list means "every label" to workCountsLive, not "none": without
	// this guard a work with no labels and no releases pays for a full aggregate
	// over catalog_work_label. works/{id}/releases reaches it on any work that
	// has no release rows at all.
	if len(ids) == 0 {
		return nil
	}
	counts, err := s.workCountsFor(ctx, labelWorkEdge, ids, nsfw)
	if err != nil {
		return err
	}
	for _, b := range blocks {
		for i := range b {
			b[i].WorkCount = counts[b[i].ID]
		}
	}
	return nil
}
