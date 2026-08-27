package handler

import (
	"encoding/json"
	"fmt"
	"testing"

	"api/internal/middleware"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/service"
	"api/pkg/oidctoken"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func userClaimApp(db *gorm.DB) *fiber.App {
	app := fiber.New()
	verifier := oidctoken.NewVerifier(userTestSecret, nil)
	app.Use(UserPrefix, middleware.JWTAuth(verifier), UserGate(userEditClients()))
	SetupUser(app, service.NewCoverVoteService(db), nil, nil, service.NewClaimLifecycleService(db),
		service.NewReadService(db))
	return app
}

func resetClaims(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, tbl := range []string{
		"catalog_claim_event", "catalog_release", "edit_suppressed_row", "catalog_work_title", "catalog_work",
	} {
		require.NoError(t, db.Exec("TRUNCATE "+tbl+" RESTART IDENTITY CASCADE").Error)
	}
}

func seedClaimedWork(t *testing.T, db *gorm.DB, state int16, ownerUID int64) int64 {
	t.Helper()
	site := "kungal"
	w := &model.CatalogWork{
		MediumID: 1, OLang: "ja", DisplayName: "権利テスト",
		Status: model.WorkStatusLive, Site: &site, ClaimState: &state,
	}
	require.NoError(t, db.Create(w).Error)
	require.NoError(t, db.Exec(
		`UPDATE catalog_work SET product_work_id = ? WHERE id = ?`, w.ID, w.ID).Error)
	if ownerUID > 0 {
		require.NoError(t, db.Exec(
			`UPDATE catalog_work SET owner_user_id = ? WHERE id = ?`, ownerUID, w.ID).Error)
	}
	return w.ID
}

func userClaimActionPath(workID int64, action string) string {
	return fmt.Sprintf("%s/works/%d/claim-actions/%s", UserPrefix, workID, action)
}

func moderatorToken(t *testing.T, uid uint) string {
	t.Helper()
	return userTokenRoles(t, uid, ScopeCatalogEdit, "kungal-client", "user", "moderator")
}

func claimStateOf(t *testing.T, db *gorm.DB, workID int64) string {
	t.Helper()
	var row struct {
		Site          *string
		ProductWorkID *int64
		ClaimState    *int16
	}
	require.NoError(t, db.Raw(
		`SELECT site, product_work_id, claim_state FROM catalog_work WHERE id = ?`, workID,
	).Scan(&row).Error)
	return model.ClaimStateKey(row.Site, row.ProductWorkID, row.ClaimState)
}

func TestUserClaims_BehindTheSameGate(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)
	work := seedClaimedWork(t, db, model.ClaimStateDraft, 801)

	for _, tc := range []struct {
		name, method, path, body, token string
		want                            int
	}{
		{"submit without a token", "POST", UserPrefix + "/works/submit",
			`{"fields":{"catalog.work.display_name":"無認証"}}`, "", fiber.StatusUnauthorized},
		{"act without a token", "POST", userClaimActionPath(work, "submit"), `{}`, "",
			fiber.StatusUnauthorized},
		{"mine without a token", "GET", UserPrefix + "/claims/mine", "", "",
			fiber.StatusUnauthorized},
		{"submit with the wrong scope", "POST", UserPrefix + "/works/submit",
			`{"fields":{"catalog.work.display_name":"無権限"}}`,
			userToken(t, 5, "openid profile", "kungal-client"), fiber.StatusForbidden},
		{"act with the wrong scope", "POST", userClaimActionPath(work, "submit"), `{}`,
			userToken(t, 5, "openid profile", "kungal-client"), fiber.StatusForbidden},
		{"mine with the wrong scope", "GET", UserPrefix + "/claims/mine", "",
			userToken(t, 5, "openid profile", "kungal-client"), fiber.StatusForbidden},
	} {
		status, raw := userEditReq(t, app, tc.method, tc.path, tc.token, tc.body)
		assert.Equalf(t, tc.want, status, "%s: %s", tc.name, raw)
	}

	var rows int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM catalog_claim_event`).Scan(&rows).Error)
	assert.EqualValues(t, 0, rows, "no refusal wrote an event")
	assert.Equal(t, model.ClaimStateKeyDraft, claimStateOf(t, db, work), "and none moved the claim")
}

func TestUserClaims_SubmitDerivesActorAndSite(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)

	status, raw := userEditReq(t, app, "POST", UserPrefix+"/works/submit",
		userToken(t, 811, ScopeCatalogEdit, "kungal-client"),
		`{"fields":{"catalog.work.display_name":"利用者投稿"}}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))

	var env struct {
		Data struct {
			WorkID        int64  `json:"work_id"`
			ProductWorkID int64  `json:"product_work_id"`
			ClaimState    string `json:"claim_state"`
			EventID       int64  `json:"event_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env), string(raw))
	assert.Equal(t, model.ClaimStateKeyPending, env.Data.ClaimState)
	assert.Equal(t, env.Data.WorkID, env.Data.ProductWorkID,
		"an omitted product_work_id makes the registry issue the identity")
	require.NotZero(t, env.Data.EventID)

	var row struct {
		Site        string `gorm:"column:site"`
		OwnerUserID *int64 `gorm:"column:owner_user_id"`
		DisplayName string `gorm:"column:display_name"`
	}
	require.NoError(t, db.Raw(
		`SELECT site, owner_user_id, display_name FROM catalog_work WHERE id = ?`, env.Data.WorkID,
	).Scan(&row).Error)
	assert.Equal(t, "kungal", row.Site, "the tenant is the token client's catalog_site")
	require.NotNil(t, row.OwnerUserID)
	assert.EqualValues(t, 811, *row.OwnerUserID, "the owner is the token's id claim")
	assert.Equal(t, "利用者投稿", row.DisplayName)

	var ev struct {
		ActorUID int64  `gorm:"column:actor_uid"`
		Site     string `gorm:"column:site"`
	}
	require.NoError(t, db.Raw(
		`SELECT actor_uid, site FROM catalog_claim_event WHERE id = ?`, env.Data.EventID,
	).Scan(&ev).Error)
	assert.EqualValues(t, 811, ev.ActorUID)
	assert.Equal(t, "kungal", ev.Site)

	before := env.Data.WorkID
	for _, body := range []string{
		`{"site":"letmoe","fields":{"catalog.work.display_name":"偽装"}}`,
		`{"actor":{"user_id":999},"fields":{"catalog.work.display_name":"偽装"}}`,
	} {
		status, raw = userEditReq(t, app, "POST", UserPrefix+"/works/submit",
			userToken(t, 812, ScopeCatalogEdit, "kungal-client"), body)
		assert.NotEqual(t, fiber.StatusOK, status, string(raw))
	}
	var maxID int64
	require.NoError(t, db.Raw(`SELECT coalesce(max(id), 0) FROM catalog_work`).Scan(&maxID).Error)
	assert.Equal(t, before, maxID, "no spoofed submission minted anything")

	status, raw = userEditReq(t, app, "POST", UserPrefix+"/works/submit",
		userToken(t, 812, ScopeCatalogEdit, "kungal-client"), `{"fields":{}}`)
	assert.Equal(t, fiber.StatusUnprocessableEntity, status, string(raw))
}

func TestUserClaims_TrustedSubmitPublishesDirectly(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)

	type submitEnv struct {
		Data struct {
			WorkID     int64  `json:"work_id"`
			ClaimState string `json:"claim_state"`
			EventID    int64  `json:"event_id"`
		} `json:"data"`
	}

	admin := userTokenRoles(t, 871, ScopeCatalogEdit, "kungal-client", "user", "admin")
	status, raw := userEditReq(t, app, "POST", UserPrefix+"/works/submit",
		admin, `{"fields":{"catalog.work.display_name":"信頼投稿"}}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))

	var env submitEnv
	require.NoError(t, json.Unmarshal(raw, &env), string(raw))
	assert.Equal(t, model.ClaimStateKeyLive, env.Data.ClaimState,
		"a trusted submitter publishes directly, bypassing the review queue")

	var ev struct {
		FromState *int16 `gorm:"column:from_state"`
		ToState   int16  `gorm:"column:to_state"`
	}
	require.NoError(t, db.Raw(
		`SELECT from_state, to_state FROM catalog_claim_event WHERE id = ?`, env.Data.EventID,
	).Scan(&ev).Error)
	require.Nil(t, ev.FromState)
	assert.EqualValues(t, model.ClaimStateLive, ev.ToState,
		"the birth event lands on live, not pending")

	plain := userToken(t, 872, ScopeCatalogEdit, "kungal-client")
	status, raw = userEditReq(t, app, "POST", UserPrefix+"/works/submit",
		plain, `{"fields":{"catalog.work.display_name":"一般投稿"}}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	require.NoError(t, json.Unmarshal(raw, &env), string(raw))
	assert.Equal(t, model.ClaimStateKeyPending, env.Data.ClaimState,
		"a non-trusted submitter still lands in the review queue")
}

func TestUserClaims_OwnerActionsNeedOwnership(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)
	work := seedClaimedWork(t, db, model.ClaimStatePending, 821)

	status, raw := userEditReq(t, app, "POST", userClaimActionPath(work, "withdraw"),
		userToken(t, 822, ScopeCatalogEdit, "kungal-client"), `{}`)
	assert.Equal(t, fiber.StatusForbidden, status, string(raw))

	status, raw = userEditReq(t, app, "POST", userClaimActionPath(work, "withdraw"),
		userToken(t, 821, ScopeCatalogEdit, "letmoe-client"), `{}`)
	assert.Equal(t, fiber.StatusForbidden, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyPending, claimStateOf(t, db, work), "neither refusal moved it")

	owner := userToken(t, 821, ScopeCatalogEdit, "kungal-client")
	status, raw = userEditReq(t, app, "POST", userClaimActionPath(work, "withdraw"), owner, `{}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyDraft, claimStateOf(t, db, work))

	status, raw = userEditReq(t, app, "POST", userClaimActionPath(work, "submit"), owner, `{}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	var res struct {
		Data struct {
			From *string `json:"from_state"`
			To   string  `json:"to_state"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &res), string(raw))
	require.NotNil(t, res.Data.From)
	assert.Equal(t, model.ClaimStateKeyDraft, *res.Data.From)
	assert.Equal(t, model.ClaimStateKeyPending, res.Data.To)

	status, raw = userEditReq(t, app, "POST", userClaimActionPath(work, "submit"), owner, `{}`)
	require.Equal(t, fiber.StatusConflict, status, string(raw))
	var conflict struct {
		Data struct {
			CurrentState string   `json:"current_state"`
			AllowedFrom  []string `json:"allowed_from"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &conflict), string(raw))
	assert.Equal(t, model.ClaimStateKeyPending, conflict.Data.CurrentState)
	assert.NotEmpty(t, conflict.Data.AllowedFrom)

	status, _ = userEditReq(t, app, "POST", userClaimActionPath(work, "annihilate"), owner, `{}`)
	assert.Equal(t, fiber.StatusBadRequest, status)
	status, _ = userEditReq(t, app, "POST", userClaimActionPath(9_000_179, "withdraw"), owner, `{}`)
	assert.Equal(t, fiber.StatusNotFound, status)
}

func TestUserClaims_FirstClaimantAdoptsAFreeDraft(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)
	draft := seedClaimedWork(t, db, model.ClaimStateDraft, 0)

	var before *int64
	require.NoError(t, db.Raw(
		`SELECT owner_user_id FROM catalog_work WHERE id = ?`, draft).Scan(&before).Error)
	require.Nil(t, before, "the imported draft starts ownerless")

	status, raw := userEditReq(t, app, "POST", userClaimActionPath(draft, "publish"),
		userToken(t, 851, ScopeCatalogEdit, "kungal-client"), `{}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyLive, claimStateOf(t, db, draft))

	var after *int64
	require.NoError(t, db.Raw(
		`SELECT owner_user_id FROM catalog_work WHERE id = ?`, draft).Scan(&after).Error)
	require.NotNil(t, after, "moving a free claim adopts it")
	assert.EqualValues(t, 851, *after, "the owner is the token's id claim")

	status, raw = userEditReq(t, app, "POST", userClaimActionPath(draft, "withdraw"),
		userToken(t, 852, ScopeCatalogEdit, "kungal-client"), `{}`)
	assert.Equal(t, fiber.StatusForbidden, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyLive, claimStateOf(t, db, draft), "the refusal moved nothing")

	status, raw = userEditReq(t, app, "POST", userClaimActionPath(draft, "withdraw"),
		userToken(t, 851, ScopeCatalogEdit, "kungal-client"), `{}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyDraft, claimStateOf(t, db, draft))
}

func TestUserClaims_ReviewActionsNeedThePermission(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)
	work := seedClaimedWork(t, db, model.ClaimStatePending, 831)

	for _, action := range []string{"approve", "decline", "ban", "unban"} {
		status, raw := userEditReq(t, app, "POST", userClaimActionPath(work, action),
			userToken(t, 831, ScopeCatalogEdit, "kungal-client"), `{"reason":"x"}`)
		assert.Equalf(t, fiber.StatusForbidden, status, "%s: %s", action, raw)
	}
	assert.Equal(t, model.ClaimStateKeyPending, claimStateOf(t, db, work), "no refusal decided anything")

	status, raw := userEditReq(t, app, "POST", userClaimActionPath(work, "approve"),
		moderatorToken(t, 900), `{}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyLive, claimStateOf(t, db, work))

	var ev struct {
		ActorUID int64  `gorm:"column:actor_uid"`
		Site     string `gorm:"column:site"`
	}
	require.NoError(t, db.Raw(
		`SELECT actor_uid, site FROM catalog_claim_event WHERE work_id = ? ORDER BY id DESC LIMIT 1`, work,
	).Scan(&ev).Error)
	assert.EqualValues(t, 900, ev.ActorUID)
	assert.Equal(t, "kungal", ev.Site)

	second := seedClaimedWork(t, db, model.ClaimStatePending, 832)
	status, raw = userEditReq(t, app, "POST", userClaimActionPath(second, "decline"),
		moderatorToken(t, 900), `{}`)
	assert.Equal(t, fiber.StatusUnprocessableEntity, status, string(raw))

	status, raw = userEditReq(t, app, "POST", userClaimActionPath(second, "decline"),
		moderatorToken(t, 900), `{"reason":"重複投稿"}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyDeclined, claimStateOf(t, db, second))

	third := seedClaimedWork(t, db, model.ClaimStatePending, 833)
	status, raw = userEditReq(t, app, "POST", userClaimActionPath(third, "approve"),
		userTokenRoles(t, 901, ScopeCatalogEdit, "letmoe-client", "user", "moderator"), `{}`)
	assert.Equal(t, fiber.StatusForbidden, status, string(raw))
	assert.Equal(t, model.ClaimStateKeyPending, claimStateOf(t, db, third))
}

func TestUserClaims_Mine(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)

	submit := func(token, name string) int64 {
		status, raw := userEditReq(t, app, "POST", UserPrefix+"/works/submit", token,
			fmt.Sprintf(`{"fields":{"catalog.work.display_name":%q}}`, name))
		require.Equal(t, fiber.StatusOK, status, string(raw))
		var env struct {
			Data struct {
				WorkID int64 `json:"work_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &env), string(raw))
		return env.Data.WorkID
	}
	mine := userToken(t, 841, ScopeCatalogEdit, "kungal-client")
	a := submit(mine, "私の投稿A")
	b := submit(mine, "私の投稿B")
	submit(userToken(t, 842, ScopeCatalogEdit, "kungal-client"), "他人の投稿")
	submit(userToken(t, 841, ScopeCatalogEdit, "letmoe-client"), "別テナントの投稿")

	type minePage struct {
		Data struct {
			Items []struct {
				WorkID      int64  `json:"work_id"`
				Site        string `json:"site"`
				ClaimState  string `json:"claim_state"`
				DisplayName string `json:"display_name"`
				LastEventID int64  `json:"last_event_id"`
			} `json:"items"`
			NextBefore int64 `json:"next_before"`
			Total      int64 `json:"total"`
		} `json:"data"`
	}
	get := func(query, token string) minePage {
		status, raw := userEditReq(t, app, "GET", UserPrefix+"/claims/mine"+query, token, "")
		require.Equal(t, fiber.StatusOK, status, string(raw))
		var page minePage
		require.NoError(t, json.Unmarshal(raw, &page), string(raw))
		return page
	}

	page := get("", mine)
	require.EqualValues(t, 2, page.Data.Total, "only this user's claims, on this user's site")
	require.Len(t, page.Data.Items, 2)
	assert.Equal(t, b, page.Data.Items[0].WorkID, "most recent activity first")
	assert.Equal(t, a, page.Data.Items[1].WorkID)
	for _, it := range page.Data.Items {
		assert.Equal(t, "kungal", it.Site)
		assert.Equal(t, model.ClaimStateKeyPending, it.ClaimState)
	}
	assert.EqualValues(t, 0, page.Data.NextBefore,
		"a short page (fewer than the limit) must not advertise a next page — that is the 'load more clears the list' bug")

	next := get(fmt.Sprintf("?before=%d", page.Data.Items[1].LastEventID), mine)
	assert.Empty(t, next.Data.Items)
	assert.EqualValues(t, 2, next.Data.Total, "the total counts the whole list, not the page")

	assert.EqualValues(t, 2, get("?claim_state=pending", mine).Data.Total)
	assert.EqualValues(t, 0, get("?claim_state=live", mine).Data.Total)

	other := get("", userToken(t, 841, ScopeCatalogEdit, "letmoe-client"))
	require.EqualValues(t, 1, other.Data.Total)
	assert.Equal(t, "letmoe", other.Data.Items[0].Site)

	status, raw := userEditReq(t, app, "GET", UserPrefix+"/claims/mine?claim_state=published", mine, "")
	require.Equal(t, fiber.StatusBadRequest, status, string(raw))
	var errBody struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(raw, &errBody), string(raw))
	assert.Equal(t, msgBadClaimState, errBody.Message)
}

