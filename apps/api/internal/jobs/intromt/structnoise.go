package intromt

import (
	"regexp"
	"strings"
)

// Work intros arrive with the store listing's furniture still attached: a spec
// table, a staff roll, a bare product URL. Wave 214 adjudicated those as
// strippable because each already has a structured home — platform/engine rows,
// the credits roster, the release links — and 2026-08-23 counted them in the ja
// corpus: 436 intros with a 対応OS/動作環境 header, 2,280 with a labelled staff
// line, 1,069 with a URL.
//
// Two things deliberately survive, both from reading the text rather than the
// label list:
//
//   - `CV：` is NOT a staff label here. It reads as one, but in practice it sits
//     inside a character block (work 481: `すみれ` / `CV:野中みかん` / `身長:158cm`
//     / then the character's actual description), and extract-char-intros reads
//     exactly those blocks.
//   - 身長 / 体重 / スリーサイズ / 血液型 and the rest of the stat sheet stay for
//     the same reason: 736 intros carry them, interleaved with the prose that
//     makes the block worth extracting.
//
// This runs inside sanitizeSource, so it is part of src_hash: a work whose ja
// text strips down to something different retranslates on its own, with no
// forced pass.
var (
	reSpecLabel = regexp.MustCompile(`^[\s・◆●■□▼▽★☆\-–—]*` +
		`(対応OS|動作環境|必要環境|推奨環境|必要動作環境|推奨動作環境|` +
		`CPU|メモリ|メモリー|VRAM|DirectX|HDD|ハードディスク|` +
		`解像度|画面サイズ|画面解像度|サウンド|グラフィック|グラフィックス|OS)` +
		`\s*[:：]`)

	reStaffLabel = regexp.MustCompile(`^[\s・◆●■□▼▽★☆\-–—]*` +
		`(シナリオ|原画|イラスト|キャラクターデザイン|キャラデザ|` +
		`音楽|楽曲制作|楽曲|主題歌|挿入歌|エンディングテーマ|オープニングテーマ|` +
		`声優|制作|企画|企画原案|プロデューサー|ディレクター|背景|塗り)` +
		`\s*[:：]`)

	reSectionHeader = regexp.MustCompile(`^[\s・◆●■□▼▽★☆\-–—【\[]*` +
		`(SPEC|Spec|spec|STAFF|Staff|staff|スタッフ|動作環境|必要動作環境|推奨動作環境|SPEC表)` +
		`[】\]\s:：]*$`)

	reDupPurchase = regexp.MustCompile(`重複(して)?(ご)?購入|二重購入`)

	reBareURL = regexp.MustCompile(`^\s*(https?://\S+|www\.\S+)\s*$`)
)

// A run stripped past this is not an intro with furniture attached, it IS the
// furniture; keeping the original beats shipping two words.
const minStrippedRunes = 60

func stripStructuredNoise(s string) string {
	lines := strings.Split(s, "\n")
	drop := make([]bool, len(lines))
	for i, ln := range lines {
		t := strings.TrimRight(ln, "\r")
		drop[i] = reSpecLabel.MatchString(t) ||
			reStaffLabel.MatchString(t) ||
			reBareURL.MatchString(t) ||
			(reDupPurchase.MatchString(t) && strings.TrimSpace(t) != "")
	}
	for i, ln := range lines {
		if drop[i] || !reSectionHeader.MatchString(strings.TrimRight(ln, "\r")) {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			drop[i] = drop[j]
			break
		}
	}

	dropped := false
	for _, d := range drop {
		if d {
			dropped = true
			break
		}
	}
	if !dropped {
		return s
	}

	out := closeUp(lines, drop)
	if len([]rune(strings.TrimSpace(out))) < minStrippedRunes {
		return s
	}
	return out
}

// closeUp joins the kept lines, swallowing only the blank runs that removal
// created. A preview over 2,521 prod intros caught the naive version doing the
// opposite: work 39255 lost 804 runes to blank-line collapsing while its content
// was untouched, which both rewrites text no rule objected to and contradicts
// the prompt's own "keep the original paragraph structure".
func closeUp(lines []string, drop []bool) string {
	out := make([]string, 0, len(lines))
	droppedSinceEmit := false
	for i, ln := range lines {
		if drop[i] {
			droppedSinceEmit = true
			continue
		}
		blank := strings.TrimSpace(ln) == ""
		if blank && droppedSinceEmit &&
			(len(out) == 0 || strings.TrimSpace(out[len(out)-1]) == "") {
			continue
		}
		out = append(out, ln)
		droppedSinceEmit = false
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n \t\r")
}
