package main

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// gateClass is the deterministic three-way split every ja work title goes
// through before any model is asked anything (refs/proj/210 §2).
type gateClass string

const (
	// classKanjiOnly: Han and no kana — the glyphs already read as Chinese and
	// the work is a glyph-conversion candidate (shinjitai → simplified), not a
	// translation one.
	classKanjiOnly gateClass = "kanji_only"
	// classNeedsMT: carries kana, so a Chinese rendering has to be produced.
	classNeedsMT gateClass = "needs_mt"
	// classSkipLatin: neither Han nor kana (latin / digits / symbols). No row is
	// ever written for these — "Fate/EXTELLA" has no Chinese rendering to invent.
	classSkipLatin gateClass = "skip_latin"
)

// The katakana middle dots ・(U+30FB) / ･(U+FF65) sit inside the kana code
// blocks, so a fully translated title that reuses the source's separator was
// killed by the kana-left gate in the char-name rehearsal (fixed there in
// 3c3671a1). Normalize to the zh convention · before any gate sees the string.
var titleNormalizer = strings.NewReplacer("・", "·", "･", "·")

func cleanTitle(s string) string {
	s = strings.TrimSpace(titleNormalizer.Replace(s))
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return strings.Trim(s, " \t\"'“”「」")
}

// containsKana keeps the prolonged sound mark ー (U+30FC) inside the kana range
// on purpose: it has no Chinese reading, so a title that still carries one has
// not been rendered into Chinese.
func containsKana(s string) bool {
	for _, r := range s {
		if (r >= 0x3040 && r <= 0x30FF) || (r >= 0xFF66 && r <= 0xFF9D) {
			return true
		}
	}
	return false
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func classify(title string) gateClass {
	t := cleanTitle(title)
	switch {
	case containsKana(t):
		return classNeedsMT
	case containsHan(t):
		return classKanjiOnly
	default:
		return classSkipLatin
	}
}

// maxTitleRunes mirrors editspec's ceiling for the field — a proposal longer
// than the field accepts can never be written, so reject it here where the
// reason is legible.
const maxTitleRunes = 500

// withinLengthBand is deliberately loose. A Chinese title is usually SHORTER
// than its Japanese source (kana collapse into Han), so the band mostly exists
// to catch the two failure shapes that are not translations at all: the model
// explaining itself instead of answering, and the model emitting a fragment.
func withinLengthBand(src, out string) bool {
	s, o := utf8.RuneCountInString(src), utf8.RuneCountInString(out)
	if o == 0 || o > maxTitleRunes {
		return false
	}
	if o > 2*s+10 {
		return false
	}
	return 3*o+3 >= s
}

var sameTitleNormalizer = strings.NewReplacer("・", "·", "･", "·", "•", "·", " ", "", "　", "")

func normalizeTitle(s string) string {
	return sameTitleNormalizer.Replace(strings.TrimSpace(s))
}

func sameTitle(a, b string) bool { return normalizeTitle(a) == normalizeTitle(b) }

type titleVerdict string

const (
	verdictAccept        titleVerdict = "accept"
	verdictSkipModel     titleVerdict = "skip_model"
	verdictKanaLeft      titleVerdict = "reject_kana_left"
	verdictLength        titleVerdict = "reject_length"
	verdictEchoedSource  titleVerdict = "reject_echoed_source"
	verdictFailVerify    titleVerdict = "reject_verify"
	verdictFailRefute    titleVerdict = "reject_refute"
	verdictFailRederive  titleVerdict = "reject_rederive"
	verdictErrorUpstream titleVerdict = "error"
)

// gateOutput runs the deterministic output gates in the order that makes the
// cheapest rejection first. src is the ja title the proposal was made from.
func gateOutput(src, out string) titleVerdict {
	if out == "" || strings.EqualFold(out, skipMarker) {
		return verdictSkipModel
	}
	if containsKana(out) {
		return verdictKanaLeft
	}
	if !withinLengthBand(src, out) {
		return verdictLength
	}
	if containsKana(cleanTitle(src)) && sameTitle(src, out) {
		return verdictEchoedSource
	}
	return verdictAccept
}
