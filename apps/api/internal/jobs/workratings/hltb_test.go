package workratings

import (
	"context"
	"testing"

	"api/internal/platform/catalog/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkHltbGame(t *testing.T, id int64, raw string) {
	t.Helper()
	require.NoError(t, testDB.Exec(`INSERT INTO workratings_hltb.games (hltb_id, raw) VALUES (?, ?::jsonb)`, id, raw).Error)
}

func TestHltbLane(t *testing.T) {
	clean(t)
	ctx := context.Background()
	reg, err := resolveRegistry(ctx, testDB)
	require.NoError(t, err)

	wScored := mkWork(t, reg.galgameMedium, "hltb-scored", nil)
	mkAnchor(t, wScored, "1736", reg.hltbSource, model.LinkKindProbable, "rule:hltb-steam")
	// Bucket counts are strings and null means zero — the mirror stores the
	// payload verbatim, so the fixture must reproduce both.
	mkHltbGame(t, 1736, `{"data":{"game":[{"review_score": 88}],
		"userReviews":{"review_count": 100, "100": "66", "90": "24", "80": "10", "5": null}}}`)

	wNoScore := mkWork(t, reg.galgameMedium, "hltb-unscored", nil)
	mkAnchor(t, wNoScore, "200", reg.hltbSource, model.LinkKindProbable, "rule:hltb-steam")
	mkHltbGame(t, 200, `{"data":{"game":[{"review_score": 0}],"userReviews":{"review_count": 0}}}`)

	wMissing := mkWork(t, reg.galgameMedium, "hltb-missing-mirror", nil)
	mkAnchor(t, wMissing, "999", reg.hltbSource, model.LinkKindProbable, "rule:hltb-steam")

	wMulti := mkWork(t, reg.galgameMedium, "hltb-multi", nil)
	mkAnchor(t, wMulti, "300", reg.hltbSource, model.LinkKindProbable, "rule:hltb-steam")
	mkAnchor(t, wMulti, "301", reg.hltbSource, model.LinkKindExact, "rule:hltb-steam")
	mkHltbGame(t, 300, `{"data":{"game":[{"review_score": 50}],"userReviews":{"review_count": 2, "50": "2"}}}`)
	mkHltbGame(t, 301, `{"data":{"game":[{"review_score": 70}],"userReviews":{"review_count": 9, "70": "9"}}}`)

	st, err := Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Equal(t, 4, st.HltbCandidates)
	assert.Equal(t, 1, st.HltbMultiAnchor)
	assert.Equal(t, 1, st.HltbMissingMirror)
	assert.Equal(t, 1, st.HltbNoScore)
	assert.Equal(t, 2, st.HltbPlanned)
	assert.Equal(t, 2, st.HltbWritten)
	assert.Equal(t, 2, st.HltbDistribution)
	assert.Zero(t, st.Errors)

	type row struct {
		Score        float64
		VoteCount    int
		Distribution []byte
	}
	var got row
	require.NoError(t, testDB.Raw(`SELECT score, vote_count, distribution FROM catalog_work_rating
		WHERE work_id = ? AND source_id = ?`, wScored, reg.hltbSource).Scan(&got).Error)
	assert.Equal(t, 88.0, got.Score)
	assert.Equal(t, 100, got.VoteCount)
	assert.JSONEq(t, `{"100": 66, "90": 24, "80": 10}`, string(got.Distribution))

	var multi row
	require.NoError(t, testDB.Raw(`SELECT score, vote_count FROM catalog_work_rating
		WHERE work_id = ? AND source_id = ?`, wMulti, reg.hltbSource).Scan(&multi).Error)
	assert.Equal(t, 70.0, multi.Score)
	assert.Equal(t, 9, multi.VoteCount)

	assert.Zero(t, ratingCount(t, "WHERE work_id = ? AND source_id = ?", wNoScore, reg.hltbSource))

	st2, err := Run(ctx, runOpts(true))
	require.NoError(t, err)
	assert.Zero(t, st2.HltbWritten)
	assert.Equal(t, 2, st2.HltbUnchanged)
}
