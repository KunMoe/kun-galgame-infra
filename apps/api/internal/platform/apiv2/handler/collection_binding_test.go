package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"api/internal/platform/apiv2/problem"
	"api/internal/platform/devapi"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

var embeddedCollectionPaths = []string{
	"/v2/catalog/redirects",
	"/v2/catalog/calendar",
	"/v2/catalog/credit-names",
	"/v2/catalog/search",
	"/v2/me/playtimes",
	"/v2/me/proposals",
}

func bindingApp(t *testing.T) (*fiber.App, huma.API, string, string) {
	t.Helper()
	appKey := mustV2Key(t)
	userToken := "binding-user-token"
	app := fiber.New(fiber.Config{ErrorHandler: problem.WriteFiberError})
	api := SetupWith(app, Options{
		LookupCredential: func(_ context.Context, raw string) (*devapi.Credential, error) {
			if raw == appKey {
				return &devapi.Credential{KeyID: 1, Scopes: []string{devapi.ScopeCatalogRead}}, nil
			}
			return nil, nil
		},
		LookupUser: func(_ context.Context, raw string) (UserIdentity, error) {
			if raw != userToken {
				return UserIdentity{}, os.ErrPermission
			}
			return UserIdentity{UID: 7, ClientID: "kungal-client"}, nil
		},
		LookupSite: func(context.Context, string) (SiteBinding, error) { return SiteBinding{Site: "kungal"}, nil },
	})
	return app, api, appKey, userToken
}

func TestEmbeddedCollectionParamsAreDocumented(t *testing.T) {
	_, api, _, _ := bindingApp(t)
	doc := api.OpenAPI()
	want := []string{
		"cursor", "limit", "view", "include", "fields",
		"ids", "refs", "include_total", "facets", "sort", "nsfw",
	}
	for _, path := range embeddedCollectionPaths {
		item := doc.Paths[path]
		require.NotNil(t, item, path)
		require.NotNil(t, item.Get, path)
		got := map[string]bool{}
		for _, p := range item.Get.Parameters {
			if p != nil && p.In == "query" {
				got[p.Name] = true
			}
		}
		for _, name := range want {
			require.True(t, got[name], "%s is missing the %s query parameter", path, name)
		}
	}
}

func TestEmbeddedCollectionParamsBind(t *testing.T) {
	app, _, appKey, userToken := bindingApp(t)
	for _, path := range embeddedCollectionPaths {
		token := appKey
		if strings.HasPrefix(path, "/v2/me/") {
			token = userToken
		}
		req := httptest.NewRequest(http.MethodGet, path+"?limit=101", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req)
		require.NoError(t, err, path)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, path)
		var p problem.Problem
		require.NoError(t, json.Unmarshal(body, &p), path+" "+string(body))
		require.Equal(t, http.StatusBadRequest, resp.StatusCode, path+" "+string(body))
		require.Equal(t, problem.CodeLimitTooLarge, p.Code, path)
	}
}
