package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestGroupWorks(t *testing.T) {
	ids := []int64{5, 1, 3, 9, 7}
	edges := []siblingEdge{{A: 1, B: 3}, {A: 3, B: 9}, {A: 5, B: 7}, {A: 7, B: 100}}
	got := groupWorks(ids, edges)
	want := [][]int64{{1, 3, 9}, {5, 7}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("groupWorks = %v, want %v (edges to non-members are ignored)", got, want)
	}
}

func TestBuildBatchesKeepsPopularityOrderAndCarriesContext(t *testing.T) {
	cands := []mtCandidate{
		{WorkID: 40, JaTitle: "ひとりぼっち", PopScore: 900},
		{WorkID: 10, JaTitle: "ロマンス", PopScore: 500},
		{WorkID: 11, JaTitle: "ロマンス2", PopScore: 100},
	}
	edges := []siblingEdge{{A: 10, B: 11}}
	known := map[int64][]string{11: {"ロマンス0 → 罗曼史0"}, 40: nil}

	batches := buildBatches(cands, edges, known, 12)
	if len(batches) != 2 {
		t.Fatalf("batches = %d, want 2", len(batches))
	}
	// The lone popular work outranks the series whose best member is second.
	if len(batches[0].Members) != 1 || batches[0].Members[0].WorkID != 40 {
		t.Fatalf("batches[0] = %+v", batches[0].Members)
	}
	if len(batches[1].Members) != 2 {
		t.Fatalf("batches[1] = %+v, want the series together", batches[1].Members)
	}
	// The sibling's existing zh title reaches the batch that translates BOTH
	// members, not only the member it hangs on.
	if !reflect.DeepEqual(batches[1].Known, []string{"ロマンス0 → 罗曼史0"}) {
		t.Fatalf("batches[1].Known = %v", batches[1].Known)
	}
}

func TestBuildBatchesSplitsOversizedSeries(t *testing.T) {
	var cands []mtCandidate
	var edges []siblingEdge
	for i := int64(1); i <= 5; i++ {
		cands = append(cands, mtCandidate{WorkID: i, JaTitle: "作品"})
		if i > 1 {
			edges = append(edges, siblingEdge{A: 1, B: i})
		}
	}
	batches := buildBatches(cands, edges, nil, 2)
	if len(batches) != 3 {
		t.Fatalf("batches = %d, want 3 chunks of at most 2", len(batches))
	}
	total := 0
	for _, b := range batches {
		if len(b.Members) > 2 {
			t.Fatalf("chunk too large: %+v", b.Members)
		}
		total += len(b.Members)
	}
	if total != 5 {
		t.Fatalf("members = %d, want 5", total)
	}
}

func TestUserMessageCarriesSeriesContext(t *testing.T) {
	b := mtBatch{
		Members: []mtCandidate{{WorkID: 1, JaTitle: "ロマンス"}, {WorkID: 2, JaTitle: "ロマンス2"}},
		Known:   []string{"ロマンス0 → 罗曼史0"},
	}
	msg := userMessage(b.Members[0], b.Known, batchmatesOf(b, 1))
	for _, want := range []string{"ロマンス", "罗曼史0", "ロマンス2"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("prompt missing %q:\n%s", want, msg)
		}
	}
	if strings.Count(msg, "ロマンス2") != 1 {
		t.Fatalf("the batchmate must appear once, as context:\n%s", msg)
	}
}

func TestAuditSeriesFlagsPrefixDisagreement(t *testing.T) {
	rows := []seriesTitleRow{
		{SeriesID: 1, SeriesName: "ロマンス", WorkID: 1, JaTitle: "ロマンス 上", ZhTitle: "罗曼史 上"},
		{SeriesID: 1, SeriesName: "ロマンス", WorkID: 2, JaTitle: "ロマンス 下", ZhTitle: "浪漫 下"},
		{SeriesID: 2, SeriesName: "そら", WorkID: 3, JaTitle: "そら 上", ZhTitle: "天空 上"},
		{SeriesID: 2, SeriesName: "そら", WorkID: 4, JaTitle: "そら 下", ZhTitle: "天空 下"},
		{SeriesID: 3, SeriesName: "単発", WorkID: 5, JaTitle: "単発", ZhTitle: "单发"},
		{SeriesID: 4, SeriesName: "無関係", WorkID: 6, JaTitle: "AAA", ZhTitle: "甲"},
		{SeriesID: 4, SeriesName: "無関係", WorkID: 7, JaTitle: "BBB", ZhTitle: "乙"},
	}
	got := auditSeries(rows)
	if len(got) != 1 || got[0].SeriesID != 1 {
		t.Fatalf("auditSeries = %+v, want only series 1", got)
	}
	if got[0].JaPrefix != "ロマンス" {
		t.Fatalf("ja prefix = %q", got[0].JaPrefix)
	}
	if len(got[0].Members) != 2 {
		t.Fatalf("members = %v", got[0].Members)
	}
}

func TestCommonPrefix(t *testing.T) {
	if got := commonPrefix([]string{"ロマンス 上", "ロマンス 下"}); got != "ロマンス" {
		t.Fatalf("commonPrefix = %q", got)
	}
	if got := commonPrefix([]string{"甲", "乙"}); got != "" {
		t.Fatalf("commonPrefix = %q, want empty", got)
	}
	if got := commonPrefix(nil); got != "" {
		t.Fatalf("commonPrefix(nil) = %q", got)
	}
}
