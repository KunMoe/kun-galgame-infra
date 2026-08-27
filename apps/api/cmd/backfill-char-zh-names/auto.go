package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

// --auto: the machine-review lane (user adjudication 08-15: the per-batch human
// CSV review is replaced by a machine gate). Every proposal must clear the
// deterministic pre-gates and then three DIFFERENT question framings
// unanimously; anything else keeps its audit trail in the CSV with an empty
// proposed_zh, so --apply-csv skips it and the character stays in the residue
// pool for a future pass.

const verifySystemPrompt = `你是 galgame(视觉小说)领域的译名审校。判断给出的简体中文译名对于该角色名是否正确、自然、符合通行译法。
只输出一行:「通过」或「不通过」。拿不准就输出「不通过」。`

const refuteSystemPrompt = `你是挑剔的译名质检员。找出下面这个简体中文译名的问题:错译、自创生僻写法、残留假名、音译不通行、与官方或社区通行译法冲突等。
若确实挑不出问题,只输出「无误」;否则只输出一行最主要的问题。`

// rederiveSystemPrompt must NOT be the translate prompt verbatim: at
// temperature 0 an identical request would trivially re-emit the identical
// answer and this vote would always pass. Different phrasing + temperature 0.3
// turn agreement into a stability probe instead.
const rederiveSystemPrompt = `给出这个 galgame 角色名最通行的简体中文写法。只输出写法本身,不要解释。
西式名用通行音译,和风假名人名用最常见汉字写法。没有把握就只输出 SKIP。`

type autoVerdict string

const (
	verdictAccept       autoVerdict = "accept"
	verdictSkipModel    autoVerdict = "skip_model"
	verdictKanaLeft     autoVerdict = "reject_kana_left"
	verdictTooLong      autoVerdict = "reject_too_long"
	verdictFailVerify   autoVerdict = "reject_verify"
	verdictFailRefute   autoVerdict = "reject_refute"
	verdictFailRederive autoVerdict = "reject_rederive"
	verdictError        autoVerdict = "error"
)

var autoCSVHeader = []string{"character_id", "name", "latin", "works", "uses",
	"proposed_zh", "model", "verdict", "candidate_zh", "rederived_zh", "detail"}

type autoStats struct {
	Accepted, Skips, Kana, TooLong, Verify, Refute, Rederive, Errors int
}

type autoTranslator interface {
	translator
	chat(ctx context.Context, system, user string, temperature float64) (string, error)
}

func runAuto(ctx context.Context, db *gorm.DB, tr autoTranslator, modelName, out string,
	limit, shardI, shardN int, exclude map[int64]bool, delay time.Duration) error {
	cands, err := loadMTResidue(ctx, db, 0)
	if err != nil {
		return err
	}
	todo := cands[:0]
	for _, c := range cands {
		if shardN > 0 && c.ID%int64(shardN) != int64(shardI) {
			continue
		}
		if exclude[c.ID] {
			continue
		}
		todo = append(todo, c)
		if limit > 0 && len(todo) >= limit {
			break
		}
	}
	fmt.Printf("\n=== backfill-char-zh-names AUTO (this run: %d, shard %d/%d, excluded %d) ===\n",
		len(todo), shardI, shardN, len(exclude))

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	w := csv.NewWriter(bw)
	if err := w.Write(autoCSVHeader); err != nil {
		return err
	}
	st := autoStats{}
	for i, c := range todo {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if i > 0 && delay > 0 {
			time.Sleep(delay)
		}
		rec := judgeOne(ctx, tr, modelName, c, delay, &st)
		if err := w.Write(rec); err != nil {
			return err
		}
		w.Flush()
		if err := bw.Flush(); err != nil {
			return err
		}
		if (i+1)%25 == 0 {
			fmt.Fprintf(os.Stderr, "progress %d/%d accepted=%d skips=%d pregate=%d verify=%d refute=%d rederive=%d errors=%d\n",
				i+1, len(todo), st.Accepted, st.Skips, st.Kana+st.TooLong, st.Verify, st.Refute, st.Rederive, st.Errors)
		}
	}
	fmt.Printf("\naccepted=%d skipped_by_model=%d kana_left=%d too_long=%d fail_verify=%d fail_refute=%d fail_rederive=%d errors=%d → %s\n",
		st.Accepted, st.Skips, st.Kana, st.TooLong, st.Verify, st.Refute, st.Rederive, st.Errors, out)
	return nil
}

