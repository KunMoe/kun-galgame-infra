package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// The exact map moyu's publish wizard builds (SubmissionFields in
// kun-galgame-patch): official titles per language, one lang-less alias, an
// explicit content_rating, display_nsfw, and a zh-Hans intro. No refs, no
// site_work_id, no product_work_id.
const liveWizardFields = `{
	"catalog.work.display_name": "奇迹☆棉花糖",
	"catalog.work.olang": "ja",
	"catalog.work.titles": [
		{"lang": "zh-Hans", "title": "奇迹☆棉花糖", "kind": 0},
		{"lang": "ja", "title": "みらくる☆ましまろ", "kind": 0},
		{"lang": "", "title": "mirumashi", "kind": 1}
	],
	"catalog.work.content_rating": 2,
	"catalog.work.display_nsfw": true,
	"catalog.work.intros": [{"lang": "zh-Hans", "intro": "向导提交的简介。"}]
}`

// The token has to hold moderation authority: the snapshot face publishes raw
// registered field values with no display axis, so it is a moderator read of
// the caller's own tenant, not a receipt the submitter can fetch.
func liveSnapshot(t *testing.T, env *liveEnv, workID, token string) map[string]any {
	t.Helper()
	status, _, body := liveDo(t, env, http.MethodGet, "/v2/moderation/snapshots/work/"+workID, token, "")
	require.Equal(t, 200, status, string(body))
	var rec struct {
		FieldValues map[string]any `json:"field_values"`
	}
	require.NoError(t, json.Unmarshal(body, &rec))
	return rec.FieldValues
}

func liveCreateClaim(t *testing.T, env *liveEnv, token, body string) (int, liveClaimRecord, problem.Problem) {
	t.Helper()
	status, _, raw := liveDo(t, env, http.MethodPost, "/v2/me/claims", token, body)
	if status == 201 {
		var rec liveClaimRecord
		require.NoError(t, json.Unmarshal(raw, &rec), string(raw))
		return status, rec, problem.Problem{}
	}
	return status, liveClaimRecord{}, liveProblem(t, raw)
}

func liveRefTarget(t *testing.T, env *liveEnv, name, externalID string) int64 {
	t.Helper()
	empty := datatypes.JSON([]byte("{}"))
	w := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: name,
		ContentRating: model.ContentRatingAllAges, Status: model.WorkStatusLive,
		Extra: empty, FieldProvenance: empty,
	}
	require.NoError(t, env.db.Create(w).Error)
	require.NoError(t, env.db.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: w.ID, SourceID: 2,
		ExternalID: externalID, LinkKind: model.LinkKindExact, MatchedBy: "test",
	}).Error)
	return w.ID
}

// The publish wizard's whole submission, with nothing else in the body. This is
// the shape that had no v2 lane at all before wave R4.
func TestLiveMintClaimFromWizardFields(t *testing.T) {
	env := liveCatalog(t)

	status, rec, p := liveCreateClaim(t, env, livePlainToken, `{"field_values":`+liveWizardFields+`}`)
	require.Equal(t, 201, status, p.Detail)
	require.Equal(t, "pending", rec.State)
	require.NotEmpty(t, rec.ID)

	snap := liveSnapshot(t, env, rec.ID, liveUserToken)
	// display_name in the map is a seed, not the final value: applyTitles runs
	// after olang and rewrites the column to the official title in that language.
	// The wizard sends the zh-Hans name first and olang ja, so the work is born
	// under its Japanese title.
	require.Equal(t, "みらくる☆ましまろ", snap[editspec.FieldWorkDisplayName])
	require.Equal(t, "ja", snap[editspec.FieldWorkOLang])
	require.Equal(t, true, snap[editspec.FieldWorkDisplayNSFW])
	require.EqualValues(t, model.ContentRatingR18, snap[editspec.FieldWorkContentRating],
		"an explicit content_rating is honored, not overwritten by DeriveContentRating")

	titles, ok := snap[editspec.FieldWorkTitles].([]any)
	require.True(t, ok, snap[editspec.FieldWorkTitles])
	require.ElementsMatch(t, []any{
		map[string]any{"lang": "zh-Hans", "title": "奇迹☆棉花糖", "kind": float64(0)},
		map[string]any{"lang": "ja", "title": "みらくる☆ましまろ", "kind": float64(0)},
		map[string]any{"lang": "", "title": "mirumashi", "kind": float64(1)},
	}, titles)

	require.Equal(t, []any{
		map[string]any{"lang": "zh-Hans", "intro": "向导提交的简介。"},
	}, snap[editspec.FieldWorkIntros])

	var work model.CatalogWork
	require.NoError(t, env.db.First(&work, liveNum(t, rec.ID)).Error)
	require.Equal(t, liveSite, *work.Site)
	require.Equal(t, livePlainUID, *work.OwnerUserID)
	require.EqualValues(t, model.ContentRatingR18, work.ContentRating)
}

