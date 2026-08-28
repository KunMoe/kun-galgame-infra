package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/require"
)

type liveClaimEventRef struct {
	Object    string  `json:"object"`
	ID        string  `json:"id"`
	FromState *string `json:"from_state"`
	ToState   string  `json:"to_state"`
	Reason    *string `json:"reason"`
	ActorUID  string  `json:"actor_uid"`
	CreatedAt string  `json:"created_at"`
}

type liveClaimRecord struct {
	Object        string             `json:"object"`
	ID            string             `json:"id"`
	State         string             `json:"state"`
	DisplayName   string             `json:"display_name"`
	Site          string             `json:"site"`
	ProductWorkID *string            `json:"product_work_id"`
	LastEvent     *liveClaimEventRef `json:"last_event"`
	FirstActedAt  *string            `json:"first_acted_at"`
	ActedCount    *int               `json:"acted_count"`
}

type liveClaimPage struct {
	Object     string            `json:"object"`
	Items      []liveClaimRecord `json:"items"`
	NextCursor *string           `json:"next_cursor"`
	Total      *int64            `json:"total"`
}

func liveClaimList(t *testing.T, env *liveEnv, query, token string) liveClaimPage {
	t.Helper()
	status, _, body := liveDo(t, env, http.MethodGet, "/v2/me/claims"+query, token, "")
	require.Equal(t, 200, status, string(body))
	var page liveClaimPage
	require.NoError(t, json.Unmarshal(body, &page))
	return page
}

func liveClaimByID(page liveClaimPage, id string) *liveClaimRecord {
	for i := range page.Items {
		if page.Items[i].ID == id {
			return &page.Items[i]
		}
	}
	return nil
}

// liveMintClaim mints a work through the site_work_id anchor and returns the
// catalog work id it landed on. Not for liveUserToken: that user is an admin,
// admin carries catalog.edit.trusted, and since wave R4 a trusted mint lands
// live rather than pending.
func liveMintClaim(t *testing.T, env *liveEnv, token, siteWorkID, displayName string) string {
	t.Helper()
	status, _, body := liveDo(t, env, http.MethodPost, "/v2/me/claims", token,
		`{"site_work_id":"`+siteWorkID+`","display_name":"`+displayName+`"}`)
	require.Equal(t, 201, status, string(body))
	var rec liveClaimRecord
	require.NoError(t, json.Unmarshal(body, &rec))
	require.Equal(t, "pending", rec.State)
	require.NotEmpty(t, rec.ID)
	return rec.ID
}

// The two moderation claim reads only required a site-bound user token; only
// DecideClaim checked the permission, so any logged-in user could read the whole
// pending queue including decline reasons.
func TestLiveModerationClaimsRequireReviewPermission(t *testing.T) {
	env := liveCatalog(t)
	one := "/v2/moderation/claims/" + liveMintClaim(t, env, livePlainToken, "41004", "Permission Gate Claim")

	for _, path := range []string{"/v2/moderation/claims", one} {
		status, _, body := liveDo(t, env, http.MethodGet, path, livePlainToken, "")
		require.Equal(t, 403, status, string(body))
		require.Equal(t, problem.CodePermissionRequired, liveProblem(t, body).Code, path)
	}
	for _, path := range []string{"/v2/moderation/claims", one} {
		status, _, body := liveDo(t, env, http.MethodGet, path, liveUserToken, "")
		require.Equal(t, 200, status, string(body))
	}
}

// The moderation queue rows carry the claim identity now; they still have no
// actor aggregate, so the last_event block stays absent there.
func TestLiveModerationQueueCarriesIdentity(t *testing.T) {
	env := liveCatalog(t)
	id := liveMintClaim(t, env, livePlainToken, "41003", "Queue Identity Claim")

	status, _, body := liveDo(t, env, http.MethodGet, "/v2/moderation/claims?limit=100", liveUserToken, "")
	require.Equal(t, 200, status, string(body))
	var page liveClaimPage
	require.NoError(t, json.Unmarshal(body, &page))
	row := liveClaimByID(page, id)
	require.NotNil(t, row, string(body))
	require.Equal(t, "pending", row.State)
	require.Equal(t, liveSite, row.Site)
	require.NotNil(t, row.ProductWorkID)
	require.Equal(t, "41003", *row.ProductWorkID)
	require.Nil(t, row.LastEvent)
	require.Nil(t, row.ActedCount)
}

