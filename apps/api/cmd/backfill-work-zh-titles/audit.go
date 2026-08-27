package main

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// The post-audit answers the question the per-work lane structurally cannot:
// after everything is written, do the members of a series still agree on the
// name they share?

type seriesDisagreement struct {
	SeriesID   int64
	SeriesName string
	JaPrefix   string
	Members    []string
}

type seriesTitleRow struct {
	SeriesID   int64  `gorm:"column:series_id"`
	SeriesName string `gorm:"column:series_name"`
	WorkID     int64  `gorm:"column:work_id"`
	JaTitle    string `gorm:"column:ja_title"`
	ZhTitle    string `gorm:"column:zh_title"`
}

func loadSeriesTitleRows(ctx context.Context, db *gorm.DB) ([]seriesTitleRow, error) {
	var rows []seriesTitleRow
	err := db.WithContext(ctx).Raw(`
		SELECT s.id AS series_id, s.display_name AS series_name, m.work_id,
			COALESCE((SELECT t.title FROM catalog_work_title t
			           WHERE t.work_id = m.work_id AND (t.lang = 'ja' OR t.lang LIKE 'ja-%')
			             AND t.kind <= 1 ORDER BY t.kind, t.id LIMIT 1), '') AS ja_title,
			COALESCE((SELECT t.title FROM catalog_work_title t
			           WHERE t.work_id = m.work_id AND t.lang LIKE 'zh%' AND t.kind <= 1
			           ORDER BY t.provenance, t.kind, t.id LIMIT 1), '') AS zh_title
		FROM catalog_series_member m
		JOIN catalog_series s ON s.id = m.series_id
		JOIN catalog_work w ON w.id = m.work_id AND w.deleted_at IS NULL
		ORDER BY s.id, m.work_id`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load series title rows: %w", err)
	}
	return rows, nil
}

const minSharedPrefixRunes = 2

// auditSeries reports the series whose members share a Japanese title prefix
// but whose Chinese titles do not — the shape a per-work translation lane
// produces when nothing tells it about the siblings.
func auditSeries(rows []seriesTitleRow) []seriesDisagreement {
	type bucket struct {
		name       string
		ja, zh     []string
		memberDesc []string
	}
	order := []int64{}
	byID := map[int64]*bucket{}
	for _, r := range rows {
		if r.JaTitle == "" || r.ZhTitle == "" {
			continue
		}
		b, ok := byID[r.SeriesID]
		if !ok {
			b = &bucket{name: r.SeriesName}
			byID[r.SeriesID] = b
			order = append(order, r.SeriesID)
		}
		b.ja = append(b.ja, r.JaTitle)
		b.zh = append(b.zh, r.ZhTitle)
		b.memberDesc = append(b.memberDesc, fmt.Sprintf("%d %s → %s", r.WorkID, r.JaTitle, r.ZhTitle))
	}
	var out []seriesDisagreement
	for _, id := range order {
		b := byID[id]
		if len(b.ja) < 2 {
			continue
		}
		jaPrefix := commonPrefix(b.ja)
		if runeLen(jaPrefix) < minSharedPrefixRunes {
			continue
		}
		if runeLen(commonPrefix(b.zh)) >= minSharedPrefixRunes {
			continue
		}
		out = append(out, seriesDisagreement{
			SeriesID: id, SeriesName: b.name, JaPrefix: jaPrefix, Members: b.memberDesc,
		})
	}
	return out
}

func commonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	prefix := []rune(ss[0])
	for _, s := range ss[1:] {
		r := []rune(s)
		n := min(len(prefix), len(r))
		i := 0
		for i < n && prefix[i] == r[i] {
			i++
		}
		prefix = prefix[:i]
		if len(prefix) == 0 {
			return ""
		}
	}
	return strings.TrimRight(string(prefix), " 　")
}

func runeLen(s string) int { return len([]rune(s)) }

func runSeriesAudit(ctx context.Context, db *gorm.DB, samples int) error {
	rows, err := loadSeriesTitleRows(ctx, db)
	if err != nil {
		return err
	}
	bad := auditSeries(rows)
	fmt.Printf("\n=== backfill-work-zh-titles series audit ===\nseries_rows=%d disagreeing_series=%d\n",
		len(rows), len(bad))
	for i, d := range bad {
		if i >= samples {
			break
		}
		fmt.Printf("  series=%d %q ja_prefix=%q\n", d.SeriesID, d.SeriesName, d.JaPrefix)
		for _, m := range d.Members {
			fmt.Printf("      %s\n", m)
		}
	}
	return nil
}
