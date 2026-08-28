package workplaytime

import (
	"context"
	"fmt"
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkHltbAnchor(t *testing.T, workID int64, hltbID string, source, kind int16) {
	t.Helper()
	require.NoError(t, testDB.Create(&model.CatalogExternalRef{
		EntityType: model.EntityTypeWork, EntityID: workID, SourceID: source,
		ExternalID: hltbID, LinkKind: kind, MatchedBy: "rule:hltb-steam",
	}).Error)
}

func mkHltbGame(t *testing.T, id int64, med, main, count int64) {
	t.Helper()
	raw := fmt.Sprintf(`{"data":{"game":[{"comp_main_med": %d, "comp_main": %d, "comp_main_count": %d}]}}`, med, main, count)
	require.NoError(t, testDB.Exec(`INSERT INTO workplaytime_hltb.games (hltb_id, raw) VALUES (?, ?::jsonb)`, id, raw).Error)
}

func TestHltbLane(t *testing.T) {
	clean(t)
	ctx := context.Background()
	medium := mediumID(t)
	hltb := sourceID(t, "howlongtobeat")

	wMed := mkWork(t, medium, "hltb-median", nil)
	mkHltbAnchor(t, wMed, "1736", hltb, model.LinkKindProbable)
	mkHltbGame(t, 1736, 207870, 197852, 44)

	wAvg := mkWork(t, medium, "hltb-avg-fallback", nil)
	mkHltbAnchor(t, wAvg, "200", hltb, model.LinkKindExact)
	mkHltbGame(t, 200, 0, 3600, 7)

	wNoVotes := mkWork(t, medium, "hltb-no-votes", nil)
	mkHltbAnchor(t, wNoVotes, "300", hltb, model.LinkKindProbable)
	mkHltbGame(t, 300, 7200, 7200, 0)

	wCap := mkWork(t, medium, "hltb-over-cap", nil)
	mkHltbAnchor(t, wCap, "400", hltb, model.LinkKindProbable)
	mkHltbGame(t, 400, 4000*3600, 4000*3600, 3)

	wMulti := mkWork(t, medium, "hltb-multi-anchor", nil)
	mkHltbAnchor(t, wMulti, "500", hltb, model.LinkKindProbable)
	mkHltbAnchor(t, wMulti, "501", hltb, model.LinkKindProbable)
	mkHltbGame(t, 500, 600, 600, 2)
	mkHltbGame(t, 501, 1200, 1200, 9)

	st, err := Run(ctx, Opts{DSN: testDSN, HltbDSN: hltbTestDSN, Source: "hltb", Apply: true})
	require.NoError(t, err)
	assert.Equal(t, 4, st.HltbAnchored)
	assert.Equal(t, 1, st.HltbRejected)
	assert.Equal(t, 3, st.HltbPlanned)
	assert.Equal(t, 3, st.HltbWritten)
	assert.Zero(t, st.Errors)

	type row struct {
		Minutes   int
		VoteCount int
	}
	get := func(workID int64) *row {
		var r []row
		require.NoError(t, testDB.Raw(`SELECT minutes, vote_count FROM catalog_work_playtime
			WHERE work_id = ? AND source_id = ?`, workID, hltb).Scan(&r).Error)
		if len(r) == 0 {
			return nil
		}
		return &r[0]
	}
	require.NotNil(t, get(wMed))
	assert.Equal(t, 3465, get(wMed).Minutes)
	assert.Equal(t, 44, get(wMed).VoteCount)
	require.NotNil(t, get(wAvg))
	assert.Equal(t, 60, get(wAvg).Minutes)
	assert.Nil(t, get(wNoVotes))
	assert.Nil(t, get(wCap))
	require.NotNil(t, get(wMulti))
	assert.Equal(t, 20, get(wMulti).Minutes)
	assert.Equal(t, 9, get(wMulti).VoteCount)

	st2, err := Run(ctx, Opts{DSN: testDSN, HltbDSN: hltbTestDSN, Source: "hltb", Apply: true})
	require.NoError(t, err)
	assert.Zero(t, st2.HltbWritten)
	assert.Equal(t, 3, st2.HltbUnchanged)
}

func TestHltbLaneRequiresDSN(t *testing.T) {
	_, err := Run(context.Background(), Opts{DSN: testDSN, Source: "hltb"})
	require.Error(t, err)
}
