package service

import (
	"fmt"
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeRejectsDeadEndpoints(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	alive := createPerson(t, "alive")
	dead := createPerson(t, "dead")
	require.NoError(t, testDB.Delete(&model.CatalogPerson{}, dead.ID).Error)

	_, err := testMerge.ProposeMerge(ctx, model.EntityTypePerson, dead.ID, alive.ID, 7, "")
	require.ErrorIs(t, err, ErrEntityNotLive)
	_, err = testMerge.ProposeMerge(ctx, model.EntityTypePerson, alive.ID, dead.ID, 7, "")
	require.ErrorIs(t, err, ErrEntityNotLive)

	other := createPerson(t, "other")
	p, err := testMerge.ProposeMerge(ctx, model.EntityTypePerson, other.ID, alive.ID, 7, "")
	require.NoError(t, err)
	approveAndForceExecutable(t, p.ID)
	require.NoError(t, testDB.Delete(&model.CatalogPerson{}, other.ID).Error)
	require.ErrorIs(t, testMerge.ExecuteMerge(ctx, p.ID, nil), ErrEntityNotLive)
}

func TestProbableBucketIncludesMergeDemotedRows(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	target := createPerson(t, "target")
	source := createPerson(t, "source")
	addExternalRef(t, model.EntityTypePerson, target.ID, 3, "bgm-1", model.LinkKindExact)
	addExternalRef(t, model.EntityTypePerson, source.ID, 3, "bgm-2", model.LinkKindExact)

	p, err := testMerge.ProposeMerge(ctx, model.EntityTypePerson, source.ID, target.ID, 7, "")
	require.NoError(t, err)
	approveAndForceExecutable(t, p.ID)
	require.NoError(t, testMerge.ExecuteMerge(ctx, p.ID, nil))

	items, total, err := testQueues.ListProbableRefs(ctx, RefFilters{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "both demoted exacts enter the confirmation bucket")
	for _, item := range items {
		assert.Equal(t, "import:test", item.MatchedBy, "demotion keeps the original matched_by")
		assert.Nil(t, item.VerifiedAt)
		assert.Equal(t, target.ID, item.EntityID)
		assert.Equal(t, target.DisplayName, item.Entity.DisplayName, "entity brief attached")
	}
}

func TestDecideCandidatePaths(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	mk := func(a, b int64) {
		require.NoError(t, testDB.Create(&model.CatalogMatchCandidate{
			EntityType: model.EntityTypePerson, AID: min(a, b), BID: max(a, b),
			Reason: model.CandidateReasonNameNormEqual, Status: model.CandidateStatusPending,
		}).Error)
	}
	p1, p2 := createPerson(t, "P1"), createPerson(t, "P2")
	p3, p4 := createPerson(t, "P3"), createPerson(t, "P4")
	p5, p6 := createPerson(t, "P5"), createPerson(t, "P6")
	mk(p1.ID, p2.ID)
	mk(p3.ID, p4.ID)
	mk(p5.ID, p6.ID)

	outcome, err := testQueues.DecideCandidate(ctx, CandidateDecision{
		EntityType: model.EntityTypePerson, AID: p1.ID, BID: p2.ID,
		Action: "accept", SourceID: p2.ID, TargetID: p1.ID, Note: "same person", DecidedBy: 9,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Proposal)
	assert.Equal(t, p2.ID, outcome.Proposal.SourceEntityID)
	assert.Equal(t, p1.ID, outcome.Proposal.TargetEntityID)
	var cand model.CatalogMatchCandidate
	require.NoError(t, testDB.Where("a_id = ? AND b_id = ?", p1.ID, p2.ID).First(&cand).Error)
	assert.Equal(t, model.CandidateStatusAccepted, cand.Status)
	require.NotNil(t, cand.DecidedBy)
	assert.Equal(t, int64(9), *cand.DecidedBy)

	_, err = testQueues.DecideCandidate(ctx, CandidateDecision{
		EntityType: model.EntityTypePerson, AID: p1.ID, BID: p2.ID, Action: "reject", DecidedBy: 9,
	})
	require.ErrorIs(t, err, ErrProposalState)

	_, err = testQueues.DecideCandidate(ctx, CandidateDecision{
		EntityType: model.EntityTypePerson, AID: p3.ID, BID: p4.ID, Action: "reject", DecidedBy: 9,
	})
	require.NoError(t, err)
	var rejCount, candCount int64
	testDB.Model(&model.CatalogMatchRejection{}).Count(&rejCount)
	assert.Zero(t, rejCount, "candidate reject must NOT write catalog_match_rejection")
	testDB.Model(&model.CatalogMatchCandidate{}).Where("status = ?", model.CandidateStatusRejected).Count(&candCount)
	assert.Equal(t, int64(1), candCount)

	_, err = testQueues.DecideCandidate(ctx, CandidateDecision{
		EntityType: model.EntityTypePerson, AID: p5.ID, BID: p6.ID, Action: "defer", DecidedBy: 9,
	})
	require.NoError(t, err)
	_, err = testQueues.DecideCandidate(ctx, CandidateDecision{
		EntityType: model.EntityTypePerson, AID: p5.ID, BID: p6.ID,
		Action: "accept", SourceID: p6.ID, TargetID: p5.ID, DecidedBy: 9,
	})
	require.NoError(t, err)

	items, total, err := testQueues.ListCandidates(ctx, CandidateFilters{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.NotEmpty(t, items)
	assert.NotEmpty(t, items[0].A.DisplayName)
}

func TestDecideCandidateResolvesNeedsManual(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	file := func(a, b int64) CandidateDecision {
		require.NoError(t, testDB.Create(&model.CatalogMatchCandidate{
			EntityType: model.EntityTypePerson, AID: min(a, b), BID: max(a, b),
			Reason: model.CandidateReasonNameNormEqual, Status: model.CandidateStatusNeedsManual,
		}).Error)
		return CandidateDecision{EntityType: model.EntityTypePerson, AID: min(a, b), BID: max(a, b), DecidedBy: 9}
	}
	statusOf := func(d CandidateDecision) int16 {
		var cand model.CatalogMatchCandidate
		require.NoError(t, testDB.Where("a_id = ? AND b_id = ?", d.AID, d.BID).First(&cand).Error)
		return cand.Status
	}

	p1, p2 := createPerson(t, "NM1"), createPerson(t, "NM2")
	accept := file(p1.ID, p2.ID)
	accept.Action, accept.SourceID, accept.TargetID = "accept", p2.ID, p1.ID
	outcome, err := testQueues.DecideCandidate(ctx, accept)
	require.NoError(t, err)
	require.NotNil(t, outcome.Proposal)
	assert.Equal(t, model.CandidateStatusAccepted, statusOf(accept))

	p3, p4 := createPerson(t, "NM3"), createPerson(t, "NM4")
	reject := file(p3.ID, p4.ID)
	reject.Action = "reject"
	_, err = testQueues.DecideCandidate(ctx, reject)
	require.NoError(t, err)
	assert.Equal(t, model.CandidateStatusRejected, statusOf(reject))
}

func TestConfirmAndRejectRefs(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	holder := createPerson(t, "holder")
	contender := createPerson(t, "contender")
	addExternalRef(t, model.EntityTypePerson, holder.ID, 3, "bgm-9", model.LinkKindExact)
	addExternalRef(t, model.EntityTypePerson, contender.ID, 3, "bgm-9", model.LinkKindProbable)
	addExternalRef(t, model.EntityTypePerson, contender.ID, 2, "s77", model.LinkKindProbable)

	err := testQueues.ConfirmRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: contender.ID, SourceID: 3, ExternalID: "bgm-9",
	}, 9)
	require.ErrorIs(t, err, ErrExactTaken)
	assert.Contains(t, err.Error(), fmt.Sprintf("held by entity %d", holder.ID))

	require.NoError(t, testQueues.ConfirmRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: contender.ID, SourceID: 2, ExternalID: "s77",
	}, 9))
	var ref model.CatalogExternalRef
	require.NoError(t, testDB.Where("entity_id = ? AND source_id = ?", contender.ID, 2).First(&ref).Error)
	assert.Equal(t, model.LinkKindExact, ref.LinkKind)
	require.NotNil(t, ref.VerifiedBy)
	assert.Equal(t, int64(9), *ref.VerifiedBy)
	assert.NotNil(t, ref.VerifiedAt)

	err = testQueues.RejectRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: contender.ID, SourceID: 3, ExternalID: "bgm-9",
	}, "", 9)
	require.Error(t, err)
	require.NoError(t, testQueues.RejectRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: contender.ID, SourceID: 3, ExternalID: "bgm-9",
	}, "homonym, different person", 9))
	var refCount int64
	testDB.Model(&model.CatalogExternalRef{}).Where("entity_id = ? AND source_id = ?", contender.ID, 3).Count(&refCount)
	assert.Zero(t, refCount)
	var rej model.CatalogMatchRejection
	require.NoError(t, testDB.Where("entity_id = ? AND source_id = ?", contender.ID, 3).First(&rej).Error)
	assert.Equal(t, "homonym, different person", rej.Reason)
	require.NotNil(t, rej.RejectedBy)
	assert.Equal(t, int64(9), *rej.RejectedBy)
}

