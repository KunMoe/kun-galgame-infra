package entityintromt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamedEntityIDsAreThePopulation(t *testing.T) {
	clean(t)
	vndb, _ := srcIDs(t)
	ctx := context.Background()

	wanted := mkCharacter(t, "wanted")
	mkCharIntro(t, wanted, "ja", "主人公のクラスメイト。", vndb)
	other := mkCharacter(t, "other")
	mkCharIntro(t, other, "ja", "幼馴染の妹。", vndb)

	st, err := Run(ctx, MockTranslator{}, Opts{
		DSN: testDSN, Apply: true, Lane: LaneCharacter, EntityIDs: []int64{wanted},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, st[0].Candidates, "the named list is the population, not a filter over it")
	assert.Equal(t, 1, st[0].Inserted)

	var untouched int64
	require.NoError(t, testDB.Raw(
		`SELECT count(*) FROM catalog_character_intro WHERE character_id = ? AND provenance = 1`,
		other).Scan(&untouched).Error)
	assert.Zero(t, untouched, "an unnamed character must not be written")
}

func TestEntityIDsWithoutALaneAreRefused(t *testing.T) {
	clean(t)
	_, err := Run(context.Background(), MockTranslator{}, Opts{
		DSN: testDSN, Apply: true, EntityIDs: []int64{1, 2, 3},
	})
	require.Error(t, err, "character 42, person 42 and label 42 are three different rows")
	assert.Contains(t, err.Error(), "--entity-ids needs --lane")
}
