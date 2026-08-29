package service

import (
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/require"
)

// The works list hydrated four blocks while declaring the detail face's whole
// vocabulary, so include=tags,credits was a 200 with neither block. Both are
// batch reads; what has to stay true is that batching them changes nothing but
// the number of round trips.
func TestWorksListCreditsAreTheDetailRowsBatched(t *testing.T) {
	f := seedCreditReads(t)
	read := NewReadService(testDB)
	ctx := t.Context()
	suppressCredit(t, f.work.ID, f.badVA)

	byWork, err := read.workCreditsFor(ctx, []int64{f.work.ID, f.other.ID})
	require.NoError(t, err)

	for _, id := range []int64{f.work.ID, f.other.ID} {
		one, err := read.WorkCredits(ctx, id)
		require.NoError(t, err)
		require.NotEmpty(t, one, "work %d must carry credits or this test cannot fail", id)
		require.Equal(t, one, byWork[id], "work %d reads differently in a batch", id)
	}

	// A suppression is per work: the batch must not leak one work's excluded row
	// into the other's slice, which a map keyed on anything but work_id would.
	for _, r := range byWork[f.other.ID] {
		require.Equal(t, f.other.ID, r.WorkID)
	}
	require.Len(t, byWork[f.work.ID], 3)
	require.Len(t, byWork[f.other.ID], 1)

	empty, err := read.workCreditsFor(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestWorksListBlocksMatchTheWorkDetail(t *testing.T) {
	f := seedCreditReads(t)
	ctx := t.Context()
	pub := newPublicSvc()
	require.NoError(t, testDB.Create(&model.CatalogWorkTag{
		WorkID: f.work.ID, Name: "純愛", Count: 3, SourceID: 3,
	}).Error)

	detail, found, err := pub.WorkDetail(ctx, f.work.ID,
		PublicInclude{Credits: true}, true, listSpoilerCeiling, PublicFields{})
	require.NoError(t, err)
	require.True(t, found)
	require.NotEmpty(t, detail.Tags)
	require.NotEmpty(t, detail.Credits)

	page, err := pub.WorksList(ctx, WorksListFilter{
		IDs: []int64{f.work.ID}, NSFW: true, Sort: "id",
		OLang:   PublicOLang{All: true},
		Include: WorksListInclude{Tags: true, Credits: true},
	}, "", 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, detail.Tags, page.Items[0].Tags)
	require.Equal(t, detail.Credits, page.Items[0].Credits)

	bare, err := pub.WorksList(ctx, WorksListFilter{
		IDs: []int64{f.work.ID}, NSFW: true, Sort: "id", OLang: PublicOLang{All: true},
	}, "", 10)
	require.NoError(t, err)
	require.Len(t, bare.Items, 1)
	require.Nil(t, bare.Items[0].Tags)
	require.Nil(t, bare.Items[0].Credits)
}
