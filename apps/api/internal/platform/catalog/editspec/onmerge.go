package editspec

import (
	"context"

	"api/internal/platform/catalog/repository"
	"api/internal/platform/editing"

	"gorm.io/gorm"
)

func buildWorkOnMerge(db *gorm.DB) func(context.Context, editing.MergeEvent) error {
	return func(ctx context.Context, ev editing.MergeEvent) error {
		return repository.TouchWorks(ctx, db, []int64{ev.EntityID})
	}
}

func buildReleaseOnMerge(db *gorm.DB) func(context.Context, editing.MergeEvent) error {
	return func(ctx context.Context, ev editing.MergeEvent) error {
		return repository.TouchReleaseWorks(ctx, db, []int64{ev.EntityID})
	}
}