func TestRejectRefRefusesCuratedLink(t *testing.T) {
	cleanTables(t)
	ctx := t.Context()

	person := createPerson(t, "human-linked")
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypePerson, EntityID: person.ID,
		SourceID: 3, ExternalID: "bgm-human", LinkKind: model.LinkKindProbable, MatchedBy: matchedByCurated,
	}).Error)
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypePerson, EntityID: person.ID,
		SourceID: 2, ExternalID: "v-machine", LinkKind: model.LinkKindProbable, MatchedBy: "import:test",
	}).Error)

	err := testQueues.RejectRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: person.ID, SourceID: 3, ExternalID: "bgm-human",
	}, "looks wrong", 9)
	require.ErrorIs(t, err, ErrProposalState)
	assert.Contains(t, err.Error(), "human-linked")

	var curated model.CatalogExternalRef
	require.NoError(t, testDB.Where("entity_id = ? AND source_id = ? AND external_id = ?",
		person.ID, 3, "bgm-human").First(&curated).Error)
	assert.Equal(t, matchedByCurated, curated.MatchedBy)

	var rejCount int64
	testDB.Model(&model.CatalogMatchRejection{}).Where("entity_id = ? AND source_id = ?", person.ID, 3).Count(&rejCount)
	assert.Zero(t, rejCount)

	err = testQueues.RejectRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: person.ID, SourceID: 9, ExternalID: "missing",
	}, "gone", 9)
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, testQueues.RejectRef(ctx, RefKey{
		EntityType: model.EntityTypePerson, EntityID: person.ID, SourceID: 2, ExternalID: "v-machine",
	}, "wrong person", 9))
	var leftover int64
	testDB.Model(&model.CatalogExternalRef{}).Where("entity_id = ? AND source_id = ?", person.ID, 2).Count(&leftover)
	assert.Zero(t, leftover)
	var rej model.CatalogMatchRejection
	require.NoError(t, testDB.Where("entity_id = ? AND source_id = ?", person.ID, 2).First(&rej).Error)
	assert.Equal(t, "wrong person", rej.Reason)
}
