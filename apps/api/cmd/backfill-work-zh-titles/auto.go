package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// --auto: the machine-review lane. A proposal must clear the deterministic
// output gates and then three DIFFERENT question framings unanimously;
// anything else keeps its audit trail in the CSV with an empty proposed_zh, so
// the apply step skips it and the work stays in the pool for a later pass.

var autoCSVHeader = []string{"work_id", "ja_title", "src_hash", "decision", "pop",
	"proposed_zh", "model", "verdict", "candidate_zh", "rederived_zh", "detail"}

type autoStats struct {
	Accepted, Skips, Kana, Length, Echoed, Verify, Refute, Rederive, Errors, Unchanged int
}

func (s autoStats) line() string {
	return fmt.Sprintf("accepted=%d skipped_by_model=%d kana_left=%d length=%d echoed=%d "+
		"fail_verify=%d fail_refute=%d fail_rederive=%d errors=%d unchanged=%d",
		s.Accepted, s.Skips, s.Kana, s.Length, s.Echoed,
		s.Verify, s.Refute, s.Rederive, s.Errors, s.Unchanged)
}

type autoOpts struct {
	Out       string
	Limit     int
	BatchSize int
	ShardI    int
	ShardN    int
	Exclude   map[int64]bool
	Delay     time.Duration
	Model     string
}

func runAuto(ctx context.Context, db *gorm.DB, tr translator, opts autoOpts) error {
	batches, err := planBatches(ctx, db, opts.Limit, opts.BatchSize, opts.ShardI, opts.ShardN, opts.Exclude)
	if err != nil {
		return err
	}
	total := 0
	for _, b := range batches {
		total += len(b.Members)
	}
	fmt.Printf("\n=== backfill-work-zh-titles AUTO (works %d in %d series batches, shard %d/%d, excluded %d) ===\n",
		total, len(batches), opts.ShardI, opts.ShardN, len(opts.Exclude))

	f, err := os.Create(opts.Out)
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
	done := 0
	for _, b := range batches {
		// Titles accepted earlier in this batch join the context of the ones
		// after them: within a series the lane must agree with itself, not only
		// with what the catalog already had.
		known := append([]string(nil), b.Known...)
		for _, c := range b.Members {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if done > 0 && opts.Delay > 0 {
				time.Sleep(opts.Delay)
			}
			rec, accepted := judgeOne(ctx, tr, opts.Model, c, known, batchmatesOf(b, c.WorkID), opts.Delay, &st)
			if accepted != "" {
				known = append(known, cleanTitle(c.JaTitle)+" → "+accepted)
			}
			if err := w.Write(rec); err != nil {
				return err
			}
			w.Flush()
			if err := bw.Flush(); err != nil {
				return err
			}
			done++
			if done%25 == 0 {
				fmt.Fprintf(os.Stderr, "progress %d/%d %s\n", done, total, st.line())
			}
		}
	}
	fmt.Printf("\n%s → %s\n", st.line(), opts.Out)
	return nil
}

func judgeOne(ctx context.Context, tr translator, modelName string, c mtCandidate,
	known, mates []string, delay time.Duration, st *autoStats) ([]string, string) {
	dec, hash := decide(c)
	row := func(v titleVerdict, proposed, candidate, rederived, detail string) []string {
		return []string{strconv.FormatInt(c.WorkID, 10), cleanTitle(c.JaTitle), hash, string(dec),
			strconv.FormatInt(c.PopScore, 10), proposed, modelName, string(v), candidate, rederived, detail}
	}
	if dec == decUnchanged {
		st.Unchanged++
		return row("unchanged", "", "", "", ""), ""
	}
	zh, _, err := tr.Translate(ctx, c, known, mates)
	if err != nil {
		st.Errors++
		return row(verdictErrorUpstream, "", "", "", truncateRunes(err.Error(), 160)), ""
	}
	switch v := gateOutput(cleanTitle(c.JaTitle), zh); v {
	case verdictAccept:
	case verdictSkipModel:
		st.Skips++
		return row(v, "", "", "", ""), ""
	case verdictKanaLeft:
		st.Kana++
		return row(v, "", zh, "", ""), ""
	case verdictLength:
		st.Length++
		return row(v, "", zh, "", ""), ""
	default:
		st.Echoed++
		return row(v, "", zh, "", ""), ""
	}

	subject := userMessage(c, known, mates) + "\n待审译名: " + zh
	pause := func() {
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	pause()
	ans, err := tr.Chat(ctx, verifySystemPrompt, subject, 0)
	if err != nil {
		st.Errors++
		return row(verdictErrorUpstream, "", zh, "", truncateRunes(err.Error(), 160)), ""
	}
	if firstLine(ans) != "通过" {
		st.Verify++
		return row(verdictFailVerify, "", zh, "", truncateRunes(ans, 120)), ""
	}
	pause()
	ans, err = tr.Chat(ctx, refuteSystemPrompt, subject, 0)
	if err != nil {
		st.Errors++
		return row(verdictErrorUpstream, "", zh, "", truncateRunes(err.Error(), 160)), ""
	}
	if firstLine(ans) != "无误" {
		st.Refute++
		return row(verdictFailRefute, "", zh, "", truncateRunes(ans, 120)), ""
	}
	pause()
	ans, err = tr.Chat(ctx, rederiveSystemPrompt, userMessage(c, known, mates), 0.3)
	if err != nil {
		st.Errors++
		return row(verdictErrorUpstream, "", zh, "", truncateRunes(err.Error(), 160)), ""
	}
	rederived := cleanTitle(ans)
	if !sameTitle(rederived, zh) {
		st.Rederive++
		return row(verdictFailRederive, "", zh, rederived, ""), ""
	}
	st.Accepted++
	return row(verdictAccept, zh, zh, rederived, ""), zh
}

// planBatches is the shared front half of --auto and --census: load, split by
// the deterministic pre-gate, keep only the kana lane, then batch by series.
func planBatches(ctx context.Context, db *gorm.DB, limit, batchSize, shardI, shardN int,
	exclude map[int64]bool) ([]mtBatch, error) {
	cands, err := loadMTCandidates(ctx, db, 0)
	if err != nil {
		return nil, err
	}
	todo := make([]mtCandidate, 0, len(cands))
	for _, c := range filterClass(cands, classNeedsMT) {
		if shardN > 0 && c.WorkID%int64(shardN) != int64(shardI) {
			continue
		}
		if exclude[c.WorkID] {
			continue
		}
		todo = append(todo, c)
		if limit > 0 && len(todo) >= limit {
			break
		}
	}
	ids := make([]int64, len(todo))
	for i, c := range todo {
		ids[i] = c.WorkID
	}
	edges, err := loadSeriesEdges(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	known, err := loadKnownZhTitles(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	return buildBatches(todo, edges, known, batchSize), nil
}
