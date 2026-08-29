package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	authModel "api/internal/platform/auth/model"
	"api/internal/platform/devapi"
	"api/internal/platform/site/model"
	"api/internal/platform/site/repository"
	storeModel "api/internal/platform/store/model"
	"api/internal/testsupport/dbtest"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

const suiteLockKey = 0x73697465

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("site/service")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("site/service", "cannot connect to test database: %v", err)
	}
	sqlDB, _ := db.DB()
	release := acquireSuiteLock(sqlDB)

	// The four tables below are not site models, but EnsureDeletable counts rows
	// in every table that carries a client_id before it lets the row go. Without
	// them the guard would only ever be tested against tables that do not exist.
	models := []any{
		&model.Site{}, &model.OAuthClient{},
		&authModel.Session{}, &authModel.AuthorizationCode{},
		&storeModel.PurchaseLink{}, &storeModel.CouponLink{},
	}
	models = append(models, devapi.Models()...)
	if err := db.AutoMigrate(models...); err != nil {
		release()
		dbtest.SkipMainf("site/service", "migration failed: %v", err)
	}

	testDB = db
	code := m.Run()
	release()
	os.Exit(code)
}

func acquireSuiteLock(db *sql.DB) func() {
	if db == nil {
		return func() {}
	}
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return func() {}
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", suiteLockKey); err != nil {
		_ = conn.Close()
		return func() {}
	}
	return func() {
		_, _ = conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", suiteLockKey)
		_ = conn.Close()
	}
}

// TestDeleteOAuthClientGuard pins the site console's delete to the same rule the
// developer console enforces. It shipped without one, which made
// DELETE /api/v1/oauth/clients/:id a way around every check the other door makes.
func TestDeleteOAuthClientGuard(t *testing.T) {
	svc := NewSiteService(
		repository.NewSiteRepository(testDB),
		repository.NewOAuthClientRepository(testDB),
		devapi.NewRepository(testDB),
	)
	ctx := context.Background()
	owner := uint(1)
	archived := time.Now().UTC()

	testDB.Exec(`DELETE FROM developer_api_keys WHERE client_id LIKE 'sitetest-%'`)
	testDB.Exec(`DELETE FROM oauth_clients WHERE id LIKE 'sitetest-%'`)
	testDB.Exec(`DELETE FROM sites WHERE domain LIKE 'sitetest-%'`)

	archivedApp := func(c *model.OAuthClient) {
		c.OwnerUserID = &owner
		c.DevArchivedAt = &archived
	}
	noPrep := func(*testing.T, string) {}

	cases := []struct {
		name  string
		build func(c *model.OAuthClient)
		prep  func(t *testing.T, clientID string)
		want  error
	}{
		{"a client that can sign users in", func(c *model.OAuthClient) {
			c.RedirectURIs = datatypes.JSON([]byte(`["https://example.invalid/cb"]`))
		}, noPrep, devapi.ErrAppHasReferences},
		{"a live developer application", func(c *model.OAuthClient) {
			c.OwnerUserID = &owner
		}, noPrep, devapi.ErrAppNotArchived},
		{"an archived developer application still holding a key", archivedApp, func(t *testing.T, clientID string) {
			key := devapi.DeveloperAPIKey{
				ClientID:  clientID,
				Name:      "k",
				KeyHash:   "sitetest-hash-" + clientID,
				KeyPrefix: "nmk_test_",
				Last4:     "abcd",
				Scopes:    datatypes.JSON([]byte(`["catalog:read"]`)),
			}
			if err := testDB.Create(&key).Error; err != nil {
				t.Fatalf("create key: %v", err)
			}
		}, devapi.ErrAppHasReferences},
		{"a client bound to a site", archivedApp, func(t *testing.T, clientID string) {
			site := model.Site{Name: "sitetest " + clientID, Domain: clientID + ".example.invalid"}
			if err := testDB.Create(&site).Error; err != nil {
				t.Fatalf("create site: %v", err)
			}
			if err := testDB.Model(&model.OAuthClient{}).Where("id = ?", clientID).
				Update("site_id", site.ID).Error; err != nil {
				t.Fatalf("bind to site: %v", err)
			}
		}, devapi.ErrAppHasReferences},
		{"an archived developer application nothing points at", archivedApp, noPrep, nil},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clientID := "sitetest-" + string(rune('a'+i))
			client := &model.OAuthClient{
				ID:              clientID,
				Name:            tc.name,
				Secret:          model.HashOAuthClientSecret("s"),
				RedirectURIs:    datatypes.JSON([]byte(`[]`)),
				Grants:          datatypes.JSON([]byte(`[]`)),
				DevTier:         devapi.TierFree,
				DevReviewStatus: devapi.AppReviewApproved,
			}
			tc.build(client)
			if err := testDB.Create(client).Error; err != nil {
				t.Fatalf("create client: %v", err)
			}
			// The client row goes before the site row it points at: oauth_clients
			// has a real foreign key onto sites, so dropping the site first fails
			// silently and leaves a row that collides with idx_sites_domain on the
			// next run.
			t.Cleanup(func() {
				testDB.Exec(`DELETE FROM developer_api_keys WHERE client_id = ?`, clientID)
				testDB.Exec(`DELETE FROM oauth_clients WHERE id = ?`, clientID)
				testDB.Exec(`DELETE FROM sites WHERE domain = ?`, clientID+".example.invalid")
			})

			tc.prep(t, clientID)

			if err := svc.DeleteOAuthClient(ctx, clientID); !errors.Is(err, tc.want) {
				t.Fatalf("delete = %v, want %v", err, tc.want)
			}

			_, getErr := svc.GetOAuthClient(ctx, clientID)
			if tc.want != nil {
				if getErr != nil {
					t.Errorf("a refused delete must leave the row in place, got %v", getErr)
				}
				return
			}
			if !errors.Is(getErr, gorm.ErrRecordNotFound) {
				t.Errorf("deleted client row = %v, want gorm.ErrRecordNotFound", getErr)
			}
		})
	}
}
