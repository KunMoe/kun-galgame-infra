package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConstantGroupsHaveUniqueValues(t *testing.T) {
	groups := map[string][]int16{
		"entity_type": {
			EntityTypePerson, EntityTypeCreditName,
			EntityTypeLabel, EntityTypeCharacter, EntityTypeWork, EntityTypeRelease,
			EntityTypeTag, EntityTypeEngine,
		},
		"gender": {GenderMale, GenderFemale, GenderOther},
		"credit_name_kind": {
			CreditNameKindMain, CreditNameKindPenName,
			CreditNameKindDistinctPersona, CreditNameKindFormerName,
		},
		"link_visibility": {LinkVisibilityPublic, LinkVisibilityHidden},
		"alias_kind":      {AliasKindTranslation, AliasKindSpellingVariant, AliasKindSearchHint},
		"label_kind": {
			LabelKindGameBrand, LabelKindBunko, LabelKindPublisher,
			LabelKindAnimeStudio, LabelKindDoujinCircle, LabelKindGroup,
		},
		"revision_action": {
			RevisionActionCreated, RevisionActionUpdated, RevisionActionMergedSource,
			RevisionActionMergedTarget, RevisionActionSplit, RevisionActionImported,
			RevisionActionRedirect, RevisionActionReverted,
		},
		"relation_domain": {RelationDomainWork, RelationDomainEntity},
		"content_rating":  {ContentRatingAllAges, ContentRatingSensitive, ContentRatingR18},
		"work_status":     {WorkStatusLive, WorkStatusStub, WorkStatusMerged, WorkStatusQuarantine},
		"work_title_kind": {
			WorkTitleKindOfficial, WorkTitleKindAlias,
			WorkTitleKindAbbreviation, WorkTitleKindSearchHint,
		},
		"release_kind": {
			ReleaseKindDefault, ReleaseKindDigital, ReleaseKindPhysical,
			ReleaseKindTrial, ReleaseKindPatch,
		},
		"link_kind": {LinkKindExact, LinkKindProbable, LinkKindRelated},
		"candidate_reason": {
			CandidateReasonSharedExternalID, CandidateReasonNameNormEqual,
			CandidateReasonNameFuzzy, CandidateReasonImporterSuggest, CandidateReasonLLMSuggest,
		},
		"candidate_status": {
			CandidateStatusPending, CandidateStatusAccepted,
			CandidateStatusRejected, CandidateStatusDeferred, CandidateStatusNeedsManual,
		},
		"proposal_status": {
			ProposalStatusOpen, ProposalStatusApproved, ProposalStatusExecuted,
			ProposalStatusRejected, ProposalStatusWithdrawn,
		},
		"spoiler": {SpoilerNone, SpoilerMild, SpoilerSevere},
		"work_character_kind": {
			WorkCharacterKindUnknown, WorkCharacterKindMain,
			WorkCharacterKindSecondary, WorkCharacterKindAppears,
		},
		"label_relation": {
			LabelRelationParent, LabelRelationSubsidiary,
			LabelRelationImprint, LabelRelationImprintOf,
			LabelRelationSpawned, LabelRelationOrigin,
			LabelRelationSucceededBy, LabelRelationFormerly,
		},
	}
	for name, values := range groups {
		seen := make(map[int16]bool, len(values))
		for _, v := range values {
			assert.False(t, seen[v], "%s: duplicate value %d", name, v)
			seen[v] = true
		}
	}
}

func TestDisplayLimitKeyIsATwoValuePartition(t *testing.T) {
	wiki, empty := "galgame_wiki", ""
	pwid := int64(42)

	for _, tc := range []struct {
		name    string
		site    *string
		pwid    *int64
		display bool
		rating  int16
		want    string
	}{
		{"bodyless all_ages", nil, nil, false, ContentRatingAllAges, DisplayLimitKeySFW},
		{"bodyless sensitive", nil, nil, false, ContentRatingSensitive, DisplayLimitKeySFW},
		{"bodyless r18", nil, nil, false, ContentRatingR18, DisplayLimitKeyNSFW},
		{"empty site is bodyless", &empty, &pwid, true, ContentRatingR18, DisplayLimitKeyNSFW},
		{"site without a product work id is bodyless", &wiki, nil, true, ContentRatingAllAges, DisplayLimitKeySFW},
		{"claimed, editorially sfw, game r18", &wiki, &pwid, false, ContentRatingR18, DisplayLimitKeySFW},
		{"claimed, editorially nsfw, game all_ages", &wiki, &pwid, true, ContentRatingAllAges, DisplayLimitKeyNSFW},
		{"claimed, editorially nsfw, game r18", &wiki, &pwid, true, ContentRatingR18, DisplayLimitKeyNSFW},
		{"claimed, nothing declared", &wiki, &pwid, false, ContentRatingR18, DisplayLimitKeySFW},
	} {
		got := DisplayLimitKey(tc.site, tc.pwid, tc.display, tc.rating)
		assert.Equal(t, tc.want, got, "%s", tc.name)
		assert.Contains(t, []string{DisplayLimitKeySFW, DisplayLimitKeyNSFW}, got,
			"%s: the vocabulary is exactly two values", tc.name)
	}

	assert.NotEqual(t,
		DisplayLimitKey(&wiki, &pwid, false, ContentRatingR18),
		DisplayLimitKey(nil, nil, false, ContentRatingR18),
		"an r18 game reads sfw when claimed with safe material, nsfw when bodyless")
}

func TestLabelRelationVocabularyPairsAndRenders(t *testing.T) {
	inverse := map[int16]int16{
		LabelRelationParent:      LabelRelationSubsidiary,
		LabelRelationImprint:     LabelRelationImprintOf,
		LabelRelationSpawned:     LabelRelationOrigin,
		LabelRelationSucceededBy: LabelRelationFormerly,
	}
	for a, b := range inverse {
		assert.NotEqual(t, a, b, "a relation is never its own inverse")
		assert.Contains(t, LabelRelationKey, a)
		assert.Contains(t, LabelRelationKey, b)
	}
	assert.Len(t, LabelRelationKey, 2*len(inverse))
	assert.Equal(t, "imprint_of", LabelRelationKey[LabelRelationImprintOf])
	assert.Equal(t, "succeeded_by", LabelRelationKey[LabelRelationSucceededBy])
	assert.NotContains(t, LabelRelationKey, int16(0))
}

func TestEntityTypeValuesArePinned(t *testing.T) {
	assert.Equal(t, int16(0), EntityTypePerson)
	assert.Equal(t, int16(1), EntityTypeCreditName)
	assert.Equal(t, int16(3), EntityTypeLabel)
	assert.Equal(t, int16(4), EntityTypeCharacter)
	assert.Equal(t, int16(5), EntityTypeWork)
	assert.Equal(t, int16(6), EntityTypeRelease)
	assert.Equal(t, int16(7), EntityTypeTag)
	assert.Equal(t, int16(8), EntityTypeEngine)
}

func TestWorkStatusValuesArePinned(t *testing.T) {
	assert.Equal(t, int16(0), WorkStatusLive)
	assert.Equal(t, int16(1), WorkStatusStub)
	assert.Equal(t, int16(2), WorkStatusMerged)
	assert.Equal(t, int16(3), WorkStatusQuarantine)
}
