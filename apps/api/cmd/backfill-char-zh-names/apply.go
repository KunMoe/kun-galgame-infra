package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type reviewedName struct {
	CharacterID int64
	Name        string
	Zh          string
	Model       string
	Line        int
}

// readReviewedCSV accepts the --mt output after human review. Rows whose
// proposed_zh was blanked (or left SKIP) are the reviewer's rejections and are
// simply skipped; a trailing ° or ' quality marker is tolerated and stripped
// (the trait-review convention).
func readReviewedCSV(path string) ([]reviewedName, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, fmt.Errorf("%s: empty CSV", path)
	}
	idCol := indexOf(recs[0], "character_id")
	nameCol := indexOf(recs[0], "name")
	zhCol := indexOf(recs[0], "proposed_zh")
	modelCol := indexOf(recs[0], "model")
	if idCol < 0 || nameCol < 0 || zhCol < 0 {
		return nil, fmt.Errorf("%s: header must contain character_id, name and proposed_zh", path)
	}
	var out []reviewedName
	for i, rec := range recs[1:] {
		if idCol >= len(rec) || nameCol >= len(rec) || zhCol >= len(rec) {
			continue
		}
		zh := stripQualityMarkers(rec[zhCol])
		if zh == "" || strings.EqualFold(zh, skipMarker) {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSpace(rec[idCol]), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: bad character_id %q", path, i+2, rec[idCol])
		}
		m := ""
		if modelCol >= 0 && modelCol < len(rec) {
			m = strings.TrimSpace(rec[modelCol])
		}
		out = append(out, reviewedName{
			CharacterID: id, Name: strings.TrimSpace(rec[nameCol]), Zh: zh, Model: m, Line: i + 2,
		})
	}
	return out, nil
}

func stripQualityMarkers(s string) string {
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), "°'"))
}

// runApplyCSV writes the reviewed names as machine rows: provenance=machine,
// never the locale primary, so they reach aliases[], search and the MT
// glossaries but are refused by the public localized{} face. A proposal equal
// to the character's current display name is dropped — an identity row is the
// passthrough lane's decision, not the model's.
func runApplyCSV(ctx context.Context, db *gorm.DB, path string, samples, limit int) error {
	rows, err := readReviewedCSV(path)
	if err != nil {
		return err
	}
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	fmt.Printf("\n=== backfill-char-zh-names APPLY reviewed CSV (%d accepted rows this run) ===\n", len(rows))
	var inserted, conflict, sameAsDisplay, missing, errs int
	touched := make([]int64, 0, len(rows))
	shown := 0
	for _, r := range rows {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var current struct {
			DisplayName string `gorm:"column:display_name"`
		}
		res := db.WithContext(ctx).Raw(`SELECT display_name FROM catalog_character
			WHERE id = ? AND deleted_at IS NULL`, r.CharacterID).Scan(&current)
		if res.Error != nil {
			errs++
			continue
		}
		if res.RowsAffected == 0 {
			missing++
			continue
		}
		if r.Zh == current.DisplayName {
			sameAsDisplay++
			continue
		}
		create := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "character_id"}, {Name: "name"}, {Name: "lang"}},
			DoNothing: true,
		}).Create(&model.CatalogCharacterAlias{
			CharacterID:        r.CharacterID,
			Name:               r.Zh,
			Lang:               langZhHans,
			Kind:               model.AliasKindTranslation,
			IsPrimaryForLocale: false,
			Provenance:         model.AliasProvenanceMachine,
			MTModel:            r.Model,
		})
		if create.Error != nil {
			errs++
			continue
		}
		if create.RowsAffected == 0 {
			conflict++
			continue
		}
		inserted++
		touched = append(touched, r.CharacterID)
		if shown < samples {
			fmt.Printf("  sample: %d %s → %s\n", r.CharacterID, r.Name, r.Zh)
			shown++
		}
	}
	works, err := hostWorks(ctx, db, touched)
	if err != nil {
		return err
	}
	if err := repository.TouchWorks(ctx, db, works); err != nil {
		return fmt.Errorf("touch host works: %w", err)
	}
	fmt.Printf("\ninserted=%d conflict=%d same_as_display=%d missing_character=%d errors=%d touched_works=%d\n",
		inserted, conflict, sameAsDisplay, missing, errs, len(works))
	if errs > 0 {
		os.Exit(1)
	}
	return nil
}

func indexOf(row []string, col string) int {
	for i, c := range row {
		if strings.TrimSpace(c) == col {
			return i
		}
	}
	return -1
}
