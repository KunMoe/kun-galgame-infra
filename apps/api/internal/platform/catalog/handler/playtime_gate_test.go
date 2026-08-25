package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaytimeGate_AnyClientBoundToken(t *testing.T) {
	cases := []struct {
		name       string
		uid        uint
		clientID   string
		scope      string
		wantStatus int
	}{
		{name: "no user", wantStatus: fiber.StatusUnauthorized},
		{name: "user without client", uid: 7, wantStatus: fiber.StatusForbidden},
		{name: "client-bound token without playtime scope", uid: 7, clientID: "letmoe", wantStatus: fiber.StatusOK},
		{name: "client-bound token that still has the old scopes", uid: 7, clientID: "kungal", scope: "openid playtime:read playtime:write", wantStatus: fiber.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(PlaytimePrefix, func(c fiber.Ctx) error {
				if tc.uid != 0 {
					c.Locals("user_id", tc.uid)
				}
				c.Locals("token_client_id", tc.clientID)
				c.Locals("user_scope", tc.scope)
				return c.Next()
			}, PlaytimeGate(nil))
			app.Get(PlaytimePrefix+"/mine", func(c fiber.Ctx) error { return c.SendString("ok") })

			resp, err := app.Test(httptest.NewRequest("GET", PlaytimePrefix+"/mine", nil))
			require.NoError(t, err)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}
