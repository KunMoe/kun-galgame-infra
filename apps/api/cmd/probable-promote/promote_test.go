package main

import (
	"context"
	"io"
	"os"
	"testing"

	"api/internal/platform/catalog/migrate"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/seed"
	"api/internal/testsupport/dbtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testDB *gorm.DB

const (
	ruleRosetta   = "rule:eg-vndb-rosetta"
	ruleTitleYear = "rule:title-year-strict"
	ruleWikiVNDB  = "rule:wiki-vndb-id"
)

func TestMain(m *testing.M) {
	dsn, ok := dbtest.DSN()
	if !ok {
		dbtest.SkipMain("cmd/probable-promote")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		dbtest.SkipMainf("cmd/probable-promote", "cannot connect to test database: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		dbtest.SkipMainf("cmd/probable-promote", "catalog migration failed: %v", err)
	}
	if err := seed.Run(db); err != nil {
		dbtest.SkipMainf("cmd/probable-promote", "catalog seeding failed: %v", err)
	}
	testDB = db
	os.Exit(m.Run())
}

func clean(t *testing.T) {
	t.Helper()
	require.NoError(t, testDB.Exec(
		`TRUNCATE catalog_external_ref, catalog_work RESTART IDENTITY CASCADE`).Error)
}

func seedWork(t *testing.T, id int64) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_work
		(id, medium_id, olang, display_name, content_rating, status, extra, field_provenance, display_nsfw)
		VALUES (?, 1, 'ja', 'W', 0, 0, '{}', '{}', false)`, id).Error)
}

func seedRef(t *testing.T, work int64, source int16, ext string, kind int16, matchedBy string) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO catalog_external_ref
		(entity_type, entity_id, source_id, external_id, link_kind, matched_by)
		VALUES (5, ?, ?, ?, ?, ?)`, work, source, ext, kind, matchedBy).Error)
}

func refState(t *testing.T, work int64, source int16, ext string) (int16, *int64) {
	t.Helper()
	var row struct {
		LinkKind   int16  `gorm:"column:link_kind"`
		VerifiedBy *int64 `gorm:"column:verified_by"`
	}
	require.NoError(t, testDB.Raw(
		`SELECT link_kind, verified_by FROM catalog_external_ref WHERE entity_type=5 AND entity_id=? AND source_id=? AND external_id=?`,
		work, source, ext).Scan(&row).Error)
	return row.LinkKind, row.VerifiedBy
}

func TestPromoteCircleAndIdempotency(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}
	clean(t)
	ctx := context.Background()
	rules := []string{ruleRosetta, ruleTitleYear}

	seedWork(t, 900010)
	seedWork(t, 900020)
	seedWork(t, 900030)
	seedRef(t, 900010, 5, "eg1", model.LinkKindProbable, ruleRosetta)
	seedRef(t, 900020, 3, "bgm1", model.LinkKindProbable, ruleTitleYear)
	seedRef(t, 900030, 2, "v9", model.LinkKindExact, ruleWikiVNDB)

	st, err := runPromote(ctx, testDB, io.Discard, rules, 7, false, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, sumToPromote(st))
	assert.Equal(t, 1, st.Rules[ruleRosetta].ToPromote)
	assert.Equal(t, 1, st.Rules[ruleTitleYear].ToPromote)
	k, _ := refState(t, 900010, 5, "eg1")
	assert.Equal(t, model.LinkKindProbable, k, "dry leaves it probable")

	st, err = runPromote(ctx, testDB, io.Discard, rules, 7, true, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, st.Promoted)
	for _, tc := range []struct {
		work   int64
		source int16
		ext    string
	}{{900010, 5, "eg1"}, {900020, 3, "bgm1"}} {
		k, vb := refState(t, tc.work, tc.source, tc.ext)
		assert.Equal(t, model.LinkKindExact, k, "promoted to exact")
		require.NotNil(t, vb)
		assert.Equal(t, int64(7), *vb, "verified_by = policy executor")
	}
	k, vb := refState(t, 900030, 2, "v9")
	assert.Equal(t, model.LinkKindExact, k)
	assert.Nil(t, vb, "out-of-policy ref never confirmed by this tool")

	st2, err := runPromote(ctx, testDB, io.Discard, rules, 7, true, 0)
	require.NoError(t, err)
	assert.Zero(t, st2.Promoted)
	assert.Equal(t, 2, st2.Already)
}

func TestPromoteConflict(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}
	clean(t)
	ctx := context.Background()
	rules := []string{ruleRosetta}

	seedWork(t, 900001)
	seedWork(t, 900002)
	seedRef(t, 900001, 5, "shared", model.LinkKindExact, ruleRosetta)
	seedRef(t, 900002, 5, "shared", model.LinkKindProbable, ruleRosetta)

	st, err := runPromote(ctx, testDB, io.Discard, rules, 7, true, 0)
	require.NoError(t, err)
	assert.Zero(t, st.Promoted)
	assert.Equal(t, 1, st.Conflict, "probable losing the exact slot is a conflict, not an error")
	assert.Equal(t, 1, st.Already, "the incumbent exact is counted")
	k, _ := refState(t, 900002, 5, "shared")
	assert.Equal(t, model.LinkKindProbable, k, "the contender stays probable")
}

func TestPromoteRefusesCuratedRule(t *testing.T) {
	if testDB == nil {
		t.Skip("no catalog test DB")
	}
	clean(t)
	ctx := context.Background()
	seedWork(t, 900040)
	seedRef(t, 900040, 3, "bgm-curated", model.LinkKindProbable, matchedByCurated)

	_, err := runPromote(ctx, testDB, io.Discard, []string{matchedByCurated}, 7, true, 0)
	require.EqualError(t, err, "curated is the human lane, not a promotion rule")

	_, err = runPromote(ctx, testDB, io.Discard, []string{ruleRosetta, matchedByCurated}, 7, false, 0)
	require.EqualError(t, err, "curated is the human lane, not a promotion rule")

	k, vb := refState(t, 900040, 3, "bgm-curated")
	assert.Equal(t, model.LinkKindProbable, k)
	assert.Nil(t, vb)
}
