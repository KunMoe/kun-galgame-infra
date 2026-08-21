package imagegradesync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func workUpdatedAt(t *testing.T, id int64) time.Time {
	t.Helper()
	var ts time.Time
	require.NoError(t, catalogDB.Raw(`SELECT updated_at FROM catalog_work WHERE id = ?`, id).Scan(&ts).Error)
	return ts
}

func TestSyncTouchesOnlyRegradedWorks(t *testing.T) {
	reset(t)

	regraded := mkWork(t, "grade moves")
	agreed := mkWork(t, "grade already correct")
	mkImage(t, hashOf("b1"), 3)
	mkCover(t, regraded, hashOf("b1"), "bangumi", 0, 0)
	mkImage(t, hashOf("b2"), 2)
	mkCover(t, agreed, hashOf("b2"), "bangumi", 2, 0)

	require.NoError(t, catalogDB.Exec(`UPDATE catalog_work SET updated_at = now() - interval '1 hour'`).Error)
	before := map[int64]time.Time{
		regraded: workUpdatedAt(t, regraded),
		agreed:   workUpdatedAt(t, agreed),
	}

	st, err := Run(context.Background(), Opts{DSN: catalogDSN, ImagesDSN: imagesDSN, Apply: true})
	require.NoError(t, err)
	require.Equal(t, 1, st.Updated, "exactly one cover disagreed with its grade")

	assert.True(t, workUpdatedAt(t, regraded).After(before[regraded]),
		"sexual decides which cover the NSFW gate serves, so the public face changed")
	assert.True(t, workUpdatedAt(t, agreed).Equal(before[agreed]),
		"a cover already agreeing with its grade is not written, so nothing may enter the feed")
}
