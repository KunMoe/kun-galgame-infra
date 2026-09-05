package hihyou

import (
	"strings"
	"testing"
)

// The corpus lives outside the module (refs/ is gitignored), so these fixtures
// reproduce the shapes the census found rather than loading real issues.

func text(size int, bold bool, words ...string) Paragraph {
	p := Paragraph{ParaType: paraText}
	for _, w := range words {
		var n struct {
			Word struct {
				Words    string `json:"words"`
				FontSize int    `json:"font_size"`
				Style    struct {
					Bold bool `json:"bold"`
				} `json:"style"`
			} `json:"word"`
		}
		n.Word.Words = w
		n.Word.FontSize = size
		n.Word.Style.Bold = bold
		p.Text.Nodes = append(p.Text.Nodes, n)
	}
	return p
}

func picture(url string) Paragraph {
	p := Paragraph{ParaType: paraPicture}
	p.Pic.Pics = append(p.Pic.Pics, struct {
		URL string `json:"url"`
	}{URL: url})
	return p
}

func rule() Paragraph {
	return Paragraph{ParaType: paraRule}
}

func article(title string, ps ...Paragraph) *Article {
	a := &Article{Code: 0}
	a.Data.ID = 12345
	a.Data.Title = title
	a.Data.Opus.Content.Paragraphs = ps
	return a
}

const body = "正文正文正文正文正文正文正文正文正文正文正文正文正文正文正文正文正文正文正文正文"