// {site_work_id, display_name} used to fall through to "work_id or refs is
// required" — the only anchored-mint shape v1 offered had no v2 equivalent.
func TestLiveCreateClaimFromSiteWorkID(t *testing.T) {
	env := liveCatalog(t)

	id := liveMintClaim(t, env, livePlainToken, "41001", "Anchored Mint")

	status, _, body := liveDo(t, env, http.MethodGet, "/v2/me/claims/"+id, livePlainToken, "")
	require.Equal(t, 200, status, string(body))
	var rec liveClaimRecord
	require.NoError(t, json.Unmarshal(body, &rec))
	require.Equal(t, "Anchored Mint", rec.DisplayName)
	require.Equal(t, liveSite, rec.Site)
	require.NotNil(t, rec.ProductWorkID)
	require.Equal(t, "41001", *rec.ProductWorkID)

	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/claims", livePlainToken,
		`{"site_work_id":"41002"}`)
	require.Equal(t, 422, status, string(body))
	p := liveProblem(t, body)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Len(t, p.Errors, 1)
	require.Equal(t, "/display_name", p.Errors[0].Pointer)

	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/claims", livePlainToken,
		`{"display_name":"No Anchor At All"}`)
	require.Equal(t, 422, status, string(body))
	p = liveProblem(t, body)
	require.Equal(t, problem.CodeValidationFailed, p.Code)
	require.Contains(t, p.Detail, "site_work_id")

	// The anchor is unique per site: minting the same (site, product_work_id)
	// twice is the existing 409, not a second work.
	status, _, body = liveDo(t, env, http.MethodPost, "/v2/me/claims", livePlainToken,
		`{"site_work_id":"41001","display_name":"Anchored Mint Again"}`)
	require.Equal(t, 409, status, string(body))
	require.Equal(t, problem.CodeAlreadyExists, liveProblem(t, body).Code)
}

