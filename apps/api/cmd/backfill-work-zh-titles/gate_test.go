package main

import "testing"

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		title string
		want  gateClass
	}{
		{"月に寄りそう乙女の作法", classNeedsMT},
		{"月姫", classKanjiOnly},
		{"Fate/EXTELLA", classSkipLatin},
		{"AIR", classSkipLatin},
		{"", classSkipLatin},
		{"12345 -~-", classSkipLatin},
		// The katakana middle dot is a SEPARATOR, not kana: a pure-Han title
		// that uses it still needs only glyph conversion (3c3671a1's lesson,
		// applied to the source side of the gate).
		{"魔法・少女", classKanjiOnly},
		{"魔法･少女", classKanjiOnly},
		// The prolonged sound mark has no Chinese reading, so a title carrying
		// one is not already Chinese even when the rest is Han.
		{"少女ー", classNeedsMT},
		{"ハーレム", classNeedsMT},
		{"ｱｲｳ", classNeedsMT},
		{"英雄伝説 空の軌跡", classNeedsMT},
	} {
		if got := classify(tc.title); got != tc.want {
			t.Errorf("classify(%q) = %s, want %s", tc.title, got, tc.want)
		}
	}
}

func TestCleanTitleNormalizesMiddleDot(t *testing.T) {
	if got := cleanTitle(" 「双叶・莉莉」 \n第二行"); got != "双叶·莉莉" {
		t.Fatalf("cleanTitle = %q", got)
	}
}

func TestGateOutput(t *testing.T) {
	const src = "月に寄りそう乙女の作法"
	for _, tc := range []struct {
		name, src, out string
		want           titleVerdict
	}{
		{"accepts a plain rendering", src, "依偎在月下的少女礼仪", verdictAccept},
		{"accepts a middle-dot separator", "ロミオ・ジュリエット", "罗密欧·朱丽叶", verdictAccept},
		{"empty is a model skip", src, "", verdictSkipModel},
		{"SKIP is a model skip", src, "SKIP", verdictSkipModel},
		{"kana left in the output", src, "月に寄りそう少女", verdictKanaLeft},
		{"prolonged mark left in the output", "ハーレム", "后宫ー", verdictKanaLeft},
		{"an explanation instead of a title", "空", "这个标题的意思是天空，通常译作天空或者苍穹之类的词", verdictLength},
		{"a fragment instead of a title", src, "礼", verdictLength},
		{"echoing a kana source back", "そら", "そら", verdictKanaLeft},
		{"echoing a han-only source is fine", "月姫", "月姫", verdictAccept},
	} {
		if got := gateOutput(tc.src, tc.out); got != tc.want {
			t.Errorf("%s: gateOutput(%q, %q) = %s, want %s", tc.name, tc.src, tc.out, got, tc.want)
		}
	}
}

// The echo gate only fires when the source has kana AND the output is
// kana-free, which the kana gate would otherwise have caught first. Latin
// sources are the case it really guards.
func TestGateOutputEchoedLatinSource(t *testing.T) {
	if got := gateOutput("そらAIR", "そらAIR"); got != verdictKanaLeft {
		t.Fatalf("kana echo = %s, want the kana gate to fire first", got)
	}
	if got := gateOutput("ソラ", "ソラ"); got != verdictKanaLeft {
		t.Fatalf("katakana echo = %s", got)
	}
}

func TestWithinLengthBand(t *testing.T) {
	long := ""
	for range 260 {
		long += "长"
	}
	if withinLengthBand("空", long+long) {
		t.Fatal("a 520-rune proposal must fail the field's own ceiling")
	}
	if !withinLengthBand("月に寄りそう乙女の作法", "依偎月下的少女作法") {
		t.Fatal("a normal rendering must pass")
	}
}