func judgeOne(ctx context.Context, tr autoTranslator, modelName string, c mtResidueCandidate, delay time.Duration, st *autoStats) []string {
	row := func(v autoVerdict, proposed, candidate, rederived, detail string) []string {
		return []string{strconv.FormatInt(c.ID, 10), c.DisplayName, c.Latin,
			truncateRunes(c.Works, 120), strconv.FormatInt(c.Uses, 10),
			proposed, modelName, string(v), candidate, rederived, detail}
	}
	zh, _, err := tr.Translate(ctx, c)
	if err != nil {
		st.Errors++
		return row(verdictError, "", "", "", truncateRunes(err.Error(), 160))
	}
	if zh == "" || strings.EqualFold(zh, skipMarker) {
		st.Skips++
		return row(verdictSkipModel, "", "", "", "")
	}
	if containsKana(zh) {
		st.Kana++
		return row(verdictKanaLeft, "", zh, "", "")
	}
	if utf8.RuneCountInString(zh) > 20 {
		st.TooLong++
		return row(verdictTooLong, "", zh, "", "")
	}
	subject := userMessage(c) + "\n待审译名: " + zh

	pause := func() {
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	pause()
	ans, err := tr.chat(ctx, verifySystemPrompt, subject, 0)
	if err != nil {
		st.Errors++
		return row(verdictError, "", zh, "", truncateRunes(err.Error(), 160))
	}
	if firstLine(ans) != "通过" {
		st.Verify++
		return row(verdictFailVerify, "", zh, "", truncateRunes(ans, 120))
	}
	pause()
	ans, err = tr.chat(ctx, refuteSystemPrompt, subject, 0)
	if err != nil {
		st.Errors++
		return row(verdictError, "", zh, "", truncateRunes(err.Error(), 160))
	}
	if !strings.HasPrefix(firstLine(ans), "无误") {
		st.Refute++
		return row(verdictFailRefute, "", zh, "", truncateRunes(ans, 120))
	}
	pause()
	ans, err = tr.chat(ctx, rederiveSystemPrompt, userMessage(c), 0.3)
	if err != nil {
		st.Errors++
		return row(verdictError, "", zh, "", truncateRunes(err.Error(), 160))
	}
	rederived := cleanProposal(ans)
	if !sameName(rederived, zh) {
		st.Rederive++
		return row(verdictFailRederive, "", zh, rederived, "")
	}
	st.Accepted++
	return row(verdictAccept, zh, zh, rederived, "")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(strings.TrimSpace(s), "「」。.\"'")
}

// sameName compares two zh renderings up to separator noise: middle-dot
// variants and spaces never distinguish two spellings of one name.
func sameName(a, b string) bool {
	return normalizeName(a) == normalizeName(b)
}

var nameNormalizer = strings.NewReplacer("・", "·", "･", "·", "•", "·", " ", "", "　", "")

func normalizeName(s string) string {
	return nameNormalizer.Replace(strings.TrimSpace(s))
}

func containsKana(s string) bool {
	for _, r := range s {
		if (r >= 0x3040 && r <= 0x30FF) || (r >= 0xFF66 && r <= 0xFF9D) {
			return true
		}
	}
	return false
}

func loadExcludeIDs(path string) (map[int64]bool, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := make(map[int64]bool)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		id, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: bad id %q", path, line)
		}
		out[id] = true
	}
	return out, sc.Err()
}

func parseShard(s string) (int, int, error) {
	if s == "" {
		return 0, 0, nil
	}
	var i, n int
	if _, err := fmt.Sscanf(s, "%d/%d", &i, &n); err != nil || n < 1 || i < 0 || i >= n {
		return 0, 0, fmt.Errorf("bad --shard %q (want i/n with 0 <= i < n)", s)
	}
	return i, n, nil
}
