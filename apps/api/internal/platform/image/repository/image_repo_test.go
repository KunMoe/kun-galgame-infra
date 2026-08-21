package repository

import "testing"

func TestPublicSexualFoldsTheGradeLadder(t *testing.T) {
	cases := []struct {
		level int
		want  int16
		ok    bool
	}{
		{0, 0, true},
		{1, 1, true},
		{2, 2, true},
		{3, 2, true},
		{4, 0, false},
		{-1, 0, false},
	}
	for _, c := range cases {
		got, ok := publicSexual(c.level)
		if ok != c.ok || got != c.want {
			t.Fatalf("publicSexual(%d) = (%d, %v), want (%d, %v)", c.level, got, ok, c.want, c.ok)
		}
	}
}