// v1's submit computed a trusted fast lane and v2's CreateClaim never did, so
// an editor holding catalog.edit.trusted lost it by moving to /v2.
func TestLiveTrustedFieldsMintLandsLive(t *testing.T) {
	env := liveCatalog(t)

	status, rec, p := liveCreateClaim(t, env, liveUserToken,
		`{"display_name":"Trusted Fields Mint","field_values":{"catalog.work.olang":"en"}}`)
	require.Equal(t, 201, status, p.Detail)
	require.Equal(t, "live", rec.State)

	status, rec, p = liveCreateClaim(t, env, livePlainToken,
		`{"display_name":"Untrusted Fields Mint","field_values":{"catalog.work.olang":"en"}}`)
	require.Equal(t, 201, status, p.Detail)
	require.Equal(t, "pending", rec.State, "the same body without the permission still queues for review")
}

func TestLiveClaimFieldsRefusals(t *testing.T) {
	env := liveCatalog(t)

	// fields describe a work being born; changing one that already exists is a
	// proposal, and silently ignoring them would answer 201 to a lost payload.
	status, _, p := liveCreateClaim(t, env, livePlainToken,
		`{"work_id":"`+idstr(env.fx.Claimable)+`","site_work_id":"41210","field_values":{"catalog.work.olang":"ja"}}`)
	require.Equal(t, 422, status)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "/field_values", p.Errors[0].Pointer)

	status, _, p = liveCreateClaim(t, env, livePlainToken,
		`{"display_name":"Bogus Key","field_values":{"catalog.work.bogus":"x"}}`)
	require.Equal(t, 422, status)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "/field_values/catalog.work.bogus", p.Errors[0].Pointer)

	status, _, p = liveCreateClaim(t, env, livePlainToken,
		`{"display_name":"Bad Language","field_values":{"catalog.work.olang":"klingon"}}`)
	require.Equal(t, 422, status)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "/field_values/"+editspec.FieldWorkOLang, p.Errors[0].Pointer)

	status, _, p = liveCreateClaim(t, env, livePlainToken,
		`{"field_values":{"catalog.work.olang":"ja"}}`)
	require.Equal(t, 422, status)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "/display_name", p.Errors[0].Pointer)
}

func TestLiveClaimFieldsDisplayNamePrecedence(t *testing.T) {
	env := liveCatalog(t)

	status, rec, p := liveCreateClaim(t, env, livePlainToken,
		`{"display_name":"Body Wins","field_values":{"catalog.work.display_name":"Map Loses","catalog.work.olang":"ja"}}`)
	require.Equal(t, 201, status, p.Detail)
	snap := liveSnapshot(t, env, rec.ID, liveUserToken)
	require.Equal(t, "Body Wins", snap[editspec.FieldWorkDisplayName])
}

func TestLiveClaimRefsWithFields(t *testing.T) {
	env := liveCatalog(t)
	target := liveRefTarget(t, env, "R4 Ref Target", "v66601")

	status, _, p := liveCreateClaim(t, env, livePlainToken,
		`{"refs":[{"source":"vndb","external_id":"v66601"}],"site_work_id":"41200","field_values":`+liveWizardFields+`}`)
	require.Equal(t, 409, status)
	require.Equal(t, problem.CodeAlreadyExists, p.Code)
	require.Contains(t, p.Detail, idstr(target))
	require.Contains(t, p.Detail, "vndb:v66601")

	// Without fields the same refs still claim the match silently: that is the
	// behaviour the 409 above deliberately does not extend to.
	status, rec, p := liveCreateClaim(t, env, livePlainToken,
		`{"refs":[{"source":"vndb","external_id":"v66601"}],"site_work_id":"41200"}`)
	require.Equal(t, 201, status, p.Detail)
	require.Equal(t, idstr(target), rec.ID)
	require.Equal(t, "draft", rec.State)
}

// The ref-derived anchors are what a later submission recognizes the work by,
// so they have to survive a caller who also sent catalog.work.links.
func TestLiveClaimRefsMintUnionsLinks(t *testing.T) {
	env := liveCatalog(t)

	status, rec, p := liveCreateClaim(t, env, livePlainToken, `{
		"refs": [{"source": "vndb", "external_id": "v66602"}],
		"display_name": "Union Links Mint",
		"field_values": {"catalog.work.links": [
			"https://x.com/r4handle",
			"https://vndb.org/v66602"
		]}
	}`)
	require.Equal(t, 201, status, p.Detail)

	snap := liveSnapshot(t, env, rec.ID, liveUserToken)
	require.ElementsMatch(t,
		[]any{"https://vndb.org/v66602", "https://x.com/r4handle"},
		snap[editspec.FieldWorkLinks],
		"the ref anchor is unioned in, and the caller's identical URL is not duplicated")
}
