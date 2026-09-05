package hihyou

import (
	"fmt"
	"regexp"
	"strings"
)

// sections is the only signal that survives all four generations of the
// column's formatting. It is a completeness assertion over a hand-maintained
// list, derived from a census of the whole corpus rather than guessed: styling
// tiers cover 74/180 issues and style.bold 176/180, while these names appear in
// ~160 issues each and 期163 writes 新作资讯 in plain body type with no machine
// signal at all.
//
// A name added upstream must surface in the gate's unknown-heading report, never
// be silently emitted as an item. The report yields CANDIDATES, not verdicts:
// on the full corpus it surfaces 「X月新作本周发售」 (one per month, 52 issues
// apart) and 「PC：」, which look exactly like sections — reading the bodies shows
// both are items, a monthly release roster being one story whose body is the
// list. Promoting them cost 32 issues at the gate (212 passing -> 180).
// Distinguished and not in this list means item.
var sections = map[string]bool{
	"新作资讯":   true,
	"新作情报":   true,
	"汉化情报":   true,
	"周边信息":   true,
	"周边情报":   true,
	"汉化":     true,
	"周边":     true,
	"「周边信息」": true,
}

// preambleRe matches the two masthead lines every issue opens with. They are
// bold, so without this they become items with no body.
var preambleRe = regexp.MustCompile(`^(新闻日期|本文首发于公众号)`)

// serialRe strips the in-issue ordinal from a title. 「8.《コイバナ恋愛》更新预览CG」
// numbers the item within its weekly; published on its own the 8 has no referent.
var serialRe = regexp.MustCompile(`^(\d{1,2})[.、．]\s*(\S)`)

// maxTitleRunes keeps the closing sign-off from becoming a 91-character title.
const maxTitleRunes = 60

// Item is one extracted piece of news. Pictures[0] becomes the banner and the
// rest become news_item_image rows.
type Item struct {
	Ordinal  int      // 1-based, paragraph order; the tail of external_id
	Title    string   `json:"title"`
	Section  string   `json:"section"`
	Body     []string `json:"body"`
	Pictures []string `json:"pictures"`
}

type Segmentation struct {
	IssueNo int
	CV      int64
	Mode    string // "distinguished" or "numeric"
	Items   []Item
	Orphans int
	Unknown []string // short distinguished headings outside the vocabulary
}

type mark struct {
	index int
	text  string
	head  bool
}

// Segment splits one weekly into items. It is deterministic: external_id embeds
// the ordinal, so a re-run that reorders items would rewrite identities.
func Segment(a *Article) Segmentation {
	ps := a.Data.Opus.Content.Paragraphs
	seg := Segmentation{CV: a.Data.ID}
	seg.IssueNo, _ = IssueNumber(a.Data.Title)

	base := bodyFontSize(ps)

	var marks []mark
	for i, p := range ps {
		if p.ParaType != paraText && p.ParaType != paraTextAlt {
			continue
		}
		ws := p.words()
		if len(ws) == 0 {
			continue
		}
		t := p.text()
		size, bold := 0, true
		for _, w := range ws {
			if w.size > size {
				size = w.size
			}
			bold = bold && w.bold
		}
		head := (bold || size > base) && !preambleRe.MatchString(t) && runeLen(t) <= maxTitleRunes
		if head {
			t = serialRe.ReplaceAllString(t, "$2")
		}
		marks = append(marks, mark{index: i, text: t, head: head})
	}

	starts := map[int]string{}
	secs := map[int]string{}
	for _, m := range marks {
		isSec := sections[m.text]
		if !m.head && !isSec {
			continue
		}
		if isSec {
			secs[m.index] = m.text
			continue
		}
		starts[m.index] = m.text
		if runeLen(m.text) <= 10 && !strings.Contains(m.text, "《") {
			seg.Unknown = append(seg.Unknown, m.text)
		}
	}

	seg.Mode = "distinguished"
	if len(starts) < 3 {
		// 期38–108: sections are large but items are plain body text prefixed
		// "1." — no visual signal reaches the JSON at all.
		seg.Mode = "numeric"
		starts = map[int]string{}
		for _, m := range marks {
			if serialRe.MatchString(m.text) {
				starts[m.index] = serialRe.ReplaceAllString(m.text, "$2")
			}
		}
	}

	seg.Items, seg.Orphans = assemble(ps, starts, secs)
	return seg
}

