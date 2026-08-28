package main

import (
	"context"
	"time"

	"api/internal/platform/catalog/service"

	"gorm.io/gorm"
)

type pairRow struct {
	A              int64      `gorm:"column:a"`
	B              int64      `gorm:"column:b"`
	SharedNorm     string     `gorm:"column:shared_norm"`
	SharedNorms    int        `gorm:"column:shared_norms"`
	LaneA          string     `gorm:"column:lane_a"`
	LaneB          string     `gorm:"column:lane_b"`
	AnchorsA       int        `gorm:"column:anchors_a"`
	AnchorsB       int        `gorm:"column:anchors_b"`
	SiteA          *string    `gorm:"column:site_a"`
	SiteB          *string    `gorm:"column:site_b"`
	NameA          string     `gorm:"column:name_a"`
	NameB          string     `gorm:"column:name_b"`
	AnchorConflict bool       `gorm:"column:anchor_conflict"`
	RelConflict    bool       `gorm:"column:release_conflict"`
	RefOverlap     bool       `gorm:"column:ref_overlap"`
	DateA          *time.Time `gorm:"column:date_a"`
	DateB          *time.Time `gorm:"column:date_b"`
	LabelOverlap   bool       `gorm:"column:label_overlap"`
}

// pairQuerySQL is the standing duplicate detector: every pair of live works
// sharing a folded official-title/display_name norm, scored with the
// corroborating facts the verdict needs. Work-level identity anchors drive
// lanes and conflicts; dlsite identity lives on releases (entity_type 6), so
// the dlsite lane and the release-level conflict/overlap read through
// catalog_release — a work-level-only read files the whole dlsite family
// under 'other' and misses every edition split.
func pairQuerySQL() string {
	return `
WITH lw AS (
  SELECT id, site, display_name FROM catalog_work WHERE deleted_at IS NULL
),
norms AS (
  SELECT DISTINCT work_id, n FROM (` + service.WorkDupeCorpusSQL() + `) c
),
pairs AS (
  SELECT a.work_id AS a, b.work_id AS b, min(a.n) AS shared_norm, count(*) AS shared_norms
  FROM norms a JOIN norms b ON a.n = b.n AND a.work_id < b.work_id
  WHERE length(a.n) >= 4
  GROUP BY a.work_id, b.work_id
),
wanchor AS (
  SELECT entity_id AS work_id, source_id, external_id
  FROM catalog_external_ref
  WHERE entity_type = 5 AND link_kind = 0 AND dead_at IS NULL
),
ranchor AS (
  SELECT rel.work_id, r.source_id, r.external_id
  FROM catalog_external_ref r
  JOIN catalog_release rel ON rel.id = r.entity_id AND rel.deleted_at IS NULL
  WHERE r.entity_type = 6 AND r.link_kind = 0 AND r.dead_at IS NULL
),
lane AS (
  SELECT lw.id,
    CASE WHEN lw.site IS NOT NULL AND lw.site <> '' THEN 'kungal'
         WHEN EXISTS (SELECT 1 FROM wanchor x WHERE x.work_id = lw.id AND x.source_id = 2) THEN 'vndb'
         WHEN EXISTS (SELECT 1 FROM wanchor x WHERE x.work_id = lw.id AND x.source_id = 3) THEN 'bgm'
         WHEN EXISTS (SELECT 1 FROM wanchor x WHERE x.work_id = lw.id AND x.source_id = 5) THEN 'eg'
         WHEN EXISTS (SELECT 1 FROM ranchor x WHERE x.work_id = lw.id AND x.source_id = 4) THEN 'dlsite'
         ELSE 'other' END AS lane,
    (SELECT count(*) FROM wanchor x WHERE x.work_id = lw.id) AS anchors
  FROM lw
),
wdate AS (
  SELECT work_id, min(make_date(released_y, coalesce(nullif(released_m,0),1), coalesce(nullif(released_d,0),1))) AS d
  FROM catalog_release
  WHERE deleted_at IS NULL AND released_y IS NOT NULL
  GROUP BY work_id
),
bgmdate AS (
  SELECT r.work_id, min(s.date::date) AS d
  FROM wanchor r
  JOIN src_bangumi.subject s ON s.id = r.external_id::bigint
  WHERE r.source_id = 3 AND s.date ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
  GROUP BY r.work_id
)
SELECT p.a, p.b, p.shared_norm, p.shared_norms,
  la.lane AS lane_a, lb.lane AS lane_b,
  la.anchors AS anchors_a, lb.anchors AS anchors_b,
  wa.site AS site_a, wb.site AS site_b,
  wa.display_name AS name_a, wb.display_name AS name_b,
  EXISTS (SELECT 1 FROM wanchor x JOIN wanchor y ON y.source_id = x.source_id AND y.external_id <> x.external_id
          WHERE x.work_id = p.a AND y.work_id = p.b) AS anchor_conflict,
  (EXISTS (SELECT 1 FROM ranchor x JOIN ranchor y ON y.source_id = x.source_id AND y.external_id <> x.external_id
           WHERE x.work_id = p.a AND y.work_id = p.b)
   AND NOT EXISTS (SELECT 1 FROM ranchor x JOIN ranchor y ON y.source_id = x.source_id AND y.external_id = x.external_id
                   WHERE x.work_id = p.a AND y.work_id = p.b)) AS release_conflict,
  (EXISTS (SELECT 1 FROM catalog_external_ref x
           JOIN catalog_external_ref y ON y.source_id = x.source_id AND y.external_id = x.external_id
           WHERE x.entity_type = 5 AND x.entity_id = p.a AND x.dead_at IS NULL
             AND y.entity_type = 5 AND y.entity_id = p.b AND y.dead_at IS NULL)
   OR EXISTS (SELECT 1 FROM ranchor x JOIN ranchor y ON y.source_id = x.source_id AND y.external_id = x.external_id
              WHERE x.work_id = p.a AND y.work_id = p.b)) AS ref_overlap,
  coalesce(da.d, ba.d) AS date_a,
  coalesce(dbb.d, bb.d) AS date_b,
  EXISTS (SELECT 1 FROM catalog_work_label x JOIN catalog_work_label y ON y.label_id = x.label_id
          WHERE x.work_id = p.a AND y.work_id = p.b) AS label_overlap
FROM pairs p
JOIN lane la ON la.id = p.a
JOIN lane lb ON lb.id = p.b
JOIN lw wa ON wa.id = p.a
JOIN lw wb ON wb.id = p.b
LEFT JOIN wdate da ON da.work_id = p.a
LEFT JOIN wdate dbb ON dbb.work_id = p.b
LEFT JOIN bgmdate ba ON ba.work_id = p.a
LEFT JOIN bgmdate bb ON bb.work_id = p.b
ORDER BY p.a, p.b`
}

type census struct {
	rows     []pairRow
	verdicts []bucket
	groups   []mergeGroup
}

func buildCensus(ctx context.Context, db *gorm.DB) (*census, error) {
	var rows []pairRow
	if err := db.WithContext(ctx).Raw(pairQuerySQL()).Scan(&rows).Error; err != nil {
		return nil, err
	}
	verdicts, groups := classify(rows)
	return &census{rows: rows, verdicts: verdicts, groups: groups}, nil
}

func (c *census) verdictByPair() map[[2]int64]bucket {
	out := make(map[[2]int64]bucket, len(c.rows))
	for i := range c.rows {
		out[[2]int64{c.rows[i].A, c.rows[i].B}] = c.verdicts[i]
	}
	return out
}

func (c *census) rowByPair() map[[2]int64]pairRow {
	out := make(map[[2]int64]pairRow, len(c.rows))
	for _, r := range c.rows {
		out[[2]int64{r.A, r.B}] = r
	}
	return out
}
