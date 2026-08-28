package releasemeta

import (
	"context"
	"testing"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// content_rating is half of the public display verdict, and this job is the one
// writer of it that never goes through editspec. Downstream mirrors poll
// /v2/catalog/changes for that axis, so a rating filled here has to move
// updated_at or the flip reaches nobody until the next full sweep.
func TestFillRatingSurfacesOnTheChangesFeed(t *testing.T) {
	clean(t)
	ctx := context.Background()
	medium := galgameMedium(t)

	filled := mkWork(t, medium, "rating-mirror", nil, nil, 0)
	bystander := mkWork(t, medium, "rating-bystander", nil, nil, 0)

	require.NoError(t, testDB.Exec(`UPDATE catalog_work SET updated_at = now() - interval '1 hour'`).Error)
	pub := service.NewPublicService(testDB, service.NewReadService(testDB),
		service.NewResolveService(repository.NewRedirectRepository(testDB)), "")
	tip, err := pub.Changes(ctx, "", 500)
	require.NoError(t, err)

	w := &writer{db: testDB, stats: &Stats{}}
	w.fillRating(ctx, filled, model.ContentRatingR18, true)
	require.Equal(t, 1, w.stats.RatingFilled)

	require.NoError(t, testDB.Exec(`UPDATE catalog_work SET updated_at = updated_at - interval '10 seconds'`).Error)
	page, err := pub.Changes(ctx, tip.NextCursor, 500)
	require.NoError(t, err)

	ids := make([]int64, 0, len(page.Items))
	for _, it := range page.Items {
		ids = append(ids, it.ID)
		assert.False(t, it.Gone, "work %d is still live", it.ID)
	}
	assert.Equal(t, []int64{filled}, ids)
	assert.NotContains(t, ids, bystander)
}
