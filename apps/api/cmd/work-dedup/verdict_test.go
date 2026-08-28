package main

import (
	"testing"
	"time"
)

func day(t *testing.T, s string) *time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return &parsed
}

func TestClassifyPairBuckets(t *testing.T) {
	base := pairRow{A: 1, B: 2, LaneA: "kungal", LaneB: "bgm", SharedNorms: 1}
	cases := []struct {
		name string
		row  func(pairRow) pairRow
		want bucket
	}{
		{"bare", func(r pairRow) pairRow { return r }, bucketBare},
		{"shared ref alone", func(r pairRow) pairRow { r.RefOverlap = true; return r }, bucketAuto},
		{"label overlap alone", func(r pairRow) pairRow { r.LabelOverlap = true; return r }, bucketAuto},
		{"two shared norms alone", func(r pairRow) pairRow { r.SharedNorms = 2; return r }, bucketAuto},
		{"near dates alone", func(r pairRow) pairRow {
			r.DateA, r.DateB = day(t, "2015-08-10"), day(t, "2015-09-25")
			return r
		}, bucketAuto},
		{"far dates in one year are not corroboration", func(r pairRow) pairRow {
			r.DateA, r.DateB = day(t, "2015-01-10"), day(t, "2015-12-25")
			return r
		}, bucketBare},
		{"year mismatch vetoes the label corroborator", func(r pairRow) pairRow {
			r.LabelOverlap = true
			r.DateA, r.DateB = day(t, "2015-08-10"), day(t, "2019-08-10")
			return r
		}, bucketDateClash},
		{"anchor conflict beats every corroborator", func(r pairRow) pairRow {
			r.AnchorConflict, r.RefOverlap, r.LabelOverlap = true, true, true
			return r
		}, bucketAnchorConflict},
		{"release conflict beats every corroborator", func(r pairRow) pairRow {
			r.RelConflict, r.RefOverlap, r.LabelOverlap = true, true, true
			return r
		}, bucketRelConflict},
		{"two claimed works are never machine-merged", func(r pairRow) pairRow {
			r.LaneB, r.RefOverlap = "kungal", true
			return r
		}, bucketBothKungal},
		{"dlsite edition splits are out of scope", func(r pairRow) pairRow {
			r.LaneA, r.LaneB, r.AnchorConflict = "dlsite", "dlsite", true
			return r
		}, bucketOutOfScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyPair(tc.row(base)); got != tc.want {
				t.Fatalf("classifyPair = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyDemotesBridgedComponents(t *testing.T) {
	rows := []pairRow{
		{A: 1, B: 2, LaneA: "kungal", LaneB: "bgm", RefOverlap: true, SharedNorms: 1},
		{A: 2, B: 3, LaneA: "bgm", LaneB: "kungal", RefOverlap: true, SharedNorms: 1},
		{A: 10, B: 11, LaneA: "bgm", LaneB: "vndb", RefOverlap: true, SharedNorms: 1},
	}
	verdicts, groups := classify(rows)
	if verdicts[0] != bucketBridged || verdicts[1] != bucketBridged {
		t.Fatalf("a component holding two claimed works must be demoted: %v", verdicts)
	}
	if verdicts[2] != bucketAuto {
		t.Fatalf("the untouched component must stay auto: %v", verdicts)
	}
	if len(groups) != 1 || groups[0].survivor != 10 {
		t.Fatalf("groups: %+v", groups)
	}
}

func TestClassifySurvivorSelection(t *testing.T) {
	cases := []struct {
		name string
		row  pairRow
		want int64
	}{
		{
			name: "claimed work beats anchor count",
			row: pairRow{A: 10, B: 11, LaneA: "bgm", LaneB: "kungal",
				AnchorsA: 5, AnchorsB: 0, RefOverlap: true, SharedNorms: 1},
			want: 11,
		},
		{
			name: "anchors decide when neither is claimed",
			row: pairRow{A: 20, B: 21, LaneA: "bgm", LaneB: "vndb",
				AnchorsA: 1, AnchorsB: 3, RefOverlap: true, SharedNorms: 1},
			want: 21,
		},
		{
			name: "an anchor tie keeps the lower id",
			row: pairRow{A: 30, B: 31, LaneA: "bgm", LaneB: "vndb",
				AnchorsA: 1, AnchorsB: 1, RefOverlap: true, SharedNorms: 1},
			want: 30,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, groups := classify([]pairRow{tc.row})
			if len(groups) != 1 {
				t.Fatalf("groups: %+v", groups)
			}
			if groups[0].survivor != tc.want {
				t.Fatalf("survivor = %d, want %d", groups[0].survivor, tc.want)
			}
			if len(groups[0].sources) != 1 || groups[0].sources[0] == tc.want {
				t.Fatalf("sources: %+v", groups[0].sources)
			}
		})
	}
}

func TestPairEvidenceNamesTheCorroborators(t *testing.T) {
	got := pairEvidence(pairRow{
		RefOverlap: true, SharedNorms: 1,
		DateA: day(t, "2015-08-10"), DateB: day(t, "2015-09-25"),
	})
	if want := "ref+date(2015-08-10~2015-09-25)"; got != want {
		t.Fatalf("evidence = %q, want %q", got, want)
	}
	if got := pairEvidence(pairRow{LabelOverlap: true, SharedNorms: 2}); got != "label+norms=2" {
		t.Fatalf("evidence = %q, want %q", got, "label+norms=2")
	}
	if got := pairEvidence(pairRow{SharedNorms: 1}); got != "none" {
		t.Fatalf("evidence = %q, want %q", got, "none")
	}
}
