package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/platform/apiv2/protocol"
	"api/internal/platform/devapi"
	"api/internal/platform/settings/keys"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func limitIdentityFor(t *testing.T, prepare func(fiber.Ctx)) (protocol.LimitIdentity, bool) {
	t.Helper()
	app := fiber.New()
	var got protocol.LimitIdentity
	var ok bool
	app.Get("/probe", func(c fiber.Ctx) error {
		prepare(c)
		got, ok = credentialLimitIdentity(c)
		return c.SendString("done")
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/probe", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return got, ok
}

func TestLimitIdentityUserTokenGetsOwnBucket(t *testing.T) {
	id, ok := limitIdentityFor(t, func(c fiber.Ctx) {
		c.Locals("user_id", int64(7))
	})
	require.True(t, ok)
	require.Equal(t, "u7", id.Key)
	require.Equal(t, int(keys.APIV2DefaultRatePerMinute.Get()), id.Rate)
	require.Equal(t, int(keys.APIV2DefaultQuotaPerDay.Get()), id.Quota)
	require.False(t, id.Unlimited)

	id, ok = limitIdentityFor(t, func(c fiber.Ctx) {
		c.Locals("user_id", uint(9))
	})
	require.True(t, ok)
	require.Equal(t, "u9", id.Key)
}

func TestLimitIdentityKeyWinsOverUser(t *testing.T) {
	id, ok := limitIdentityFor(t, func(c fiber.Ctx) {
		devapi.WithCredential(c, &devapi.Credential{KeyID: 53, Tier: devapi.TierInternal})
		c.Locals("user_id", int64(7))
	})
	require.True(t, ok)
	require.Equal(t, "k53", id.Key)
	require.True(t, id.Unlimited)
}

func TestLimitIdentityAnonymousStaysDefault(t *testing.T) {
	_, ok := limitIdentityFor(t, func(fiber.Ctx) {})
	require.False(t, ok, "no credential and no user: fall to the per-IP default")
}
