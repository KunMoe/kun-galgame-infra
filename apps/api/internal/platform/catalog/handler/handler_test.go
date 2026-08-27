package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"api/internal/middleware"
	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/perm"
	"api/internal/platform/catalog/seed"
	siteModel "api/internal/platform/site/model"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

type fakeClientLookup map[string]*siteModel.OAuthClient

func (f fakeClientLookup) FindByClientID(_ context.Context, clientID string) (*siteModel.OAuthClient, error) {
	return f[clientID], nil
}

func userEditClients() fakeClientLookup {
	thirdPartyOwner := uint(4040)
	return fakeClientLookup{
		"kungal-client": {ID: "kungal-client", CatalogSite: "kungal"},
		"letmoe-client": {ID: "letmoe-client", CatalogSite: "letmoe"},
		"thirdparty-letmoe": {
			ID: "thirdparty-letmoe", CatalogSite: "letmoe", OwnerUserID: &thirdPartyOwner,
		},
		"thirdparty-kungal": {
			ID: "thirdparty-kungal", CatalogSite: "kungal", OwnerUserID: &thirdPartyOwner,
		},
	}
}

func TestSetupAdmin_RegistersQueueOperations(t *testing.T) {
	api := SetupAdmin(fiber.New(), nil, nil, nil, nil)
	paths := api.OpenAPI().Paths
	for _, p := range []string{
		"/api/v1/admin/catalog/candidates",
		"/api/v1/admin/catalog/candidates/decide",
		"/api/v1/admin/catalog/names/detach",
		"/api/v1/admin/catalog/proposals",
		"/api/v1/admin/catalog/proposals/{id}/{action}",
		"/api/v1/admin/catalog/refs/probable",
		"/api/v1/admin/catalog/refs/confirm",
		"/api/v1/admin/catalog/refs/reject",
	} {
		assert.NotNilf(t, paths[p], "operation %s must be registered", p)
	}
}

func TestAdminGate_403WithoutRole(t *testing.T) {
	for _, roles := range [][]string{{"user"}, {"admin", "moderator"}} {
		app := fiber.New()
		app.Use("/api/v1/admin/catalog", func(c fiber.Ctx) error {
			c.Locals("user_id", uint(42))
			c.Locals("user_roles", roles)
			return c.Next()
		}, middleware.RequirePermission(perm.Resolver, perm.Review))
		SetupAdmin(app, nil, nil, nil, nil)

		resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/catalog/candidates", nil))
		require.NoError(t, err)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode, "roles %v must not pass", roles)
	}
}

var (
	catalogTestOnce sync.Once
	catalogTestDBH  *gorm.DB
	catalogTestErr  error
)

func openCatalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	catalogTestOnce.Do(func() {
		dsn := os.Getenv("TEST_DATABASE_DSN")
		if dsn == "" {
			dsn = "host=localhost port=5432 user=postgres password=postgres dbname=kun_catalog_test sslmode=disable"
		}
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: glogger.Default.LogMode(glogger.Silent)})
		if err != nil {
			catalogTestErr = fmt.Errorf("catalog test database unreachable: %w", err)
			return
		}
		if err := migrate.Run(db); err != nil {
			catalogTestErr = fmt.Errorf("catalog migrate failed: %w", err)
			return
		}
		if err := seed.Run(db); err != nil {
			catalogTestErr = fmt.Errorf("catalog seed failed: %w", err)
			return
		}
		catalogTestDBH = db
	})
	if catalogTestErr != nil {
		t.Skip(catalogTestErr.Error())
	}
	return catalogTestDBH
}
