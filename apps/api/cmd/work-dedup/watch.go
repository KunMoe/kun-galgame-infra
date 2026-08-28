package main

import (
	"context"
	"fmt"
	"io"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

const watchSamples = 10

func runWatch(ctx context.Context, db *gorm.DB, w io.Writer) (int, error) {
	c, err := buildCensus(ctx, db)
	if err != nil {
		return 0, err
	}
	cands, err := loadWorkCandidates(ctx, db)
	if err != nil {
		return 0, err
	}
	proposed, err := loadWorkProposalPairs(ctx, db)
	if err != nil {
		return 0, err
	}
	redirects, err := loadWorkRedirects(ctx, db)
	if err != nil {
		return 0, err
	}

	// Every pair this pipeline has ever looked at, so that "merged" can be
	// counted at all: executing a merge soft-deletes the source work AND
	// deletes the pair's own candidate row (merge_execute step 8), so a
	// collapsed pair is invisible to both the detector and the queue. Only the
	// proposal it was executed through still names it.
	known := make(map[[2]int64]bool, len(c.rows)+len(cands)+len(proposed))
	for _, r := range c.rows {
		known[[2]int64{r.A, r.B}] = true
	}
	for key := range cands {
		known[key] = true
	}
	for key := range proposed {
		known[[2]int64{min(key[0], key[1]), max(key[0], key[1])}] = true
	}
	merged := 0
	for key := range known {
		if resolveWork(redirects, key[0]) == resolveWork(redirects, key[1]) {
			merged++
		}
	}

	byStatus := map[int16]int{}
	var fresh, inScope, proposedOnly int
	var samples []pairRow
	for i, r := range c.rows {
		if c.verdicts[i] == bucketOutOfScope {
			continue
		}
		inScope++
		key := [2]int64{r.A, r.B}
		status, filed := cands[key]
		switch {
		case resolveWork(redirects, r.A) == resolveWork(redirects, r.B):
		case filed:
			byStatus[status]++
		case proposed[key]:
			proposedOnly++
		default:
			fresh++
			if len(samples) < watchSamples {
				samples = append(samples, r)
			}
		}
	}

	fmt.Fprintf(w, "[watch] pairs=%d new=%d pending=%d needs_manual=%d accepted=%d rejected=%d deferred=%d proposed=%d merged=%d\n",
		inScope, fresh,
		byStatus[model.CandidateStatusPending], byStatus[model.CandidateStatusNeedsManual],
		byStatus[model.CandidateStatusAccepted], byStatus[model.CandidateStatusRejected],
		byStatus[model.CandidateStatusDeferred], proposedOnly, merged)
	for _, r := range samples {
		fmt.Fprintf(w, "  new: %d<->%d lanes=%s×%s a=%q b=%q norm=%q\n",
			r.A, r.B, r.LaneA, r.LaneB, r.NameA, r.NameB, r.SharedNorm)
	}
	return fresh, nil
}

func loadWorkCandidates(ctx context.Context, db *gorm.DB) (map[[2]int64]int16, error) {
	var rows []struct {
		AID    int64 `gorm:"column:a_id"`
		BID    int64 `gorm:"column:b_id"`
		Status int16 `gorm:"column:status"`
	}
	if err := db.WithContext(ctx).Raw(
		`SELECT a_id, b_id, status FROM catalog_match_candidate WHERE entity_type = ?`,
		model.EntityTypeWork).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[[2]int64]int16, len(rows))
	for _, r := range rows {
		out[[2]int64{r.AID, r.BID}] = r.Status
	}
	return out, nil
}

func loadWorkProposalPairs(ctx context.Context, db *gorm.DB) (map[[2]int64]bool, error) {
	var rows []struct {
		Source int64 `gorm:"column:source_entity_id"`
		Target int64 `gorm:"column:target_entity_id"`
	}
	if err := db.WithContext(ctx).Raw(
		`SELECT source_entity_id, target_entity_id FROM catalog_merge_proposal WHERE entity_type = ?`,
		model.EntityTypeWork).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[[2]int64]bool, 2*len(rows))
	for _, r := range rows {
		out[[2]int64{r.Source, r.Target}] = true
		out[[2]int64{r.Target, r.Source}] = true
	}
	return out, nil
}
