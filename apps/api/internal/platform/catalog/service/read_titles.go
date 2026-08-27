package service

import (
	"context"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"
)

type WorkTitleRow struct {
	Lang       string
	Title      string
	Latin      string
	Kind       int16
	Provenance int16
}

func (s *ReadService) loadWorkTitles(ctx context.Context, subjects []claimSubject) (map[int64][]WorkTitleRow, error) {
	return s.loadTitles(ctx, subjects, false)
}

func (s *ReadService) loadWorkDetailTitles(ctx context.Context, subjects []claimSubject) (map[int64][]WorkTitleRow, error) {
	return s.loadTitles(ctx, subjects, true)
}

func (s *ReadService) loadTitles(ctx context.Context, subjects []claimSubject, withHints bool) (map[int64][]WorkTitleRow, error) {
	out := make(map[int64][]WorkTitleRow, len(subjects))
	if len(subjects) == 0 {
		return out, nil
	}
	workIDs := make([]int64, 0, len(subjects))
	for _, sub := range subjects {
		workIDs = append(workIDs, sub.WorkID)
	}
	return out, s.nativeWorkTitles(ctx, workIDs, out, withHints)
}

func (s *ReadService) nativeWorkTitles(ctx context.Context, workIDs []int64, out map[int64][]WorkTitleRow, withHints bool) error {
	live := editspec.NotSuppressedWorkTitleSQL("t")
	// provenance leads the ORDER BY, ahead of kind: a source row always beats a
	// machine one inside a locale, which is what makes the election in
	// workLocalized a plain first-row-wins scan (wave 210, the aliasBeats
	// philosophy of wave 209 pushed down into SQL).
	const cols = `work_id, lang, title, coalesce(latin, '') AS latin, kind, provenance`
	q := `SELECT ` + cols + ` FROM catalog_work_title t
		WHERE work_id IN ? AND kind <> ? AND ` + live + ` ORDER BY work_id, provenance, kind, id`
	args := []any{workIDs, model.WorkTitleKindSearchHint}
	if withHints {
		q = `SELECT ` + cols + ` FROM catalog_work_title t
			WHERE work_id IN ? AND ` + live + ` ORDER BY work_id, provenance, kind, lang, id`
		args = []any{workIDs}
	}
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		Lang       string `gorm:"column:lang"`
		Title      string `gorm:"column:title"`
		Latin      string `gorm:"column:latin"`
		Kind       int16  `gorm:"column:kind"`
		Provenance int16  `gorm:"column:provenance"`
	}
	if err := s.db.WithContext(ctx).Raw(q, args...).Scan(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		out[r.WorkID] = append(out[r.WorkID], WorkTitleRow{
			Lang: r.Lang, Title: r.Title, Latin: r.Latin, Kind: r.Kind, Provenance: r.Provenance,
		})
	}
	return nil
}
