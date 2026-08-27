package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

// The three kungal forum accounts behind refs/proj/210-artifacts imported their
// catalogue with machine-translated titles, which entered catalog_work_title as
// ordinary rows and are indistinguishable from publisher titles by inspection.
// The user's 2026-08-19 ruling: treat the whole set as machine — EXCEPT a title
// that a real source publishes verbatim, which is evidence the row is not a
// translation at all and keeps provenance=0.

const forumSite = "kungal"

type reclassifyRow struct {
	WorkID        int64  `gorm:"column:work_id"`
	ProductWorkID int64  `gorm:"column:product_work_id"`
	TitleID       int64  `gorm:"column:title_id"`
	Lang          string `gorm:"column:lang"`
	Title         string `gorm:"column:title"`
	Provenance    int16  `gorm:"column:provenance"`
	SourceBacked  bool   `gorm:"column:source_backed"`
}

func readForumIDs(path string) ([]int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		id, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s: bad forum id %q", path, line)
		}
		out = append(out, id)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

// loadReclassifyRows lists every Chinese title row on the claimed works, with
// source_backed answering the one question that decides each row: does bangumi
// or vndb publish this exact string for this work?
func loadReclassifyRows(ctx context.Context, db *gorm.DB, forumIDs []int64) ([]reclassifyRow, error) {
	if len(forumIDs) == 0 {
		return nil, nil
	}
	q := `
	WITH claimed AS (
		SELECT w.id AS work_id, w.product_work_id
		FROM catalog_work w
		WHERE w.deleted_at IS NULL AND w.site = ? AND w.product_work_id IN ?
	),
	bgm_supply AS (
		SELECT c.work_id, s.name_cn AS title
		FROM claimed c
		JOIN catalog_external_ref r
		  ON r.entity_type = ? AND r.entity_id = c.work_id
		 AND r.source_id = (SELECT id FROM catalog_source WHERE key = 'bangumi')
		 AND r.link_kind = ? AND r.dead_at IS NULL
		JOIN src_bangumi.subject s ON s.id = r.external_id::bigint
		WHERE s.name_cn <> ''
	),
	vndb_supply AS (
		SELECT c.work_id, rt.title
		FROM claimed c
		JOIN catalog_external_ref r
		  ON r.entity_type = ? AND r.entity_id = c.work_id
		 AND r.source_id = (SELECT id FROM catalog_source WHERE key = 'vndb')
		 AND r.link_kind = ? AND r.dead_at IS NULL
		JOIN src_vndb.releases_vn rv ON rv.vid = r.external_id
		JOIN src_vndb.releases_titles rt ON rt.id = rv.id
		WHERE rt.lang IN (?, ?) AND rt.mtl = false AND rt.title <> ''
	),
	supply AS (
		SELECT work_id, title FROM bgm_supply
		UNION
		SELECT work_id, title FROM vndb_supply
	)
	SELECT c.work_id, c.product_work_id, t.id AS title_id, t.lang, t.title, t.provenance,
	       EXISTS (SELECT 1 FROM supply s WHERE s.work_id = c.work_id AND s.title = t.title) AS source_backed
	FROM claimed c
	JOIN catalog_work_title t ON t.work_id = c.work_id AND t.lang LIKE 'zh%'
	ORDER BY c.product_work_id, t.id`
	var rows []reclassifyRow
	err := db.WithContext(ctx).Raw(q,
		forumSite, forumIDs,
		model.EntityTypeWork, model.LinkKindExact,
		model.EntityTypeWork, model.LinkKindExact,
		langZhHans, langZhHant).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load forum reclassify rows: %w", err)
	}
	return rows, nil
}

func runReclassify(ctx context.Context, db *gorm.DB, idsPath string, apply bool, samples int) error {
	forumIDs, err := readForumIDs(idsPath)
	if err != nil {
		return err
	}
	rows, err := loadReclassifyRows(ctx, db, forumIDs)
	if err != nil {
		return err
	}
	works := map[int64]struct{}{}
	var toMachine []int64
	var keptSource, alreadyMachine int
	for _, r := range rows {
		works[r.WorkID] = struct{}{}
		switch {
		case r.SourceBacked:
			keptSource++
		case r.Provenance == model.WorkTitleProvenanceMachine:
			alreadyMachine++
		default:
			toMachine = append(toMachine, r.TitleID)
		}
	}
	fmt.Printf("\n=== backfill-work-zh-titles reclassify-forum-mt (%s) ===\n", modeLabel(apply))
	fmt.Printf("forum_ids=%d resolved_works=%d zh_title_rows=%d → machine=%d kept_source_backed=%d already_machine=%d\n",
		len(forumIDs), len(works), len(rows), len(toMachine), keptSource, alreadyMachine)
	shown := 0
	for _, r := range rows {
		if shown >= samples {
			break
		}
		verdict := "→machine"
		switch {
		case r.SourceBacked:
			verdict = "keep_source(verbatim in bgm/vndb)"
		case r.Provenance == model.WorkTitleProvenanceMachine:
			verdict = "already_machine"
		}
		fmt.Printf("  forum=%d work=%d %s %q %s\n", r.ProductWorkID, r.WorkID, r.Lang, r.Title, verdict)
		shown++
	}
	if !apply {
		fmt.Println("\n[dry run] nothing written — re-run with --apply")
		return nil
	}
	if len(toMachine) == 0 {
		fmt.Println("\nnothing to reclassify")
		return nil
	}
	touched := make([]int64, 0, len(works))
	for id := range works {
		touched = append(touched, id)
	}
	slices.Sort(touched)
	var updated int64
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`UPDATE catalog_work_title SET provenance = ? WHERE id IN ? AND provenance <> ?`,
			model.WorkTitleProvenanceMachine, toMachine, model.WorkTitleProvenanceMachine)
		if res.Error != nil {
			return res.Error
		}
		updated = res.RowsAffected
		return repository.TouchWorks(ctx, tx, touched)
	})
	if err != nil {
		return fmt.Errorf("reclassify forum titles: %w", err)
	}
	fmt.Printf("\nreclassified=%d touched_works=%d\n", updated, len(touched))
	return nil
}
