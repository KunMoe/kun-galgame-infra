package main

import (
	"context"
	"fmt"
	"io"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

const crossMediumSamples = 10

type crossMediumRow struct {
	AID     int64  `gorm:"column:a_id"`
	BID     int64  `gorm:"column:b_id"`
	MediumA int16  `gorm:"column:medium_a"`
	MediumB int16  `gorm:"column:medium_b"`
	NameA   string `gorm:"column:name_a"`
	NameB   string `gorm:"column:name_b"`
}

func runCrossMedium(ctx context.Context, db *gorm.DB, w io.Writer, actor int64, run bool) error {
	var rows []crossMediumRow
	if err := db.WithContext(ctx).Raw(`
SELECT c.a_id, c.b_id, wa.medium_id AS medium_a, wb.medium_id AS medium_b,
  wa.display_name AS name_a, wb.display_name AS name_b
FROM catalog_match_candidate c
JOIN catalog_work wa ON wa.id = c.a_id
JOIN catalog_work wb ON wb.id = c.b_id
WHERE c.entity_type = ? AND c.status IN (?, ?, ?)
  AND wa.medium_id <> wb.medium_id
ORDER BY c.a_id, c.b_id`,
		model.EntityTypeWork,
		model.CandidateStatusPending,
		model.CandidateStatusDeferred,
		model.CandidateStatusNeedsManual,
	).Scan(&rows).Error; err != nil {
		return err
	}

	mode := "DRY-RUN (pass -run to reject cross-medium candidates)"
	if run {
		mode = "APPLIED"
	}
	fmt.Fprintf(w, "%s [crossmedium] actor=%d n=%d\n", mode, actor, len(rows))
	for i, r := range rows {
		if i >= crossMediumSamples {
			fmt.Fprintf(w, "  … and %d more\n", len(rows)-crossMediumSamples)
			break
		}
		fmt.Fprintf(w, "  %d<->%d medium=%d×%d a=%q b=%q\n",
			r.AID, r.BID, r.MediumA, r.MediumB, r.NameA, r.NameB)
	}
	if !run {
		return nil
	}

	res := db.WithContext(ctx).Exec(`
UPDATE catalog_match_candidate AS c
SET status = ?, decided_by = ?, decided_at = now()
FROM catalog_work wa, catalog_work wb
WHERE c.entity_type = ?
  AND c.status IN (?, ?, ?)
  AND wa.id = c.a_id AND wb.id = c.b_id
  AND wa.medium_id <> wb.medium_id`,
		model.CandidateStatusRejected, actor,
		model.EntityTypeWork,
		model.CandidateStatusPending,
		model.CandidateStatusDeferred,
		model.CandidateStatusNeedsManual,
	)
	if res.Error != nil {
		return res.Error
	}
	fmt.Fprintf(w, "  rejected=%d\n", res.RowsAffected)
	return nil
}
