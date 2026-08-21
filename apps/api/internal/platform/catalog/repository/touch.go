package repository

import (
	"context"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

const touchChunk = 2000

func TouchWorks(ctx context.Context, db *gorm.DB, workIDs []int64) error {
	if len(workIDs) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(workIDs))
	ids := make([]int64, 0, len(workIDs))
	for _, id := range workIDs {
		if id == 0 {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += touchChunk {
		end := min(start+touchChunk, len(ids))
		if err := db.WithContext(ctx).
			Exec(`UPDATE catalog_work SET updated_at = now() WHERE id IN (?)`, ids[start:end]).
			Error; err != nil {
			return err
		}
	}
	return nil
}

// TouchReleaseWorks resolves releases to their parent works. The lookup is
// deliberately unscoped: hiding a release changes the work's release_date and
// refs, and a soft-deleted row still has to name the work it belonged to.
func TouchReleaseWorks(ctx context.Context, db *gorm.DB, releaseIDs []int64) error {
	if len(releaseIDs) == 0 {
		return nil
	}
	var works []int64
	if err := db.WithContext(ctx).
		Raw(`SELECT work_id FROM catalog_release WHERE id IN ?`, releaseIDs).
		Scan(&works).Error; err != nil {
		return err
	}
	return TouchWorks(ctx, db, works)
}

// TouchRefHosts touches the works whose public refs block renders an external
// ref hung on (entityType, entityID). Refs on any other entity type never reach
// a work face.
func TouchRefHosts(ctx context.Context, db *gorm.DB, entityType int16, entityID int64) error {
	switch entityType {
	case model.EntityTypeWork:
		return TouchWorks(ctx, db, []int64{entityID})
	case model.EntityTypeRelease:
		return TouchReleaseWorks(ctx, db, []int64{entityID})
	}
	return nil
}
