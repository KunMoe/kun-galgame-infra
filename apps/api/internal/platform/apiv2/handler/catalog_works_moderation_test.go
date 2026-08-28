package handler

import (
	"context"
	"net/http"
	"testing"

	kunapp "api/internal/app"
	"api/internal/platform/apiv2/collect"
	"api/internal/platform/apiv2/problem"
	"api/internal/platform/apiv2/repr"
	"api/internal/platform/devapi"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

// The gate publishes its verdict as a fiber Local and the handler reads it back
// off the huma context; humafiber copies the request's user values across, so
// the two halves only meet over a real request. Everything below therefore goes
// through app.Test rather than a hand-built context.
func moderationQueueApp(t *testing.T, key string, scopes []string, ownSite string) *fiber.App {
	t.Helper()
	app := fiber.New(kunapp.FiberConfig("kun-catalog"))
	SetupWith(app, Options{
		LookupCredential: func(_ context.Context, raw string) (*devapi.Credential, error) {
			if raw != key {
				return nil, nil
			}
			return &devapi.Credential{KeyID: 53, ClientID: "forum", Tier: devapi.TierInternal, Scopes: scopes}, nil
		},
		// Works is bound and Catalog.Public is not, so a request that clears the
		// gate, the vocabulary and the fence answers 200 — an unambiguous
		// "reached the handler" that no 503 can be confused with.
		Works: func(context.Context, collect.Query) (repr.List[repr.Work], error) {
			return repr.NewList([]repr.Work{}, nil), nil
		},
		Catalog: &Catalog{
			SiteOfAppClient: func(context.Context, string) (string, error) { return ownSite, nil },
		},
	})
	return app
}

// The forum's admin submission queue reads this face with an application key
// and got 400 UNKNOWN_ENUM_VALUE in production, which the page rendered as "no
// pending submissions".
func TestModerationClaimStatesNeedTheOperatorScope(t *testing.T) {
	key := mustV2Key(t)
	operator := moderationQueueApp(t, key,
		[]string{devapi.ScopeCatalogRead, devapi.ScopeClaimEventsRead}, "kungal")

	status, code := walkGet(t, operator, "/v2/catalog/works?claimed=true&claim_state=pending&site=kungal&sort=updated", key)
	require.Equal(t, http.StatusOK, status, "the forum's exact admin-queue query")
	require.Empty(t, code)

	status, code = walkGet(t, operator, "/v2/catalog/works?claim_state=pending", key)
	// 422, like the sibling owner_uid-needs-site rule this mirrors.
	require.Equal(t, http.StatusUnprocessableEntity, status, "site= is mandatory on a moderation state")
	require.Equal(t, problem.CodeValidationFailed, code)

	status, code = walkGet(t, operator, "/v2/catalog/works?claim_state=pending&site=moyu", key)
	require.Equal(t, http.StatusForbidden, status, "another tenant's queue")
	require.Equal(t, problem.CodePermissionRequired, code)

	// Positive control on the public half: the same key still browses normally.
	status, _ = walkGet(t, operator, "/v2/catalog/works?claim_state=live", key)
	require.Equal(t, http.StatusOK, status)
}

// Without the scope the refusal must be the one R1 already gave, so the wider
// set is not discoverable through a distinguishable error.
func TestModerationClaimStatesAreInvisibleToAnOrdinaryKey(t *testing.T) {
	key := mustV2Key(t)
	plain := moderationQueueApp(t, key, []string{devapi.ScopeCatalogRead}, "kungal")

	for _, state := range []string{"pending", "declined", "hidden"} {
		status, code := walkGet(t, plain, "/v2/catalog/works?claim_state="+state+"&site=kungal", key)
		require.Equal(t, http.StatusBadRequest, status, state)
		require.Equal(t, problem.CodeUnknownEnumValue, code, state)
	}
	status, code := walkGet(t, plain, "/v2/catalog/works?claim_state=live", key)
	require.Equal(t, http.StatusOK, status)
	require.Empty(t, code)
}
