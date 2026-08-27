package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"strconv"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

type machineWrite int

const (
	writeSkippedSource machineWrite = iota
	writeUnchanged
	writeInserted
	writeReplaced
)

// writeMachineTitle installs one machine zh-Hans title for a work.
//
// Two invariants it must not break: a source row always wins (a title a
// publisher shipped is never replaced by a translation), and a machine row is
// only ever replaced by this lane's own row. The unique key carries the TITLE,
// so a re-translation is a delete + insert, not an upsert — a different string
// is a different row.
func writeMachineTitle(ctx context.Context, tx *gorm.DB, workID int64, zh, hash, mtModel string) (machineWrite, error) {
	var srcRows int64
	if err := tx.WithContext(ctx).Raw(
		`SELECT count(*) FROM catalog_work_title WHERE work_id = ? AND lang LIKE 'zh%' AND provenance = ?`,
		workID, model.WorkTitleProvenanceSource).Scan(&srcRows).Error; err != nil {
		return writeSkippedSource, err
	}
	if srcRows > 0 {
		return writeSkippedSource, nil
	}
	var cur []struct {
		ID      int64   `gorm:"column:id"`
		Title   string  `gorm:"column:title"`
		SrcHash *string `gorm:"column:src_hash"`
	}
	if err := tx.WithContext(ctx).Raw(
		`SELECT id, title, src_hash FROM catalog_work_title
		 WHERE work_id = ? AND lang LIKE 'zh%' AND provenance = ? ORDER BY kind, id`,
		workID, model.WorkTitleProvenanceMachine).Scan(&cur).Error; err != nil {
		return writeSkippedSource, err
	}
	if len(cur) == 1 && cur[0].Title == zh && cur[0].SrcHash != nil && *cur[0].SrcHash == hash {
		return writeUnchanged, nil
	}
	if len(cur) > 0 {
		if err := tx.WithContext(ctx).Exec(
			`DELETE FROM catalog_work_title WHERE work_id = ? AND lang LIKE 'zh%' AND provenance = ?`,
			workID, model.WorkTitleProvenanceMachine).Error; err != nil {
			return writeSkippedSource, err
		}
	}
	res := tx.WithContext(ctx).Exec(`
		INSERT INTO catalog_work_title (work_id, lang, title, kind, provenance, src_hash, mt_model)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (work_id, lang, title, kind) DO NOTHING`,
		workID, langZhHans, zh, model.WorkTitleKindOfficial, model.WorkTitleProvenanceMachine, hash, mtModel)
	if res.Error != nil {
		return writeSkippedSource, res.Error
	}
	if len(cur) > 0 {
		return writeReplaced, nil
	}
	return writeInserted, nil
}

type applyRow struct {
	WorkID  int64
	ZhTitle string
	SrcHash string
	Model   string
}

func readAcceptedCSV(path string) ([]applyRow, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	recs, err := r.ReadAll()
	if err != nil {
		return nil, 0, err
	}
	if len(recs) == 0 {
		return nil, 0, fmt.Errorf("%s: empty", path)
	}
	idx := map[string]int{}
	for i, h := range recs[0] {
		idx[h] = i
	}
	for _, want := range []string{"work_id", "src_hash", "proposed_zh", "model"} {
		if _, ok := idx[want]; !ok {
			return nil, 0, fmt.Errorf("%s: missing column %q", path, want)
		}
	}
	jaCol, hasJa := idx["ja_title"]
	out := make([]applyRow, 0, len(recs)-1)
	gateRefused := 0
	for i, rec := range recs[1:] {
		zh := cleanTitle(rec[idx["proposed_zh"]])
		if zh == "" {
			continue
		}
		// An external lane's proposals do not pass through gateOutput on their
		// way here, so re-assert the deterministic gates at the door whenever
		// the CSV carries the source title to gate against.
		if hasJa {
			if v := gateOutput(cleanTitle(rec[jaCol]), zh); v != verdictAccept {
				gateRefused++
				continue
			}
		}
		id, err := strconv.ParseInt(rec[idx["work_id"]], 10, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("%s line %d: bad work_id: %w", path, i+2, err)
		}
		out = append(out, applyRow{
			WorkID: id, ZhTitle: zh,
			SrcHash: rec[idx["src_hash"]], Model: rec[idx["model"]],
		})
	}
	return out, gateRefused, nil
}

func runApplyCSV(ctx context.Context, db *gorm.DB, path string, apply bool, limit, samples int) error {
	rows, gateRefused, err := readAcceptedCSV(path)
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	fmt.Printf("\n=== backfill-work-zh-titles apply-csv %s (%s) ===\naccepted_rows=%d refused_gate=%d\n",
		path, modeLabel(apply), len(rows), gateRefused)
	for i, r := range rows {
		if i >= samples {
			break
		}
		fmt.Printf("  sample: work=%d %q\n", r.WorkID, r.ZhTitle)
	}
	if !apply {
		fmt.Println("\n[dry run] nothing written — re-run with --apply")
		return nil
	}
	var inserted, replaced, unchanged, refused int
	touched := make([]int64, 0, len(rows))
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			res, err := writeMachineTitle(ctx, tx, r.WorkID, r.ZhTitle, r.SrcHash, r.Model)
			if err != nil {
				return err
			}
			switch res {
			case writeInserted:
				inserted++
				touched = append(touched, r.WorkID)
			case writeReplaced:
				replaced++
				touched = append(touched, r.WorkID)
			case writeUnchanged:
				unchanged++
			case writeSkippedSource:
				refused++
			}
		}
		slices.Sort(touched)
		return repository.TouchWorks(ctx, tx, touched)
	})
	if err != nil {
		return fmt.Errorf("apply machine titles: %w", err)
	}
	fmt.Printf("\ninserted=%d replaced=%d unchanged=%d refused_source_wins=%d touched_works=%d\n",
		inserted, replaced, unchanged, refused, len(touched))
	return nil
}
