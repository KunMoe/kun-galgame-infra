package handler

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"api/internal/platform/devapi"
	"api/pkg/errors"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func nsfwGateApp(cred *devapi.Credential) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if cred != nil {
			devapi.WithCredential(c, cred)
		}
		return c.Next()
	})
	app.Get("/v1/catalog/works", RequireNSFWCapability, func(c fiber.Ctx) error {
		return c.SendString("served")
	})
	return app
}

func nsfwGateGet(t *testing.T, app *fiber.App, url string) (int, string, int) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", url, nil))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	if resp.StatusCode == fiber.StatusOK {
		return resp.StatusCode, string(body), 0
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	return resp.StatusCode, env.Message, env.Code
}

func TestNSFWGateRefusesAKeyWithoutTheCapability(t *testing.T) {
	app := nsfwGateApp(&devapi.Credential{KeyID: 1, Scopes: []string{devapi.ScopeCatalogRead}})

	for _, truthy := range []string{"1", "true", "yes", "TRUE", "Yes", "%20true%20"} {
		status, msg, code := nsfwGateGet(t, app, "/v1/catalog/works?nsfw="+truthy)
		assert.Equalf(t, fiber.StatusForbidden, status, "nsfw=%s must be refused", truthy)
		assert.Equalf(t, errors.ErrForbidden, code, "nsfw=%s must use the face's own 403 envelope", truthy)
		assert.Contains(t, msg, "nsfw_allowed")
	}
}

func TestNSFWGateLetsACapableKeyThrough(t *testing.T) {
	app := nsfwGateApp(&devapi.Credential{KeyID: 2, NSFWAllowed: true, Scopes: []string{devapi.ScopeCatalogRead}})
	status, body, _ := nsfwGateGet(t, app, "/v1/catalog/works?nsfw=1")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Equal(t, "served", body)
}

// The capability, not the retired galgame:nsfw scope, is what opens the gate.
func TestNSFWGateDoesNotRequireTheRetiredScope(t *testing.T) {
	app := nsfwGateApp(&devapi.Credential{KeyID: 3, NSFWAllowed: true})
	status, _, _ := nsfwGateGet(t, app, "/v1/catalog/works?nsfw=1")
	assert.Equal(t, fiber.StatusOK, status, "a capable key must pass without holding galgame:nsfw")

	app = nsfwGateApp(&devapi.Credential{KeyID: 4, Scopes: []string{devapi.ScopeGalgameNSFW}})
	status, _, _ = nsfwGateGet(t, app, "/v1/catalog/works?nsfw=1")
	assert.Equal(t, fiber.StatusForbidden, status, "the scope alone must not substitute for the capability")
}

func TestNSFWGateIgnoresRequestsThatDoNotAskForIt(t *testing.T) {
	app := nsfwGateApp(&devapi.Credential{KeyID: 5})
	for _, url := range []string{
		"/v1/catalog/works",
		"/v1/catalog/works?nsfw=",
		"/v1/catalog/works?nsfw=0",
		"/v1/catalog/works?nsfw=false",
		"/v1/catalog/works?nsfw=no",
		"/v1/catalog/works?nsfw=maybe",
	} {
		status, body, _ := nsfwGateGet(t, app, url)
		assert.Equalf(t, fiber.StatusOK, status, "%s must be untouched by the gate", url)
		assert.Equal(t, "served", body)
	}
}

func TestNSFWGateRefusesWhenNoCredentialResolved(t *testing.T) {
	status, _, code := nsfwGateGet(t, nsfwGateApp(nil), "/v1/catalog/works?nsfw=1")
	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Equal(t, errors.ErrForbidden, code)
}
