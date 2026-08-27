package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

func runCensus(ctx context.Context, db *gorm.DB, samples int) error {
	cands, err := loadMTCandidates(ctx, db, 0)
	if err != nil {
		return err
	}
	c := censusOf(cands)
	fmt.Printf("\n=== backfill-work-zh-titles census ===\n")
	fmt.Printf("works with a ja title and no source zh title: %d\n", c.Total)
	fmt.Printf("  pre-gate: kanji_only=%d needs_mt=%d skip_latin=%d\n", c.KanjiOnly, c.NeedsMT, c.SkipLatin)
	fmt.Printf("  state:    insert=%d retranslate=%d unchanged=%d\n", c.Insert, c.Retranslate, c.Unchanged)
	for _, class := range []gateClass{classKanjiOnly, classNeedsMT, classSkipLatin} {
		shown := 0
		for _, cand := range cands {
			if cand.Class != class || shown >= samples {
				continue
			}
			fmt.Printf("  %-10s work=%d %q\n", class, cand.WorkID, cleanTitle(cand.JaTitle))
			shown++
		}
	}
	return nil
}

// converter renders a shinjitai / traditional Han title as simplified Chinese.
// The kanji lane's real converter is opencc, which is an external binary and
// deliberately optional: [[opencc-roundtrip-is-not-identity]] means its output
// is a PROPOSAL for the dictionary queue, never a verdict, so a run without
// opencc installed still produces the worklist.
type converter interface {
	Convert(ctx context.Context, s string) (string, error)
}

type openccConverter struct {
	bin    string
	config string
}

func (o openccConverter) Convert(ctx context.Context, s string) (string, error) {
	cmd := exec.CommandContext(ctx, o.bin, "-c", o.config)
	cmd.Stdin = strings.NewReader(s)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("opencc %q: %w", s, err)
	}
	return strings.TrimSpace(string(out)), nil
}

var kanjiCSVHeader = []string{"work_id", "ja_title", "src_hash", "pop", "converted_zh"}

// runKanji emits the glyph-conversion worklist. It never writes the database:
// a converted title still has to clear the human dictionary queue, because the
// opencc round trip is not the identity (郎 → 郞 produced 142 false positives
// on the character lane).
func runKanji(ctx context.Context, db *gorm.DB, conv converter, out string, limit, samples int) error {
	cands, err := loadMTCandidates(ctx, db, 0)
	if err != nil {
		return err
	}
	todo := filterClass(cands, classKanjiOnly)
	if limit > 0 && limit < len(todo) {
		todo = todo[:limit]
	}
	fmt.Printf("\n=== backfill-work-zh-titles kanji worklist (%d works) ===\n", len(todo))
	if out == "" {
		for i, c := range todo {
			if i >= samples {
				break
			}
			fmt.Printf("  work=%d %q\n", c.WorkID, cleanTitle(c.JaTitle))
		}
		fmt.Println("\n[no --out] worklist not written; this mode never writes the database")
		return nil
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(kanjiCSVHeader); err != nil {
		return err
	}
	converted := 0
	for _, c := range todo {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		ja := cleanTitle(c.JaTitle)
		_, hash := decide(c)
		zh := ""
		if conv != nil {
			zh, err = conv.Convert(ctx, ja)
			if err != nil {
				return err
			}
			if zh != "" && zh != ja {
				converted++
			}
		}
		if err := w.Write([]string{
			strconv.FormatInt(c.WorkID, 10), ja, hash,
			strconv.FormatInt(c.PopScore, 10), zh,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}
	fmt.Printf("worklist=%d converted_differs=%d → %s\n", len(todo), converted, out)
	return nil
}