// claimRecordFrom dropped 9 of UserClaimItem's 13 fields. The decision here is
// taken by a DIFFERENT user than the owner, which is the only way last_event's
// actor_uid can be shown to be the moderator rather than the claimant.
func TestLiveMyClaimsReadParity(t *testing.T) {
	env := liveCatalog(t)

	id := liveMintClaim(t, env, livePlainToken, "41010", "Parity Claim")

	etag := liveETag(t, env, "/v2/moderation/claims/"+id, liveUserToken)
	status, _, body := liveDoHeader(t, env, http.MethodPost, "/v2/moderation/claims/"+id+"/decisions",
		liveUserToken, `{"decision":"approve","note":"parity note"}`, map[string]string{"If-Match": etag})
	require.Equal(t, 201, status, string(body))

	page := liveClaimList(t, env, "?site="+liveSite+"&limit=100", livePlainToken)
	rec := liveClaimByID(page, id)
	require.NotNil(t, rec, "the owner must see their own approved claim")
	require.Equal(t, "claim", rec.Object)
	require.Equal(t, "live", rec.State)
	require.Equal(t, "Parity Claim", rec.DisplayName)
	require.Equal(t, liveSite, rec.Site)
	require.NotNil(t, rec.ProductWorkID)
	require.Equal(t, "41010", *rec.ProductWorkID)

	require.NotNil(t, rec.LastEvent, "last_event is the whole point of the actor aggregate")
	require.Equal(t, "claim_event", rec.LastEvent.Object)
	require.Equal(t, "live", rec.LastEvent.ToState)
	require.NotNil(t, rec.LastEvent.FromState)
	require.Equal(t, "pending", *rec.LastEvent.FromState)
	require.NotNil(t, rec.LastEvent.Reason)
	require.Equal(t, "parity note", *rec.LastEvent.Reason)
	require.Equal(t, idstr(liveUID), rec.LastEvent.ActorUID,
		"actor_uid is the moderator who decided, not the claim owner")
	require.NotEmpty(t, rec.LastEvent.CreatedAt)

	require.NotNil(t, rec.FirstActedAt)
	require.NotEmpty(t, *rec.FirstActedAt)
	require.NotNil(t, rec.ActedCount)
	require.Equal(t, 1, *rec.ActedCount, "the owner acted once — the approve event is the moderator's")

	page = liveClaimList(t, env, "?claim_state=live&limit=100", livePlainToken)
	require.NotNil(t, liveClaimByID(page, id))
	page = liveClaimList(t, env, "?claim_state=draft,pending&limit=100", livePlainToken)
	require.Nil(t, liveClaimByID(page, id))

	page = liveClaimList(t, env, "?kind=audited&limit=100", livePlainToken)
	require.Nil(t, liveClaimByID(page, id), "the owner did not audit their own claim")
	page = liveClaimList(t, env, "?kind=audited&limit=100", liveUserToken)
	require.NotNil(t, liveClaimByID(page, id), "the moderator audited a work they do not own")
	page = liveClaimList(t, env, "?kind=all&limit=100", livePlainToken)
	require.NotNil(t, liveClaimByID(page, id))

	page = liveClaimList(t, env, "?site=no-such-site&limit=100", livePlainToken)
	require.Empty(t, page.Items, "an unknown site matches nothing")

	page = liveClaimList(t, env, "?limit=1&include_total=true", livePlainToken)
	require.NotNil(t, page.Total)
	require.Positive(t, *page.Total)

	// fields= on this face is validate-only today (nothing projects the body),
	// so what the widened ClaimSpec actually bought is that the new keys stop
	// being 400 UNKNOWN_FIELD.
	status, _, body = liveDo(t, env, http.MethodGet,
		"/v2/me/claims?fields=last_event,first_acted_at,acted_count,site,product_work_id", livePlainToken, "")
	require.Equal(t, 200, status, string(body))
	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/claims?fields=not_a_claim_key", livePlainToken, "")
	require.Equal(t, 400, status, string(body))
	require.Equal(t, problem.CodeUnknownField, liveProblem(t, body).Code)

	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/claims?kind=everything", livePlainToken, "")
	require.Equal(t, 400, status, string(body))
	require.Equal(t, problem.CodeUnknownEnumValue, liveProblem(t, body).Code)
}

func TestLiveDeleteMyClaim(t *testing.T) {
	env := liveCatalog(t)

	draft := liveMintClaim(t, env, livePlainToken, "41020", "Draft To Delete")
	liveWithdrawToDraft(t, env, draft, livePlainToken)

	status, _, body := liveDo(t, env, http.MethodDelete, "/v2/me/claims/"+draft, liveUserToken, "")
	require.Equal(t, 403, status, string(body))
	require.Equal(t, problem.CodeClaimNotOwned, liveProblem(t, body).Code)

	status, _, body = liveDo(t, env, http.MethodDelete, "/v2/me/claims/"+draft, livePlainToken, "")
	require.Equal(t, 204, status, string(body))

	status, _, body = liveDo(t, env, http.MethodGet, "/v2/me/claims/"+draft, livePlainToken, "")
	require.Equal(t, 404, status, string(body))
	page := liveClaimList(t, env, "?limit=100&kind=all", livePlainToken)
	require.Nil(t, liveClaimByID(page, draft), "a deleted draft leaves the collection too")

	// A live claim must be withdrawn to draft first.
	live := liveMintClaim(t, env, livePlainToken, "41021", "Live Not Deletable")
	liveWithdrawToDraft(t, env, live, livePlainToken)
	livePatchClaim(t, env, live, livePlainToken, "live")
	status, _, body = liveDo(t, env, http.MethodDelete, "/v2/me/claims/"+live, livePlainToken, "")
	require.Equal(t, 409, status, string(body))
	require.Equal(t, problem.CodeInvalidStateTransition, liveProblem(t, body).Code)

	status, _, body = liveDo(t, env, http.MethodDelete, "/v2/me/claims/999000111", livePlainToken, "")
	require.Equal(t, 404, status, string(body))
	require.Equal(t, problem.CodeNotFound, liveProblem(t, body).Code)
}

