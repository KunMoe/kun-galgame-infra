package service

import (
	"context"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/model"
)

const (
	labelGraphMaxDepth = 4
	labelGraphMaxNodes = 60
)

type labelGraphNodeRow struct {
	ID       int64  `gorm:"column:id"`
	Name     string `gorm:"column:display_name"`
	LogoHash string `gorm:"column:logo_hash"`
}

func (s *PublicService) LabelRelationGraph(ctx context.Context, id int64, nsfw bool) (dto.PublicLabelGraph, bool, error) {
	var seed labelGraphNodeRow
	if err := s.db.WithContext(ctx).Raw(`
		SELECT id, display_name, logo_hash FROM catalog_label
		WHERE id = ? AND deleted_at IS NULL`, id).Scan(&seed).Error; err != nil {
		return dto.PublicLabelGraph{}, false, err
	}
	if seed.ID == 0 {
		return dto.PublicLabelGraph{}, false, nil
	}

	nodes, truncated, err := s.labelGraphNodes(ctx, seed)
	if err != nil {
		return dto.PublicLabelGraph{}, false, err
	}
	ids := make([]int64, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	edges, err := s.labelGraphEdges(ctx, ids)
	if err != nil {
		return dto.PublicLabelGraph{}, false, err
	}
	counts, err := s.workCountsFor(ctx, labelWorkEdge, ids, nsfw)
	if err != nil {
		return dto.PublicLabelGraph{}, false, err
	}
	loc, err := s.localizedFor(ctx, labelAliasSource, ids)
	if err != nil {
		return dto.PublicLabelGraph{}, false, err
	}

	logoHashes := make([]string, 0, len(nodes))
	for _, n := range nodes {
		logoHashes = append(logoHashes, n.LogoHash)
	}
	logoMeta := s.entityMetaFor(ctx, logoHashes...)

	out := dto.PublicLabelGraph{
		Nodes:     make([]dto.PublicLabelGraphNode, len(nodes)),
		Edges:     edges,
		Truncated: truncated,
	}
	for i, n := range nodes {
		out.Nodes[i] = dto.PublicLabelGraphNode{
			ID: n.ID, DisplayName: n.Name, Localized: locOrEmpty(loc[n.ID]),
			LogoHash: n.LogoHash, LogoMeta: publicImageMeta(logoMeta, n.LogoHash),
			WorkCount: counts[n.ID],
		}
	}
	return out, true, nil
}

// The second return is the response's `truncated`: both ceilings are silent
// otherwise, and a family cut at 60 nodes is indistinguishable from a complete
// one on the wire.
func (s *PublicService) labelGraphNodes(ctx context.Context, seed labelGraphNodeRow) ([]labelGraphNodeRow, bool, error) {
	nodes := []labelGraphNodeRow{seed}
	visited := map[int64]struct{}{seed.ID: {}}
	frontier := []int64{seed.ID}
	truncated := false

	depth := 0
	for ; depth < labelGraphMaxDepth && len(frontier) > 0; depth++ {
		var rows []labelGraphNodeRow
		if err := s.db.WithContext(ctx).Raw(`
			SELECT DISTINCT other.id, other.display_name, other.logo_hash
			FROM catalog_label_relation r
			JOIN catalog_label other ON other.id = r.other_label_id AND other.deleted_at IS NULL
			WHERE r.label_id IN ?
			ORDER BY other.display_name, other.id`, frontier).Scan(&rows).Error; err != nil {
			return nil, false, err
		}
		next := make([]int64, 0, len(rows))
		for _, r := range rows {
			if _, seen := visited[r.ID]; seen {
				continue
			}
			if len(nodes) >= labelGraphMaxNodes {
				truncated = true
				break
			}
			visited[r.ID] = struct{}{}
			nodes = append(nodes, r)
			next = append(next, r.ID)
		}
		frontier = next
		if truncated {
			break
		}
	}
	if !truncated && depth == labelGraphMaxDepth && len(frontier) > 0 {
		// Hitting the depth ceiling is not itself truncation: the last ring may
		// have no unvisited neighbours at all. Asking costs one EXISTS and is
		// the difference between "partial" and "complete" on the wire.
		ids := make([]int64, 0, len(visited))
		for id := range visited {
			ids = append(ids, id)
		}
		var more bool
		if err := s.db.WithContext(ctx).Raw(`
			SELECT EXISTS (
				SELECT 1 FROM catalog_label_relation r
				JOIN catalog_label other ON other.id = r.other_label_id AND other.deleted_at IS NULL
				WHERE r.label_id IN ? AND other.id NOT IN ?)`, frontier, ids).Scan(&more).Error; err != nil {
			return nil, false, err
		}
		truncated = more
	}
	return nodes, truncated, nil
}

var labelGraphEdgeRelations = []int16{
	model.LabelRelationParent,
	model.LabelRelationImprint,
	model.LabelRelationSpawned,
	model.LabelRelationSucceededBy,
}

func (s *PublicService) labelGraphEdges(ctx context.Context, ids []int64) ([]dto.PublicLabelGraphEdge, error) {
	out := []dto.PublicLabelGraphEdge{}
	if len(ids) < 2 {
		return out, nil
	}
	var rows []struct {
		From     int64 `gorm:"column:label_id"`
		To       int64 `gorm:"column:other_label_id"`
		Relation int16 `gorm:"column:relation"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT r.label_id, r.other_label_id, r.relation
		FROM catalog_label_relation r
		WHERE r.label_id IN ? AND r.other_label_id IN ? AND r.relation IN ?
		ORDER BY r.label_id, r.relation, r.other_label_id`,
		ids, ids, labelGraphEdgeRelations).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		key, ok := model.LabelRelationKey[r.Relation]
		if !ok {
			continue
		}
		out = append(out, dto.PublicLabelGraphEdge{From: r.From, To: r.To, Relation: key})
	}
	return out, nil
}
