package entityintromt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForceRewritesWhatTheHashCallsCurrent(t *testing.T) {
	clean(t)
	vndb, _ := srcIDs(t)
	ctx := context.Background()

	id := mkCharacter(t, "force-me")
	mkCharIntro(t, id, "ja", "主人公のクラスメイト。", vndb)

	opts := Opts{DSN: testDSN, Apply: true, Lane: LaneCharacter}
	st, err := Run(ctx, MockTranslator{}, opts)
	require.NoError(t, err)
	require.Equal(t, 1, st[0].Inserted)

	st, err = Run(ctx, MockTranslator{}, opts)
	require.NoError(t, err)
	require.Equal(t, 1, st[0].SkipUnchanged, "same source and glossary — nothing to redo")

	forced := opts
	forced.Force = true
	st, err = Run(ctx, MockTranslator{}, forced)
	require.NoError(t, err)
	assert.Zero(t, st[0].SkipUnchanged)
	assert.Equal(t, 1, st[0].Retranslated,
		"a prompt change is invisible to the hash, so force is the only way to redo the corpus")
}
