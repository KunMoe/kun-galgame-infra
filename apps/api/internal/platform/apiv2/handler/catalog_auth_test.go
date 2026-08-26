package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/devapi"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func testAppLookup(t *testing.T, lookup func(context.Context, string) (*devapi.Credential, error)) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	SetupWith(app, Options{LookupCredential: lookup})
	return app
}

func authGET(t *testing.T, app *fiber.App, path, token string) (int, problem.Problem) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var p problem.Problem
	require.NoError(t, json.Unmarshal(body, &p), string(body))
	return resp.StatusCode, p
}

func mustV2Key(t *testing.T) string {
	t.Helper()
	k, err := devapi.GenerateV2Key(true)
	require.NoError(t, err)
	return k
}

func TestCatalogAuthLookupRejectsUnknownAndUnscopedKeys(t *testing.T) {
	sfw := mustV2Key(t)
	unscoped := mustV2Key(t)
	missing := mustV2Key(t)
	lookup := func(_ context.Context, raw string) (*devapi.Credential, error) {
		switch raw {
		case sfw:
			return &devapi.Credential{KeyID: 1, Scopes: []string{devapi.ScopeCatalogRead}}, nil
		case unscoped:
			return &devapi.Credential{KeyID: 2}, nil
		default:
			return nil, nil
		}
	}
	app := testAppLookup(t, lookup)

	status, p := authGET(t, app, "/v2/catalog/works", "not-a-key")
	require.Equal(t, 401, status)
	require.Equal(t, problem.CodeInvalidCredential, p.Code)

	status, p = authGET(t, app, "/v2/catalog/works", "nm_live_legacy")
	require.Equal(t, 401, status)
	require.Equal(t, problem.CodeInvalidCredential, p.Code)

	status, p = authGET(t, app, "/v2/catalog/works", missing)
	require.Equal(t, 401, status)
	require.Equal(t, problem.CodeInvalidCredential, p.Code)

	status, p = authGET(t, app, "/v2/catalog/works", unscoped)
	require.Equal(t, 403, status)
	require.Equal(t, problem.CodeScopeRequired, p.Code)

	status, p = authGET(t, app, "/v2/catalog/works?nsfw=true", sfw)
	require.Equal(t, 503, status)
	require.Equal(t, problem.CodeServiceUnavailable, p.Code)
}

func TestCatalogAuthLookupAllowsNSFWForAnyKey(t *testing.T) {
	plain := mustV2Key(t)
	lookup := func(_ context.Context, raw string) (*devapi.Credential, error) {
		if raw == plain {
			return &devapi.Credential{KeyID: 3, Scopes: []string{devapi.ScopeCatalogRead}}, nil
		}
		return nil, nil
	}
	app := testAppLookup(t, lookup)

	status, p := authGET(t, app, "/v2/catalog/works?nsfw=true", plain)
	require.Equal(t, 503, status)
	require.Equal(t, problem.CodeServiceUnavailable, p.Code)

	status, p = authGET(t, app, "/v2/catalog/works/1?nsfw=true", plain)
	require.Equal(t, 503, status)
	require.Equal(t, problem.CodeServiceUnavailable, p.Code)

	status, p = authGET(t, app, "/v2/catalog/works?nsfw=yes", plain)
	require.Equal(t, 400, status)
	require.Equal(t, problem.CodeInvalidParameter, p.Code)
}

func TestCatalogAuthLookupStoreFailureIs503(t *testing.T) {
	app := testAppLookup(t, func(context.Context, string) (*devapi.Credential, error) {
		return nil, errors.New("redis down")
	})
	status, p := authGET(t, app, "/v2/catalog/works", mustV2Key(t))
	require.Equal(t, 503, status)
	require.Equal(t, problem.CodeServiceUnavailable, p.Code)
}

func TestCatalogAuthStubServesNSFWWithoutACapability(t *testing.T) {
	app := testApp(t)
	status, p := authGET(t, app, "/v2/catalog/works?nsfw=true", "test")
	require.Equal(t, 503, status)
	require.Equal(t, problem.CodeServiceUnavailable, p.Code)
}
