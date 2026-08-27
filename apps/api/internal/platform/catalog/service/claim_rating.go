package service

import (
	"context"
	"log/slog"
	"strings"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

type ClaimRef struct {
	Source     string
	ExternalID string
}

// DeriveContentRating asks the src_* staging schemas whether the refs a claim
// was minted with describe an adult work. Best-effort by design: a claim must
// mint even where the staging schemas are absent (dev/test databases), so a
// query error logs and yields no verdict rather than failing the claim.
// The claim wave that predated this left 16 adult works listed as all_ages —
// stubs mint with content_rating 0 and nothing downstream could rate them,
// because a v2 claim turns refs into links, not anchors, and the weekly
// releasemeta rating lane only reads anchors.
func (s *ClaimLifecycleService) DeriveContentRating(ctx context.Context, refs []ClaimRef) int16 {
	for _, ref := range refs {
		if sourceRefR18(ctx, s.db, ref.Source, ref.ExternalID) {
			return model.ContentRatingR18
		}
	}
	return model.ContentRatingAllAges
}

func (s *WorkService) deriveAnchorRating(ctx context.Context, anchors []ExternalAnchor) int16 {
	var rows []struct {
		ID  int16  `gorm:"column:id"`
		Key string `gorm:"column:key"`
	}
	if err := s.db.WithContext(ctx).
		Raw(`SELECT id, key FROM catalog_source WHERE key IN ('vndb', 'bangumi')`).
		Scan(&rows).Error; err != nil {
		slog.Warn("derive claim content_rating: resolve sources", "err", err)
		return model.ContentRatingAllAges
	}
	keyByID := make(map[int16]string, len(rows))
	for _, r := range rows {
		keyByID[r.ID] = r.Key
	}
	for _, a := range anchors {
		if sourceRefR18(ctx, s.db, keyByID[a.SourceID], a.ExternalID) {
			return model.ContentRatingR18
		}
	}
	return model.ContentRatingAllAges
}

func sourceRefR18(ctx context.Context, db *gorm.DB, source, externalID string) bool {
	id := strings.TrimSpace(externalID)
	if id == "" {
		return false
	}
	var r18 bool
	var err error
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "vndb":
		if strings.HasPrefix(id, "r") {
			err = db.WithContext(ctx).Raw(`SELECT EXISTS (
					SELECT 1 FROM src_vndb.releases rel
					WHERE rel.id = ? AND rel.patch = false
						AND (rel.minage >= 18 OR rel.has_ero))`, id).Scan(&r18).Error
			break
		}
		err = db.WithContext(ctx).Raw(`SELECT EXISTS (
				SELECT 1 FROM src_vndb.releases_vn rv
				JOIN src_vndb.releases rel ON rel.id = rv.id
				WHERE rv.vid = ? AND rel.patch = false
					AND (rel.minage >= 18 OR rel.has_ero))`, id).Scan(&r18).Error
	case "bangumi":
		err = db.WithContext(ctx).Raw(`SELECT COALESCE(bool_or(
				sub.nsfw OR sub.meta_tags @> '["R18"]'::jsonb), false)
				FROM src_bangumi.subject sub WHERE sub.id::text = ?`, id).Scan(&r18).Error
	default:
		return false
	}
	if err != nil {
		slog.Warn("derive claim content_rating", "source", source, "err", err)
		return false
	}
	return r18
}