// bodyFontSize is the size carrying the most characters, not the most
// paragraphs: a heading tier can outnumber body paragraphs in a short issue.
func bodyFontSize(ps []Paragraph) int {
	weight := map[int]int{}
	for _, p := range ps {
		for _, w := range p.words() {
			weight[w.size] += runeLen(w.text)
		}
	}
	best, bestN := 17, 0
	for size, n := range weight {
		if n > bestN || (n == bestN && size < best) {
			best, bestN = size, n
		}
	}
	return best
}

func assemble(ps []Paragraph, starts, secs map[int]string) ([]Item, int) {
	var items []Item
	// An index, not a *Item: append reallocates the slice, and a pointer taken
	// before that silently collects the rest of the body into a dead array.
	cur := -1
	section := ""
	orphans := 0

	for i, p := range ps {
		if title, ok := secs[i]; ok {
			section, cur = title, -1
			continue
		}
		if title, ok := starts[i]; ok {
			items = append(items, Item{Ordinal: len(items) + 1, Title: title, Section: section})
			cur = len(items) - 1
			continue
		}
		switch p.ParaType {
		case paraPicture:
			if cur >= 0 {
				items[cur].Pictures = append(items[cur].Pictures, p.pictures()...)
			}
			continue
		case paraRule:
			continue
		}
		t := p.text()
		if t == "" {
			continue
		}
		if cur >= 0 {
			items[cur].Body = append(items[cur].Body, t)
		} else if !preambleRe.MatchString(t) {
			orphans++
		}
	}
	return items, orphans
}

// Gate decides whether an issue is publishable at all. A failing issue is
// quarantined whole rather than best-efforted into the queue: a mis-segmented
// issue produces rows that look plausible one at a time.
func Gate(seg Segmentation) []string {
	var fail []string
	if n := len(seg.Items); n < 3 || n > 60 {
		fail = append(fail, fmt.Sprintf("item count %d outside [3,60]", n))
	}
	// A heading whose block holds only an image is real upstream shape — it has
	// no preview to publish, so it is dropped per item. More than a couple in one
	// issue means the segmentation itself slipped.
	//
	// The trailing contentless run is exempt: 期256's closing credits span three
	// bold lines (文案/文案审核 rosters), which quarantined a perfectly segmented
	// issue — a contentless block at the very end cannot be a lost body, while a
	// mid-issue empty still means exactly that. Trailing items that carry
	// pictures still count: an image-only story is upstream shape, not a
	// sign-off. Census 2026-09-05 over all 220 issues: the exemption flips only
	// 期256; every other issue has a trailing run of zero.
	items := seg.Items
	for len(items) > 0 {
		last := items[len(items)-1]
		if len(last.Body) != 0 || len(last.Pictures) != 0 {
			break
		}
		items = items[:len(items)-1]
	}
	empty := 0
	for _, it := range items {
		if len(it.Body) == 0 {
			empty++
		}
	}
	if empty > 2 {
		fail = append(fail, fmt.Sprintf("%d items with no body", empty))
	}
	if seg.Orphans > 3 {
		fail = append(fail, fmt.Sprintf("%d orphan paragraphs", seg.Orphans))
	}
	long := 0
	for _, it := range seg.Items {
		if runeLen(it.Title) > maxTitleRunes {
			long++
		}
	}
	if long > 0 {
		fail = append(fail, fmt.Sprintf("%d over-long titles", long))
	}
	return fail
}

func runeLen(s string) int { return len([]rune(s)) }
