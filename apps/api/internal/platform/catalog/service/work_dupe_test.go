package service

import (
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitWorkFilesSpacingDupeCandidate(t *testing.T) {
	s := newLifecycle(t)
	existing := createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "既存作品の表示名")
	require.NoError(t, testDB.Create(&model.CatalogWorkTitle{
		WorkID: existing.ID, Lang: "ja", Title: "重複検出タイトル", Kind: model.WorkTitleKindOfficial,
	}).Error)

	res, err := s.SubmitWork(t.Context(), SubmitWorkParams{
		Site: submitSite, ProductWorkID: 90501, ActorUID: 7,
		Fields: submitFields("重複 検出 タイトル"),
	})
	require.NoError(t, err)

	var cands []model.CatalogMatchCandidate
	require.NoError(t, testDB.Order("a_id, b_id").Find(&cands).Error)
	require.Len(t, cands, 1, "the mint must file one receipt, not one per matching norm")
	got := cands[0]
	assert.Equal(t, model.EntityTypeWork, got.EntityType)
	assert.Equal(t, min(existing.ID, res.WorkID), got.AID)
	assert.Equal(t, max(existing.ID, res.WorkID), got.BID)
	assert.Equal(t, model.CandidateReasonNameNormEqual, got.Reason)
	assert.Equal(t, model.CandidateStatusPending, got.Status)
	assert.Nil(t, got.DecidedBy)
}

func TestSubmitWorkWithoutCollisionFilesNothing(t *testing.T) {
	s := newLifecycle(t)
	createWorkX(t, galgameMediumID, model.ContentRatingAllAges, model.WorkStatusLive, "全く別の既存作品")

	_, err := s.SubmitWork(t.Context(), SubmitWorkParams{
		Site: submitSite, ProductWorkID: 90502, ActorUID: 7,
		Fields: submitFields("衝突しない新規作品"),
	})
	require.NoError(t, err)

	var filed int64
	require.NoError(t, testDB.Model(&model.CatalogMatchCandidate{}).Count(&filed).Error)
	assert.Zero(t, filed)
}
