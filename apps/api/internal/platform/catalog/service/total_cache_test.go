package service

import (
	"strconv"
	"testing"
	"time"

	"api/internal/platform/settings/keys"
)

func TestTotalsCacheMissOnEmpty(t *testing.T) {
	c := newTotalsCache()
	if v, ok := c.get("missing"); ok || v != 0 {
		t.Fatalf("empty get = %d, %v, want miss", v, ok)
	}
}

func TestTotalsCachePutThenGet(t *testing.T) {
	c := newTotalsCache()
	c.put("k", 7)
	v, ok := c.get("k")
	if !ok || v != 7 {
		t.Fatalf("get = %d, %v, want 7, true", v, ok)
	}
}

func TestTotalsCacheExpiry(t *testing.T) {
	c := newTotalsCache()
	t0 := time.Unix(1_700_000_000, 0)
	now := t0
	c.now = func() time.Time { return now }

	c.put("exp", 3)
	if v, ok := c.get("exp"); !ok || v != 3 {
		t.Fatalf("before expiry get = %d, %v, want 3, true", v, ok)
	}

	now = t0.Add(time.Duration(keys.CatalogTotalsCacheTTLSeconds.Get())*time.Second + time.Nanosecond)
	if v, ok := c.get("exp"); ok || v != 0 {
		t.Fatalf("after expiry get = %d, %v, want miss", v, ok)
	}
	if _, exists := c.entries["exp"]; exists {
		t.Fatal("expired entry was not deleted")
	}
}

func TestTotalsCacheCapReset(t *testing.T) {
	c := newTotalsCache()
	for i := 0; i < totalsMaxEntries; i++ {
		c.put(strconv.Itoa(i), int64(i))
	}
	if len(c.entries) != totalsMaxEntries {
		t.Fatalf("fill len = %d, want %d", len(c.entries), totalsMaxEntries)
	}

	c.put("overflow", 99)
	if len(c.entries) != 1 {
		t.Fatalf("after overflow len = %d, want 1 (whole-map reset then insert)", len(c.entries))
	}
	if v, ok := c.get("overflow"); !ok || v != 99 {
		t.Fatalf("overflow get = %d, %v, want 99, true", v, ok)
	}
	if _, ok := c.get("0"); ok {
		t.Fatal("pre-reset key survived whole-map reset")
	}
}
