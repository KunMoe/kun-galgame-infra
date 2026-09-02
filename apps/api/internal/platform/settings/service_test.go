package settings

import "testing"

func TestETagIsStableAndCanonical(t *testing.T) {
	a := map[string]any{}
	a["z"] = true
	a["m"] = "x"
	b := map[string]any{}
	b["m"] = "x"
	b["z"] = true

	gotA := etagFor(a)
	gotB := etagFor(b)
	if gotA != gotB {
		t.Errorf("etag depends on insertion order: %s vs %s", gotA, gotB)
	}

	c := map[string]any{"m": "y", "z": true}
	if etagFor(c) == gotA {
		t.Errorf("changed value kept etag %s", gotA)
	}

	if len(gotA) != 18 || gotA[0] != '"' || gotA[len(gotA)-1] != '"' {
		t.Errorf("etag format = %q, want quoted 16 hex chars", gotA)
	}
	for _, r := range gotA[1:17] {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'f' {
			continue
		}
		t.Errorf("etag %q is not lowercase hex", gotA)
		break
	}
}