func liveWithdrawToDraft(t *testing.T, env *liveEnv, id, token string) {
	t.Helper()
	livePatchClaim(t, env, id, token, "withdrawn")
}

func livePatchClaim(t *testing.T, env *liveEnv, id, token, state string) {
	t.Helper()
	etag := liveETag(t, env, "/v2/me/claims/"+id, token)
	status, _, body := liveDoHeader(t, env, http.MethodPatch, "/v2/me/claims/"+id, token,
		`{"state":"`+state+`"}`, map[string]string{"If-Match": etag})
	require.Equal(t, 200, status, string(body))
}

type liveEventItem struct {
	Object        string  `json:"object"`
	ID            string  `json:"id"`
	WorkID        string  `json:"work_id"`
	FromState     *string `json:"from_state"`
	ToState       string  `json:"to_state"`
	Reason        *string `json:"reason"`
	ActorUID      string  `json:"actor_uid"`
	Site          string  `json:"site"`
	ProductWorkID *string `json:"product_work_id"`
	CreatedAt     string  `json:"created_at"`
}

type liveEventPage struct {
	Items      []liveEventItem `json:"items"`
	NextCursor *string         `json:"next_cursor"`
	Total      *int64          `json:"total"`
	Missing    *[]string       `json:"missing"`
}

func liveEvents(t *testing.T, env *liveEnv, query string) liveEventPage {
	t.Helper()
	status, _, body := liveDo(t, env, http.MethodGet, "/v2/catalog/claim-events"+query, liveAppKeyEvent, "")
	require.Equal(t, 200, status, string(body))
	var page liveEventPage
	require.NoError(t, json.Unmarshal(body, &page))
	return page
}

func TestLiveClaimEventsAuthLadder(t *testing.T) {
	env := liveCatalog(t)
	const path = "/v2/catalog/claim-events"

	status, _, body := liveDo(t, env, http.MethodGet, path, "", "")
	require.Equal(t, 401, status, string(body))
	require.Equal(t, problem.CodeMissingCredential, liveProblem(t, body).Code)

	status, _, body = liveDo(t, env, http.MethodGet, path, liveAppKey, "")
	require.Equal(t, 403, status, string(body))
	p := liveProblem(t, body)
	require.Equal(t, problem.CodeScopeRequired, p.Code)
	require.Contains(t, p.Detail, "claim_events:read")

	status, _, body = liveDo(t, env, http.MethodGet, path, liveAppKeyEvent, "")
	require.Equal(t, 200, status, string(body))

	// The extra scope is only demanded on this one path; the rest of /v2/catalog
	// still answers a plain catalog:read key.
	status, _, body = liveDo(t, env, http.MethodGet, "/v2/catalog/works?limit=1", liveAppKey, "")
	require.Equal(t, 200, status, string(body))
}

