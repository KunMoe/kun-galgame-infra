package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gorm.io/gorm"
)

// --mode export-batches: the read-only planning face for external batch lanes.
// The candidate pool, the series batching and the known-title context are the
// hard part of the MT lane to reproduce elsewhere; exporting the planned
// batches lets a batch transport (one that cannot speak chat/completions) run
// against exactly the plan --mode auto would have used, and hand the results
// back through --mode apply-csv.

type exportMember struct {
	WorkID   int64  `json:"work_id"`
	JaTitle  string `json:"ja_title"`
	SrcHash  string `json:"src_hash"`
	Decision string `json:"decision"`
	Claimed  bool   `json:"claimed"`
	Pop      int64  `json:"pop"`
}

type exportBatch struct {
	Members []exportMember `json:"members"`
	Known   []string       `json:"known,omitempty"`
}

func runExportBatches(ctx context.Context, db *gorm.DB, opts autoOpts) error {
	batches, err := planBatches(ctx, db, opts.Limit, opts.BatchSize, opts.ShardI, opts.ShardN, opts.Exclude)
	if err != nil {
		return err
	}
	claimed, err := loadClaimedWorkIDs(ctx, db)
	if err != nil {
		return err
	}
	f, err := os.Create(opts.Out)
	if err != nil {
		return err
	}
	defer f.Close()
	bw := bufio.NewWriter(f)
	enc := json.NewEncoder(bw)

	works, claimedWorks := 0, 0
	decisions := map[decision]int{}
	for _, b := range batches {
		eb := exportBatch{Known: b.Known, Members: make([]exportMember, 0, len(b.Members))}
		for _, c := range b.Members {
			dec, hash := decide(c)
			works++
			if claimed[c.WorkID] {
				claimedWorks++
			}
			decisions[dec]++
			eb.Members = append(eb.Members, exportMember{
				WorkID: c.WorkID, JaTitle: cleanTitle(c.JaTitle), SrcHash: hash,
				Decision: string(dec), Claimed: claimed[c.WorkID], Pop: c.PopScore,
			})
		}
		if err := enc.Encode(eb); err != nil {
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	fmt.Printf("\n=== backfill-work-zh-titles export-batches ===\n"+
		"batches=%d works=%d claimed=%d insert=%d retranslate=%d unchanged=%d → %s\n",
		len(batches), works, claimedWorks,
		decisions[decInsert], decisions[decRetrans], decisions[decUnchanged], opts.Out)
	return nil
}

func loadClaimedWorkIDs(ctx context.Context, db *gorm.DB) (map[int64]bool, error) {
	var ids []int64
	err := db.WithContext(ctx).Raw(
		`SELECT id FROM catalog_work WHERE site IS NOT NULL AND deleted_at IS NULL`).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("load claimed work ids: %w", err)
	}
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
