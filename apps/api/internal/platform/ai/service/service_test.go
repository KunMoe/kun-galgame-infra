package service

import (
	"context"
	"os"
	"testing"

	suitelock "api/internal/platform/ai/dbtest"
	"api/internal/platform/ai/migrate"
	"api/internal/platform/ai/model"
	"api/internal/platform/ai/upstream"
	"api/internal/testsupport/dbtest"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("ai/service")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("ai/service", "cannot connect to test database: %v", err)
	}
	sqlDB, _ := db.DB()
	release := suitelock.AcquireSuiteLock(sqlDB)

	if err := migrate.Run(db); err != nil {
		release()
		dbtest.SkipMainf("ai/service", "ai migration failed: %v", err)
	}
	testDB = db
	code := m.Run()
	release()
	os.Exit(code)
}

func cleanTables(t *testing.T) {
	t.Helper()
	for _, table := range []string{"ai_usage", "ai_route_budget"} {
		if err := testDB.Exec("TRUNCATE " + table + " RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}

type fakeUpstream struct {
	configured bool
	model      string
	result     upstream.ChatResult
	err        error
	errSeq     []error
	calls      int
	lastSystem string
}

func (f *fakeUpstream) Configured() bool { return f.configured }
func (f *fakeUpstream) Model() string    { return f.model }
func (f *fakeUpstream) ChatJSON(_ context.Context, system, _ string, _ int) (upstream.ChatResult, error) {
	f.calls++
	f.lastSystem = system
	if len(f.errSeq) > 0 {
		e := f.errSeq[0]
		f.errSeq = f.errSeq[1:]
		if e != nil {
			return upstream.ChatResult{}, e
		}
	}
	if f.err != nil {
		return upstream.ChatResult{}, f.err
	}
	return f.result, nil
}

type fakeOmni struct {
	configured bool
	model      string
	result     upstream.OmniResult
	err        error
	calls      int
}

func (f *fakeOmni) Configured() bool { return f.configured }
func (f *fakeOmni) Model() string    { return f.model }
func (f *fakeOmni) Moderate(_ context.Context, _ string) (upstream.OmniResult, error) {
	f.calls++
	if f.err != nil {
		return upstream.OmniResult{}, f.err
	}
	return f.result, nil
}

func newLLMOnly(llm upstreamClient) *ModerationService {
	return NewModerationService(testDB, &fakeOmni{}, llm, ModerationOptions{})
}

func insertBudget(t *testing.T, route, site string, cap *int64) {
	t.Helper()
	if err := testDB.Create(&model.AIRouteBudget{Route: route, Site: site, DailyCostCapMicro: cap}).Error; err != nil {
		t.Fatalf("insert budget %s/%s: %v", route, site, err)
	}
}

func insertUsageCost(t *testing.T, site, route string, cost int64) {
	t.Helper()
	if err := testDB.Create(&model.AIUsage{
		Site: site, Route: route, Status: model.StatusOK, CostMicro: cost,
	}).Error; err != nil {
		t.Fatalf("insert usage: %v", err)
	}
}

func usageRows(t *testing.T, site, route string) []model.AIUsage {
	t.Helper()
	var rows []model.AIUsage
	if err := testDB.Where("site = ? AND route = ?", site, route).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("load usage rows: %v", err)
	}
	return rows
}

func ptr[T any](v T) *T { return &v }
