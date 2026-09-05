package repincovers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cover(id int64, kind string, w, h int) Cover {
	return Cover{ID: id, WorkID: 1, Hash: "h" + kind, Kind: kind, Width: w, Height: h, DimsKnown: true}
}

func TestTierOrder(t *testing.T) {
	assert.Equal(t, tier("dig"), tier("main"), "dig and main share one tier - the ruling of 2026-08-08 compares them on resolution, not on which word they use")
	assert.Less(t, tier("dig"), tier(""), "clean art beats an unlabelled upload")
	for _, k := range []string{"pkgfront", "pkgback", "pkgmed", "pkgcontent", "pkgside"} {
		assert.Equal(t, tierIneligible, tier(k), "%s is never a cover", k)
	}
}

func TestSizeNeverBeatsKind(t *testing.T) {
	best := selectWinner([]Cover{
		cover(1, "pkgmed", 1900, 2000),
		cover(2, "pkgback", 1400, 2000),
		cover(3, "dig", 400, 600),
	})
	require.NotNil(t, best)
	assert.Equal(t, "dig", best.Kind)
}

func TestLargestWinsWithinTier(t *testing.T) {
	best := selectWinner([]Cover{
		cover(1, "dig", 400, 600),
		cover(2, "main", 800, 1200),
	})
	require.NotNil(t, best)
	assert.EqualValues(t, 2, best.ID, "same tier - the larger file wins regardless of dig vs main")
}

func TestLandscapeIsNeverEligible(t *testing.T) {
	best := selectWinner([]Cover{
		cover(1, "dig", 1600, 900),
		cover(2, "", 800, 1200),
	})
	require.NotNil(t, best)
	assert.Equal(t, "", best.Kind)
}

func TestPackageArtIsNeverPinned(t *testing.T) {
	assert.Nil(t, selectWinner([]Cover{cover(1, "pkgfront", 2000, 3000)}),
		"the whole pkg family left the ladder: the read face vetoes it, so pinning one "+
			"only writes a pin nothing can elect")
}

func TestUnknownDimsAreNeverEligible(t *testing.T) {
	c := cover(1, "dig", 0, 0)
	c.DimsKnown = false
	assert.Nil(t, selectWinner([]Cover{c}))
}

func TestNoEligibleCoverLeavesTheWorkAlone(t *testing.T) {
	p := planWork(1, []Cover{cover(1, "pkgmed", 1000, 1400)})
	assert.Nil(t, p.New)
	assert.Equal(t, ActionNone, p.Action)
}

func TestPlanActions(t *testing.T) {
	pinned := cover(1, "pkgback", 1200, 1600)
	pinned.Pinned = true

	t.Run("agrees", func(t *testing.T) {
		c := cover(1, "dig", 800, 1200)
		c.Pinned = true
		p := planWork(1, []Cover{c})
		assert.Equal(t, ActionNone, p.Action)
	})

	t.Run("direct pin at or above the target", func(t *testing.T) {
		p := planWork(1, []Cover{pinned, cover(2, "dig", 800, 1080)})
		assert.Equal(t, ActionDirectPin, p.Action)
	})

	t.Run("upscale below the target", func(t *testing.T) {
		p := planWork(1, []Cover{pinned, cover(2, "dig", 700, 1000)})
		assert.Equal(t, ActionUpscale, p.Action)
	})

	t.Run("explicit winners are reported, never pinned", func(t *testing.T) {
		hot := cover(2, "dig", 800, 1200)
		hot.Sexual = 2
		p := planWork(1, []Cover{pinned, hot})
		assert.Equal(t, ActionDeferredNSFW, p.Action)
	})
}

func TestProductNameRoundTrip(t *testing.T) {
	p := Plan{WorkID: 4242, New: &Cover{Hash: "abc123"}}
	id, hash, ok := parseProductName(productName(p))
	require.True(t, ok)
	assert.EqualValues(t, 4242, id)
	assert.Equal(t, "abc123", hash)

	_, _, ok = parseProductName("not-a-product.webp")
	assert.False(t, ok, "an unrelated file in the reinject dir is skipped, not guessed at")
}
