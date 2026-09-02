package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHuma_RenGate_Forbidden(t *testing.T) {
	s := &AdminHumaServer{h: NewAdmin(nil, nil, nil)}
	ctx := context.Background()

	assert403 := func(name string, err error) {
		t.Helper()
		var he *houseError
		require.Truef(t, errors.As(err, &he), "%s: expected houseError, got %v", name, err)
		assert.Equalf(t, http.StatusForbidden, he.status, "%s must 403 without the ren role", name)
	}

	_, err := s.list(ctx, &adminListInput{})
	assert403("list", err)
	_, err = s.delete(ctx, &adminUUIDInput{UUID: "x"})
	assert403("delete", err)
	_, err = s.reclaim(ctx, &adminUUIDInput{UUID: "x"})
	assert403("reclaim", err)
}

func TestRequireRen(t *testing.T) {
	s := &AdminHumaServer{}
	ren := context.WithValue(context.Background(), ctxKeyAdminRoles, []string{"admin", "ren"})
	assert.NoError(t, s.requireRen(ren), "ren caller passes")
	admin := context.WithValue(context.Background(), ctxKeyAdminRoles, []string{"admin"})
	assert.Error(t, s.requireRen(admin), "admin without ren is refused")
	assert.Error(t, s.requireRen(context.Background()), "no roles in ctx → refused")
}

func TestSetupAdmin_RegistersOperations(t *testing.T) {
	api := SetupAdmin(fiber.New(), NewAdmin(nil, nil, nil))
	paths := api.OpenAPI().Paths
	for _, p := range []string{
		"/api/v1/admin/artifact/list",
		"/api/v1/admin/artifact/stats",
		"/api/v1/admin/artifact/{uuid}",
		"/api/v1/admin/artifact/{uuid}/reclaim",
	} {
		assert.NotNilf(t, paths[p], "operation %s must be registered", p)
	}
}