func TestIssueNumber(t *testing.T) {
	cases := map[string]int{
		"「Gal周报 74期」《近月》中文版特典开始众筹":    74,
		"【Gal周报255期】《My Merry May》登陆": 255,
		// 期147 is the only one of 217 without the 期 character. A regex that
		// requires it drops the whole issue and reports nothing.
		"【Gal周报147】本周新作": 147,
		"聊聊这两年的galgame":  0,
	}
	for title, want := range cases {
		got, ok := IssueNumber(title)
		if want == 0 {
			if ok {
				t.Errorf("%q: expected not a weekly, got %d", title, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("%q: got %d (%v), want %d", title, got, ok, want)
		}
	}
}

// 期163 writes its section names in plain body type with no bold and no size
// bump. Vocabulary is the only thing that recognises them; a styling-first rule
// loses the boundary and orphans everything before the first item.
func TestSectionRecognisedWithoutStyling(t *testing.T) {
	seg := Segment(article("【Gal周报163期】x",
		text(17, false, "新作资讯"),
		text(17, true, "《A》情报公开"), text(17, false, body),
		text(17, true, "《B》发售日决定"), text(17, false, body),
		text(17, false, "汉化情报"),
		text(17, true, "《C》汉化发布"), text(17, false, body),
	))
	if len(seg.Items) != 3 {
		t.Fatalf("items = %d, want 3: %+v", len(seg.Items), seg.Items)
	}
	if seg.Items[0].Section != "新作资讯" || seg.Items[2].Section != "汉化情报" {
		t.Errorf("sections = %q / %q", seg.Items[0].Section, seg.Items[2].Section)
	}
	if seg.Orphans != 0 {
		t.Errorf("orphans = %d, want 0", seg.Orphans)
	}
}

// 期126 splits one title across three nodes: 《 + Clover Memory&#39;s + 》情报公开.
// Reading node by node yields an item whose title is 《.
func TestTitleJoinsNodesAndUnescapes(t *testing.T) {
	seg := Segment(article("【Gal周报126期】x",
		text(17, false, "新作资讯"),
		text(17, true, "《", "Clover Memory&#39;s", "》情报公开"), text(17, false, body),
		text(17, true, "《B》发售"), text(17, false, body),
		text(17, true, "《C》发售"), text(17, false, body),
	))
	if got := seg.Items[0].Title; got != "《Clover Memory's》情报公开" {
		t.Errorf("title = %q", got)
	}
}

// The serial numbers the item inside its weekly; published on its own the 8 has
// no referent. It was first noticed in the numeric fallback and must be stripped
// in the distinguished mode too.
func TestSerialPrefixStrippedInBothModes(t *testing.T) {
	distinguished := Segment(article("【Gal周报200期】x",
		text(17, false, "新作资讯"),
		text(17, true, "8.《コイバナ恋愛》更新预览CG"), text(17, false, body),
		text(17, true, "9.《B》发售"), text(17, false, body),
		text(17, true, "10.《C》发售"), text(17, false, body),
	))
	if got := distinguished.Items[0].Title; got != "《コイバナ恋愛》更新预览CG" {
		t.Errorf("distinguished title = %q", got)
	}

	// 期38–108: no visual signal reaches the JSON at all, only the ordinal.
	numeric := Segment(article("「Gal周报 38期」x",
		text(24, false, "新作资讯"),
		text(17, false, "1.《A》众筹开始"), text(17, false, body),
		text(17, false, "2.《B》汉化发布"), text(17, false, body),
		text(17, false, "3.《C》发售"), text(17, false, body),
	))
	if numeric.Mode != "numeric" {
		t.Fatalf("mode = %q, want numeric", numeric.Mode)
	}
	if got := numeric.Items[0].Title; got != "《A》众筹开始" {
		t.Errorf("numeric title = %q", got)
	}
}

// para_type 3 is a decorative horizontal rule — seven distinct URLs across 1,106
// occurrences in the corpus. An earlier prototype collected THESE as item
// pictures and skipped para_type 2, reporting 842 images that were all separator
// rules while dropping the 8,275 real ones.
func TestPicturesComeFromParaType2Only(t *testing.T) {
	seg := Segment(article("【Gal周报255期】x",
		text(17, false, "新作资讯"),
		text(17, true, "《A》情报公开"), text(17, false, body),
		picture("http://i0.hdslb.com/bfs/new_dyn/aaa.png"),
		rule(),
		picture("//i0.hdslb.com/bfs/article/bbb.png"),
		text(17, true, "《B》发售"), text(17, false, body),
		text(17, true, "《C》发售"), text(17, false, body),
	))
	want := []string{"https://i0.hdslb.com/bfs/new_dyn/aaa.png", "https://i0.hdslb.com/bfs/article/bbb.png"}
	got := seg.Items[0].Pictures
	if len(got) != len(want) {
		t.Fatalf("pictures = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("picture[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(seg.Items[1].Pictures) != 0 {
		t.Errorf("pictures leaked into the next item: %v", seg.Items[1].Pictures)
	}
}

// The masthead lines are bold. Without the preamble exclusion they become items
// with no body, and the whole issue shifts by two.
func TestPreambleIsNeitherItemNorOrphan(t *testing.T) {
	seg := Segment(article("【Gal周报200期】x",
		text(17, true, "新闻日期：2026年8月1日~2026年8月8日"),
		text(17, true, "本文首发于公众号 Galgame批评"),
		text(17, false, "新作资讯"),
		text(17, true, "《A》情报公开"), text(17, false, body),
		text(17, true, "《B》发售"), text(17, false, body),
		text(17, true, "《C》发售"), text(17, false, body),
	))
	if len(seg.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(seg.Items))
	}
	if seg.Orphans != 0 {
		t.Errorf("orphans = %d, want 0", seg.Orphans)
	}
}

func TestGateQuarantinesRatherThanBestEfforts(t *testing.T) {
	cases := []struct {
		name string
		seg  Segmentation
		want string
	}{
		{"too few items", Segmentation{Items: []Item{{Title: "a", Body: []string{"x"}}}}, "item count"},
		{"orphan blowup", Segmentation{Items: threeItems(), Orphans: 10}, "orphan paragraphs"},
		{"bodyless run mid-issue", Segmentation{Items: append([]Item{
			{Title: "d"}, {Title: "e"}, {Title: "f"}}, threeItems()...)}, "items with no body"},
		{"image-only trailing run", Segmentation{Items: append(threeItems(),
			Item{Title: "d", Pictures: []string{"p"}},
			Item{Title: "e", Pictures: []string{"p"}},
			Item{Title: "f", Pictures: []string{"p"}})}, "items with no body"},
		{"sign-off as title", Segmentation{Items: append(threeItems(),
			Item{Title: strings.Repeat("长", 91), Body: []string{"x"}})}, "over-long titles"},
	}
	for _, c := range cases {
		fail := Gate(c.seg)
		if len(fail) == 0 || !strings.Contains(strings.Join(fail, ";"), c.want) {
			t.Errorf("%s: failures = %v, want one mentioning %q", c.name, fail, c.want)
		}
	}
	if fail := Gate(Segmentation{Items: threeItems()}); len(fail) != 0 {
		t.Errorf("a healthy issue was rejected: %v", fail)
	}
	signoff := append(threeItems(),
		Item{Title: "文案：Zinogre、剩斗士星星、kksk、"},
		Item{Title: "Casillas、倉小唯、poi233、萧端凛、式轩"},
		Item{Title: "文案审核：信光、远野、十二级重巡拿拿酱、尘归"})
	if fail := Gate(Segmentation{Items: signoff}); len(fail) != 0 {
		t.Errorf("期256-shaped trailing sign-off was rejected: %v", fail)
	}
}

func threeItems() []Item {
	return []Item{
		{Title: "a", Body: []string{"x"}},
		{Title: "b", Body: []string{"x"}},
		{Title: "c", Body: []string{"x"}},
	}
}

func TestPreviewTruncatesInRunes(t *testing.T) {
	long := strings.Repeat("字", 300)
	got, cut := preview([]string{long})
	if !cut {
		t.Fatal("expected truncation")
	}
	if n := len([]rune(got)); n != 200 {
		t.Errorf("preview = %d runes, want 200", n)
	}
	if _, cut := preview([]string{"短"}); cut {
		t.Error("short body reported as truncated")
	}
}

func TestExternalIDIsStable(t *testing.T) {
	if got := ExternalID(52194446, 7); got != "cv52194446#7" {
		t.Errorf("external id = %q", got)
	}
}
