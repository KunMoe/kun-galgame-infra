package service

import (
	"testing"
	"time"

	"api/internal/platform/catalog/imagerefs"
	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sourceID(t *testing.T, key string) int16 {
	t.Helper()
	var id int16
	require.NoError(t, testDB.Raw(`SELECT id FROM catalog_source WHERE key = ?`, key).Scan(&id).Error)
	require.NotZero(t, id, "source %q must be seeded", key)
	return id
}

func createCover(t *testing.T, workID int64, hash string) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogWorkCover{
		WorkID: workID, ImageHash: hash, SourceID: sourceID(t, "vndb"),
	}).Error)
}

func TestCoverDetachTouchesHostWork(t *testing.T) {
	cleanTables(t)

	host := createWork(t, "cover host")
	bystander := createWork(t, "bystander")
	createCover(t, host.ID, "hash-detached")
	createCover(t, bystander.ID, "hash-kept")

	settleWorks(t)
	before := map[int64]time.Time{
		host.ID:      workUpdatedAt(t, host.ID),
		bystander.ID: workUpdatedAt(t, bystander.ID),
	}
	cursor := drainChanges(t)

	removed, err := imagerefs.Detach(t.Context(), testDB, "hash-detached")
	require.NoError(t, err)
	require.Equal(t, int64(1), removed[imagerefs.KindWorkCover], "the detach must actually have removed the cover")

	assert.True(t, workUpdatedAt(t, host.ID).After(before[host.ID]),
		"the work lost a cover, so its public face renders different bytes")
	assert.True(t, workUpdatedAt(t, bystander.ID).Equal(before[bystander.ID]),
		"a work whose covers were untouched must stay out of the feed")

	assert.Equal(t, []int64{host.ID}, changesSince(t, cursor),
		"the feed carries exactly the work that lost the cover")
}

func TestCoverDetachNoOpLeavesWatermark(t *testing.T) {
	cleanTables(t)

	w := createWork(t, "untouched")
	createCover(t, w.ID, "hash-kept")

	settleWorks(t)
	before := workUpdatedAt(t, w.ID)

	removed, err := imagerefs.Detach(t.Context(), testDB, "hash-that-references-nothing")
	require.NoError(t, err)
	require.Zero(t, removed[imagerefs.KindWorkCover], "nothing referenced the hash")

	assert.True(t, workUpdatedAt(t, w.ID).Equal(before),
		"a detach that removed no row must not move any watermark")
}

func TestLabelLogoDetachTouchesLabelledWorks(t *testing.T) {
	cleanTables(t)

	label := &model.CatalogLabel{DisplayName: "Brand", Kind: model.LabelKindGameBrand, LogoHash: "hash-logo"}
	require.NoError(t, testDB.Create(label).Error)

	host := createWork(t, "carries the label")
	bystander := createWork(t, "no label")
	require.NoError(t, testDB.Create(&model.CatalogWorkLabel{
		WorkID: host.ID, LabelID: label.ID, Kind: model.WorkLabelKindBrand}).Error)

	settleWorks(t)
	before := map[int64]time.Time{
		host.ID:      workUpdatedAt(t, host.ID),
		bystander.ID: workUpdatedAt(t, bystander.ID),
	}

	removed, err := imagerefs.Detach(t.Context(), testDB, "hash-logo")
	require.NoError(t, err)
	require.Equal(t, int64(1), removed[imagerefs.KindLabelLogo])

	assert.True(t, workUpdatedAt(t, host.ID).After(before[host.ID]),
		"logo_hash rides the labels block, so every labelled work renders differently")
	assert.True(t, workUpdatedAt(t, bystander.ID).Equal(before[bystander.ID]),
		"a work without the label must stay out of the feed")
}

func TestConfirmRefTouchesHostWork(t *testing.T) {
	cleanTables(t)

	host := createWork(t, "ref host")
	bystander := createWork(t, "bystander")
	src := sourceID(t, "vndb")
	addExternalRef(t, model.EntityTypeWork, host.ID, src, "v100", model.LinkKindProbable)

	settleWorks(t)
	before := map[int64]time.Time{
		host.ID:      workUpdatedAt(t, host.ID),
		bystander.ID: workUpdatedAt(t, bystander.ID),
	}
	cursor := drainChanges(t)

	require.NoError(t, testQueues.ConfirmRef(t.Context(), RefKey{
		EntityType: model.EntityTypeWork, EntityID: host.ID, SourceID: src, ExternalID: "v100",
	}, 7))

	assert.True(t, workUpdatedAt(t, host.ID).After(before[host.ID]),
		"probable->exact makes the ref appear in the public refs block")
	assert.True(t, workUpdatedAt(t, bystander.ID).Equal(before[bystander.ID]))
	assert.Equal(t, []int64{host.ID}, changesSince(t, cursor))
}

func TestRejectRefTouchesHostWork(t *testing.T) {
	cleanTables(t)

	host := createWork(t, "ref host")
	src := sourceID(t, "vndb")
	addExternalRef(t, model.EntityTypeWork, host.ID, src, "v200", model.LinkKindExact)

	settleWorks(t)
	before := workUpdatedAt(t, host.ID)

	require.NoError(t, testQueues.RejectRef(t.Context(), RefKey{
		EntityType: model.EntityTypeWork, EntityID: host.ID, SourceID: src, ExternalID: "v200",
	}, "wrong game", 7))

	assert.True(t, workUpdatedAt(t, host.ID).After(before),
		"deleting an exact ref removes a row the public refs block was rendering")
}

func TestConfirmReleaseRefTouchesParentWork(t *testing.T) {
	cleanTables(t)

	host := createWork(t, "release parent")
	rel := createRelease(t, host.ID, 2020, 1, 1)
	src := sourceID(t, "vndb")
	addExternalRef(t, model.EntityTypeRelease, rel.ID, src, "r300", model.LinkKindProbable)

	settleWorks(t)
	before := workUpdatedAt(t, host.ID)

	require.NoError(t, testQueues.ConfirmRef(t.Context(), RefKey{
		EntityType: model.EntityTypeRelease, EntityID: rel.ID, SourceID: src, ExternalID: "r300",
	}, 7))

	assert.True(t, workUpdatedAt(t, host.ID).After(before),
		"the refs block unions release-level anchors, so the parent work renders differently")
}

func TestMergeRehangOfCoverTouchesTarget(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	dst := createWork(t, "merge target")
	src := createWork(t, "merge source")
	createCover(t, src.ID, "hash-moves")

	settleWorks(t)
	before := workUpdatedAt(t, dst.ID)
	cursor := drainChanges(t)

	p, err := testMerge.ProposeMerge(ctx, model.EntityTypeWork, src.ID, dst.ID, 7, "same work")
	require.NoError(t, err)
	approveAndForceExecutable(t, p.ID)
	require.NoError(t, testMerge.ExecuteMerge(ctx, p.ID, nil))

	var workID int64
	require.NoError(t, testDB.Raw(
		`SELECT work_id FROM catalog_work_cover WHERE image_hash = ?`, "hash-moves").Scan(&workID).Error)
	require.Equal(t, dst.ID, workID, "the merge must actually have rehung the cover")

	assert.True(t, workUpdatedAt(t, dst.ID).After(before),
		"the target gained a cover it did not render before")
	assert.Equal(t, []int64{dst.ID}, changesSince(t, cursor),
		"only the surviving work enters the feed")
}