func TestUserClaims_MineSubmittedVsAudited(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)

	submit := func(token, name string) int64 {
		status, raw := userEditReq(t, app, "POST", UserPrefix+"/works/submit", token,
			fmt.Sprintf(`{"fields":{"catalog.work.display_name":%q}}`, name))
		require.Equal(t, fiber.StatusOK, status, string(raw))
		var env struct {
			Data struct {
				WorkID int64 `json:"work_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &env), string(raw))
		return env.Data.WorkID
	}

	submitter := userToken(t, 861, ScopeCatalogEdit, "kungal-client")
	reviewer := moderatorToken(t, 862)

	workID := submit(submitter, "归属区分测试")
	status, raw := userEditReq(t, app, "POST", userClaimActionPath(workID, "decline"), reviewer,
		`{"reason":"区分审核"}`)
	require.Equal(t, fiber.StatusOK, status, string(raw))

	type minePage struct {
		Data struct {
			Items []struct {
				WorkID int64 `json:"work_id"`
			} `json:"items"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	get := func(query, token string) minePage {
		status, raw := userEditReq(t, app, "GET", UserPrefix+"/claims/mine"+query, token, "")
		require.Equal(t, fiber.StatusOK, status, string(raw))
		var page minePage
		require.NoError(t, json.Unmarshal(raw, &page), string(raw))
		return page
	}

	submitted := get("?kind=submitted", submitter)
	require.EqualValues(t, 1, submitted.Data.Total, "the submitter owns exactly one work")
	require.Len(t, submitted.Data.Items, 1)
	require.Equal(t, workID, submitted.Data.Items[0].WorkID)

	audited := get("?kind=audited", reviewer)
	require.EqualValues(t, 1, audited.Data.Total, "the reviewer reviewed exactly one work")
	require.Len(t, audited.Data.Items, 1)
	require.Equal(t, workID, audited.Data.Items[0].WorkID)

	require.Zero(t, get("?kind=audited", submitter).Data.Total, "the submitter reviewed nothing")
	require.Zero(t, get("?kind=submitted", reviewer).Data.Total, "the reviewer submitted nothing")
}

func TestUserClaims_MinePaginatesFullPages(t *testing.T) {
	db := openCatalogTestDB(t)
	resetClaims(t, db)
	app := userClaimApp(db)

	submit := func(token, name string) int64 {
		status, raw := userEditReq(t, app, "POST", UserPrefix+"/works/submit", token,
			fmt.Sprintf(`{"fields":{"catalog.work.display_name":%q}}`, name))
		require.Equal(t, fiber.StatusOK, status, string(raw))
		var env struct {
			Data struct {
				WorkID int64 `json:"work_id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(raw, &env), string(raw))
		return env.Data.WorkID
	}
	mine := userToken(t, 851, ScopeCatalogEdit, "kungal-client")
	for i := 0; i < 22; i++ {
		submit(mine, fmt.Sprintf("投稿%d", i))
	}

	type minePage struct {
		Data struct {
			Items      []json.RawMessage `json:"items"`
			NextBefore int64             `json:"next_before"`
			Total      int64             `json:"total"`
		} `json:"data"`
	}
	get := func(query string) minePage {
		status, raw := userEditReq(t, app, "GET", UserPrefix+"/claims/mine"+query, mine, "")
		require.Equal(t, fiber.StatusOK, status, string(raw))
		var page minePage
		require.NoError(t, json.Unmarshal(raw, &page), string(raw))
		return page
	}

	first := get("")
	require.EqualValues(t, 22, first.Data.Total)
	require.Len(t, first.Data.Items, 20, "the first page caps at the limit")
	require.NotZero(t, first.Data.NextBefore, "a full first page must carry a cursor")

	second := get(fmt.Sprintf("?before=%d", first.Data.NextBefore))
	require.Len(t, second.Data.Items, 2, "the tail page carries the remainder")
	require.EqualValues(t, 22, second.Data.Total, "total counts the whole list, not the page")
	require.Zero(t, second.Data.NextBefore, "the tail page must not advertise a further page")
}
