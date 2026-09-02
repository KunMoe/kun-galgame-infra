package settings

import "testing"

func TestForSite(t *testing.T) {
	k := Int(Meta{Name: "t.n", DescEN: "e", DescZH: "z"}, 7)
	if k.ForSite(7) != k.Get() {
		t.Errorf("ForSite with no overlay = %d, want Get() %d", k.ForSite(7), k.Get())
	}

	k.applySites(map[uint]any{7: int64(42)})
	if k.ForSite(7) != 42 {
		t.Errorf("ForSite(7) after overlay = %d, want 42", k.ForSite(7))
	}
	if k.ForSite(8) != k.Get() {
		t.Errorf("ForSite(8) without overlay = %d, want Get() %d", k.ForSite(8), k.Get())
	}
	if k.Get() != 7 {
		t.Errorf("Get() after applySites = %d, want 7", k.Get())
	}

	k.applySites(map[uint]any{})
	if k.ForSite(7) != k.Get() {
		t.Errorf("ForSite(7) after empty overlay = %d, want Get() %d", k.ForSite(7), k.Get())
	}
}
