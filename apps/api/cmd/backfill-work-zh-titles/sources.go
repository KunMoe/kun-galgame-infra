package main

import (
	"context"
	"fmt"
	"slices"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	langZhHans = "zh-Hans"
	langZhHant = "zh-Hant"
)

// zhSlotSQL renders a catalog_work_title.lang as the zh SLOT it occupies:
// zh-Hant/zh-TW/zh-HK are the traditional slot, every other zh* tag is the
// simplified one. NULL for non-Chinese tags.
const zhSlotSQL = `CASE
	WHEN t.lang IN ('zh-Hant','zh-TW','zh-HK') OR t.lang LIKE 'zh-Hant-%' THEN 'zh-Hant'
	WHEN t.lang LIKE 'zh%' THEN 'zh-Hans' END`

type supplyRow struct {
	WorkID int64  `gorm:"column:work_id"`
	Lang   string `gorm:"column:lang"`
	Title  string `gorm:"column:title"`
}

// loadBgmSupply lists the works with an exact bangumi anchor whose subject
// publishes a name_cn and that carry no SOURCE Chinese title at all. Bangumi's
// name_cn is simplified, so it fills the zh-Hans slot; the work-level (rather
// than per-slot) gate is the charter's, and keeps this lane from parking a
// simplified name beside a traditional one a source already published.
//
// Only provenance=0 rows block: 45 of the 605 reclassified forum works have a
// human bgm/vndb title that is NOT the machine string, and a gate blind to
// provenance would have locked them out of source supply forever, leaving
// re-translation as their only exit.
func loadBgmSupply(ctx context.Context, db *gorm.DB, limit int) ([]supplyRow, error) {
	q := `
	WITH anchored AS (
		SELECT w.id AS work_id, s.name_cn AS title
		FROM catalog_work w
		JOIN catalog_external_ref r
		  ON r.entity_type = ? AND r.entity_id = w.id
		 AND r.source_id = (SELECT id FROM catalog_source WHERE key = 'bangumi')
		 AND r.link_kind = ? AND r.dead_at IS NULL
		JOIN src_bangumi.subject s ON s.id = r.external_id::bigint
		WHERE w.deleted_at IS NULL AND s.name_cn <> ''
	)
	SELECT DISTINCT ON (a.work_id) a.work_id, 'zh-Hans' AS lang, a.title
	FROM anchored a
	WHERE NOT EXISTS (
		SELECT 1 FROM catalog_work_title t
		WHERE t.work_id = a.work_id AND t.lang LIKE 'zh%' AND t.provenance = ?)
	ORDER BY a.work_id, a.title`
	var rows []supplyRow
	err := db.WithContext(ctx).Raw(q,
		model.EntityTypeWork, model.LinkKindExact,
		model.WorkTitleProvenanceSource).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load bangumi zh supply: %w", err)
	}
	return capRows(rows, limit), nil
}

// vndbReleaseDecorationSQL names the release-packaging vocabulary that must
// not become a work title ("悠之空 - 18+ 補丁", "魔女的夜宴 - FHD Edition").
// Skipped works are never string-stripped: their zh arrives from the MT lane,
// honestly labeled, and a later clean release replaces it via the
// machine-supersede path. Measured on prod twice (2026-08-19): the first cut
// caught 8.2% of elected titles but was an undercount — it had the Japanese
// and traditional glyphs (体験版/體驗版) without the simplified twin (体验版),
// and killing a decorated winner promotes the next-most-agreed sibling, which
// can itself be decorated (先行版, 18禁DLC, 语音项目, 外挂, 中国語版). The
// list was then iterated in a read-only rehearsal until a broad sweep found
// nothing new. The survivors that look decorated are real names — 心跳回忆
// 女生版 is Girl's Side, 真實色情遊戲情境體驗 is the title itself — so bare
// 版 must never join the list.
const vndbReleaseDecorationSQL = `(rt.title !~* '(下載版|下载版|ダウンロード|DL版|補丁|补丁|パッチ|patch|edition|version|remake|remaster|初回|限定|通常版|体験版|體驗版|体验版|demo|trial|censored|全年齢|18\+|R18|R-18|廉価|复刻|復刻|豪華|豪华|完全版|加強|加强|移植|同梱|同捆|特典|首發|首发|合集|漢化|汉化|中文化|中文版|试玩版|試玩版|测试版|測試版|一般版|普通版|平裝|平装|特裝|特装|限量|特別版|特别版|解禁|摘要版|高清|検閲|审查|審查|携帯|携带|攜帶|重製|重制|重置|最終版|最终版|正式版|典藏版|紀念|纪念|小劇場|小剧场|版本|先行版|DLC|18禁|語音|语音|外挂|外掛|中国語|中國語|韓文版|韩文版|繁體中文|繁体中文|簡體中文|简体中文|v[0-9]+\.[0-9])')`