func TestLiveClaimEventsFeed(t *testing.T) {
	env := liveCatalog(t)

	// A tenant the token pipeline cannot reach: LookupSite binds every live
	// client to kungal, so the second site has to be written straight to the log.
	const otherSite = "othersite"
	const otherActor = int64(9901)
	foreign := make([]int64, 0, 2)
	for _, to := range []int16{model.ClaimStatePending, model.ClaimStateLive} {
		row := &model.CatalogClaimEvent{
			WorkID: env.fx.Work, ToState: to, ActorUID: otherActor, Site: otherSite,
		}
		require.NoError(t, env.db.Create(row).Error)
		foreign = append(foreign, row.ID)
	}

	page := liveEvents(t, env, "?limit=100")
	require.NotEmpty(t, page.Items)
	for i := 1; i < len(page.Items); i++ {
		require.Greater(t, liveNum(t, page.Items[i-1].ID), liveNum(t, page.Items[i].ID),
			"recorded_desc is the default")
	}

	page = liveEvents(t, env, "?site="+otherSite+"&limit=100")
	require.Len(t, page.Items, 2)
	for _, it := range page.Items {
		require.Equal(t, otherSite, it.Site)
		require.Equal(t, "claim_event", it.Object)
		require.Equal(t, idstr(env.fx.Work), it.WorkID)
		require.Equal(t, strconv.FormatInt(otherActor, 10), it.ActorUID)
	}

	page = liveEvents(t, env, "?actor_uid="+strconv.FormatInt(otherActor, 10)+"&limit=100")
	require.Len(t, page.Items, 2)

	page = liveEvents(t, env, "?work_id="+idstr(env.fx.Pending)+"&limit=100")
	require.NotEmpty(t, page.Items)
	for _, it := range page.Items {
		require.Equal(t, idstr(env.fx.Pending), it.WorkID)
		require.Equal(t, liveSite, it.Site)
	}

	page = liveEvents(t, env, "?limit=1&include_total=true")
	require.NotNil(t, page.Total)
	require.Positive(t, *page.Total)
	total := *page.Total

	// recorded_asc is the watermark walk: paging through it must reproduce the
	// ascending id sequence exactly once, with no repeats and no holes.
	var walked []int64
	query := "?sort=recorded_asc&limit=3"
	for {
		p := liveEvents(t, env, query)
		for _, it := range p.Items {
			walked = append(walked, liveNum(t, it.ID))
		}
		if p.NextCursor == nil {
			break
		}
		query = "?sort=recorded_asc&limit=3&cursor=" + *p.NextCursor
		require.LessOrEqual(t, int64(len(walked)), total, "the watermark walk overran the total")
	}
	require.Equal(t, int(total), len(walked))
	for i := 1; i < len(walked); i++ {
		require.Greater(t, walked[i], walked[i-1], "recorded_asc is strictly ascending")
	}

	missingID := "999000222"
	page = liveEvents(t, env, "?ids="+idstr(foreign[0])+","+idstr(foreign[1])+","+missingID)
	require.Len(t, page.Items, 2)
	require.NotNil(t, page.Missing)
	require.Equal(t, []string{missingID}, *page.Missing)
	require.Nil(t, page.NextCursor, "the batch lane does not paginate")
}

func liveNum(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	require.NoError(t, err, s)
	return n
}

func TestLiveWorksOwnerUIDFilter(t *testing.T) {
	env := liveCatalog(t)

	id := liveMintClaim(t, env, livePlainToken, "41030", "Owner Filter Claim")
	livePatchClaim(t, env, id, livePlainToken, "withdrawn")
	livePatchClaim(t, env, id, livePlainToken, "live")

	mine := liveWorkIDs(t, env, "/v2/catalog/works?limit=100&site="+liveSite+
		"&owner_uid="+strconv.FormatInt(livePlainUID, 10))
	require.Contains(t, mine, id)

	theirs := liveWorkIDs(t, env, "/v2/catalog/works?limit=100&site="+liveSite+"&owner_uid=987654")
	require.Empty(t, theirs, "an owner with no claims on this site matches nothing")

	status, _, body := liveDo(t, env, http.MethodGet,
		"/v2/catalog/works?owner_uid="+strconv.FormatInt(livePlainUID, 10), liveAppKey, "")
	require.Equal(t, 422, status, string(body))
	require.Equal(t, problem.CodeValidationFailed, liveProblem(t, body).Code)

	status, _, body = liveDo(t, env, http.MethodGet,
		"/v2/catalog/works?site="+liveSite+"&owner_uid=1&q=anything", liveAppKey, "")
	require.Equal(t, 400, status, string(body))
	require.Equal(t, problem.CodeMutuallyExclusiveParameters, liveProblem(t, body).Code)

	// pending/declined/hidden left the public lane with this wave.
	status, _, body = liveDo(t, env, http.MethodGet, "/v2/catalog/works?claim_state=pending", liveAppKey, "")
	require.Equal(t, 400, status, string(body))
	require.Equal(t, problem.CodeUnknownEnumValue, liveProblem(t, body).Code)
}

func liveWorkIDs(t *testing.T, env *liveEnv, path string) []string {
	t.Helper()
	status, _, body := liveDo(t, env, http.MethodGet, path, liveAppKey, "")
	require.Equal(t, 200, status, string(body))
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(body, &page))
	out := make([]string, 0, len(page.Items))
	for _, it := range page.Items {
		out = append(out, it.ID)
	}
	return out
}