// loadVndbSupply picks, per (work, zh slot), the title most releases agree on,
// breaking ties on the earliest release id. mtl=false is the whole point: VNDB
// marks machine-translated release titles and this is a SOURCE lane.
//
// The gate here is per-slot, not per-work: a work that already has a
// traditional title still takes a simplified supply and vice versa. As in the
// bgm lane, only provenance=0 rows block — a machine row in the slot is
// superseded, not respected.
//
// Two structural guards ride with the vocabulary. A zh row must contain a Han
// character: vndb stores romanizations ("Xinshiji Fuyin Zhanshi…") and plain
// Latin/kana strings in zh release rows, and none of them is a Chinese name.
// And only single-VN releases testify: releases_vn is many-to-many, so a
// bundle stamps its compilation title on every VN it contains — nine works
// wore "告别回憶 歷代回憶錄 限量版" and Rance 1 wore 婦警さんVX's bundle name.
func loadVndbSupply(ctx context.Context, db *gorm.DB, limit int) ([]supplyRow, error) {
	q := `
	WITH anchored AS (
		SELECT w.id AS work_id, r.external_id AS vid
		FROM catalog_work w
		JOIN catalog_external_ref r
		  ON r.entity_type = ? AND r.entity_id = w.id
		 AND r.source_id = (SELECT id FROM catalog_source WHERE key = 'vndb')
		 AND r.link_kind = ? AND r.dead_at IS NULL
		WHERE w.deleted_at IS NULL
	),
	supply AS (
		SELECT a.work_id, rt.lang, rt.title, count(*) AS agree,
		       min(NULLIF(regexp_replace(rv.id, '^r', ''), '')::bigint) AS first_rid
		FROM anchored a
		JOIN src_vndb.releases_vn rv ON rv.vid = a.vid
		JOIN src_vndb.releases_titles rt ON rt.id = rv.id
		WHERE rt.lang IN (?, ?) AND rt.mtl = false AND rt.title <> ''
		  AND rt.title ~ '[一-鿿]'
		  AND NOT EXISTS (SELECT 1 FROM src_vndb.releases_vn rv2
		                  WHERE rv2.id = rv.id AND rv2.vid <> rv.vid)
		  AND ` + vndbReleaseDecorationSQL + `
		GROUP BY a.work_id, rt.lang, rt.title
	)
	SELECT DISTINCT ON (s.work_id, s.lang) s.work_id, s.lang, s.title
	FROM supply s
	WHERE NOT EXISTS (
		SELECT 1 FROM catalog_work_title t
		WHERE t.work_id = s.work_id AND t.provenance = ? AND (` + zhSlotSQL + `) = s.lang)
	ORDER BY s.work_id, s.lang, s.agree DESC, s.first_rid`
	var rows []supplyRow
	err := db.WithContext(ctx).Raw(q,
		model.EntityTypeWork, model.LinkKindExact,
		langZhHans, langZhHant,
		model.WorkTitleProvenanceSource).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load vndb zh supply: %w", err)
	}
	return capRows(rows, limit), nil
}

func capRows(rows []supplyRow, limit int) []supplyRow {
	if limit > 0 && limit < len(rows) {
		return rows[:limit]
	}
	return rows
}

type sourceLane struct {
	name string
	load func(context.Context, *gorm.DB, int) ([]supplyRow, error)
}

func laneFor(name string) (sourceLane, error) {
	switch name {
	case "bgm":
		return sourceLane{name: "bgm", load: loadBgmSupply}, nil
	case "vndb":
		return sourceLane{name: "vndb", load: loadVndbSupply}, nil
	default:
		return sourceLane{}, fmt.Errorf("unknown --source %q (want bgm or vndb)", name)
	}
}

const supplyInsertBatch = 500

func runSource(ctx context.Context, db *gorm.DB, name string, apply bool, limit, samples int) error {
	lane, err := laneFor(name)
	if err != nil {
		return err
	}
	rows, err := lane.load(ctx, db, limit)
	if err != nil {
		return err
	}
	perLang := map[string]int{}
	works := map[int64]struct{}{}
	for _, r := range rows {
		perLang[r.Lang]++
		works[r.WorkID] = struct{}{}
	}
	fmt.Printf("\n=== backfill-work-zh-titles source=%s (%s) ===\n", lane.name, modeLabel(apply))
	fmt.Printf("supply_rows=%d works=%d zh-Hans=%d zh-Hant=%d\n",
		len(rows), len(works), perLang[langZhHans], perLang[langZhHant])
	for i, r := range rows {
		if i >= samples {
			break
		}
		fmt.Printf("  sample: work=%d %s %q\n", r.WorkID, r.Lang, r.Title)
	}
	if !apply {
		fmt.Println("\n[dry run] nothing written — re-run with --apply")
		return nil
	}

	titles := make([]model.CatalogWorkTitle, 0, len(rows))
	touched := make([]int64, 0, len(works))
	for _, r := range rows {
		titles = append(titles, model.CatalogWorkTitle{
			WorkID: r.WorkID, Lang: r.Lang, Title: r.Title,
			Kind: model.WorkTitleKindOfficial, Provenance: model.WorkTitleProvenanceSource,
		})
	}
	for id := range works {
		touched = append(touched, id)
	}
	slices.Sort(touched)

	bySlot := map[string][]int64{}
	for _, r := range rows {
		bySlot[r.Lang] = append(bySlot[r.Lang], r.WorkID)
	}

	var written, replacedMachine int64
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for slot, ids := range bySlot {
			res := tx.Exec(`DELETE FROM catalog_work_title t
				WHERE t.provenance = ? AND t.work_id IN ? AND (`+zhSlotSQL+`) = ?`,
				model.WorkTitleProvenanceMachine, ids, slot)
			if res.Error != nil {
				return res.Error
			}
			replacedMachine += res.RowsAffected
		}
		if len(titles) > 0 {
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(titles, supplyInsertBatch)
			if res.Error != nil {
				return res.Error
			}
			written = res.RowsAffected
		}
		return repository.TouchWorks(ctx, tx, touched)
	})
	if err != nil {
		return fmt.Errorf("write %s supply: %w", lane.name, err)
	}
	fmt.Printf("\nwritten=%d already=%d replaced_machine=%d touched_works=%d\n",
		written, int64(len(titles))-written, replacedMachine, len(touched))
	return nil
}

func modeLabel(apply bool) string {
	if apply {
		return "APPLY"
	}
	return "DRY"
}
