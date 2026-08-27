// Package imagerefs is the ONE registry of every catalog row that holds an
// image hash. An image column missing from it is invisible to the daily refping
// sweep, to image-ref-audit, and to the console's pre-delete reference check.
// Nothing fails when you forget: uploads succeed, the read face renders, and
// the bytes are collected a year later once the image service's TTL elapses —
// the "refping site-scope GC fuse" class that once froze 66k galgame images.
// Adding an image column in catalog scope means adding it HERE in the same
// change.
package imagerefs

import (
	"context"
	"fmt"
	"strings"

	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

const (
	KindWorkCover      = "work_cover"
	KindWorkScreenshot = "work_screenshot"
	KindCharacterBust  = "character_bust"
	// Same story as KindCharacterFigureSource, for the bust slot.
	KindCharacterBustSource = "character_bust_source"
	KindCharacterFigure     = "character_figure"
	// The white-plate original a cut-out figure_hash was derived from. It is
	// referenced by nothing the read face renders, which is exactly why it has
	// to be listed here: unlisted bytes are collected once the image service
	// TTL elapses, and then the cut-outs can never be regenerated.
	KindCharacterFigureSource = "character_figure_source"
	KindLabelLogo             = "label_logo"
	KindPersonPhoto           = "person_photo"
)

type kindSpec struct {
	Kind         string
	Table        string
	HashColumn   string
	EntityColumn string
	LabelTable   string
	LabelJoinOn  string
	LabelColumn  string
	Filter       string
	DetachSet    string
	// Works whose public face renders this kind's rows, so a detach can move
	// their /v1/catalog/changes watermark. Empty for the kinds that reach only
	// the character and person faces.
	WorkIDsSQL string
}

var specs = []kindSpec{
	{
		Kind: KindWorkCover, Table: "catalog_work_cover",
		HashColumn: "image_hash", EntityColumn: "work_id",
		LabelTable: "catalog_work", LabelJoinOn: "l.id = t.work_id", LabelColumn: "display_name",
		Filter: "", DetachSet: "",
		WorkIDsSQL: `SELECT work_id FROM catalog_work_cover WHERE image_hash = ?`,
	},
	{
		Kind: KindWorkScreenshot, Table: "catalog_work_screenshot",
		HashColumn: "image_hash", EntityColumn: "work_id",
		LabelTable: "catalog_work", LabelJoinOn: "l.id = t.work_id", LabelColumn: "display_name",
		Filter: "", DetachSet: "",
		WorkIDsSQL: `SELECT work_id FROM catalog_work_screenshot WHERE image_hash = ?`,
	},
	{
		Kind: KindCharacterBust, Table: "catalog_character",
		HashColumn: "image_hash", EntityColumn: "id", LabelColumn: "display_name",
		Filter: "t.deleted_at IS NULL", DetachSet: "NULL",
	},
	{
		Kind: KindCharacterBustSource, Table: "catalog_character",
		HashColumn: "image_source_hash", EntityColumn: "id", LabelColumn: "display_name",
		Filter: "t.deleted_at IS NULL", DetachSet: "NULL",
	},
	{
		Kind: KindCharacterFigure, Table: "catalog_character",
		HashColumn: "figure_hash", EntityColumn: "id", LabelColumn: "display_name",
		Filter: "t.deleted_at IS NULL", DetachSet: "NULL",
	},
	{
		Kind: KindCharacterFigureSource, Table: "catalog_character",
		HashColumn: "figure_source_hash", EntityColumn: "id", LabelColumn: "display_name",
		Filter: "t.deleted_at IS NULL", DetachSet: "NULL",
	},
	{
		Kind: KindLabelLogo, Table: "catalog_label",
		HashColumn: "logo_hash", EntityColumn: "id", LabelColumn: "display_name",
		Filter: "t.deleted_at IS NULL", DetachSet: "''",
		WorkIDsSQL: `SELECT wl.work_id FROM catalog_work_label wl
			JOIN catalog_label l ON l.id = wl.label_id
			WHERE l.logo_hash = ? AND l.deleted_at IS NULL`,
	},
	{
		Kind: KindPersonPhoto, Table: "catalog_person",
		HashColumn: "photo_hash", EntityColumn: "id", LabelColumn: "display_name",
		Filter: "t.deleted_at IS NULL", DetachSet: "''",
	},
}

type Ref struct {
	Hash     string `gorm:"column:hash"`
	Kind     string `gorm:"column:kind"`
	EntityID int64  `gorm:"column:entity_id"`
	Label    string `gorm:"column:label"`
}

func Kinds() []string {
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, s.Kind)
	}
	return out
}

func (s kindSpec) hashPresent() string {
	return fmt.Sprintf("t.%s IS NOT NULL AND t.%s <> ''", s.HashColumn, s.HashColumn)
}

func (s kindSpec) where(extra ...string) string {
	parts := []string{s.hashPresent()}
	if s.Filter != "" {
		parts = append(parts, s.Filter)
	}
	parts = append(parts, extra...)
	return strings.Join(parts, " AND ")
}

func Collect(ctx context.Context, db *gorm.DB) ([]Ref, error) {
	branches := make([]string, 0, len(specs))
	for _, s := range specs {
		branches = append(branches, fmt.Sprintf(
			"SELECT t.%s AS hash, '%s' AS kind, t.%s AS entity_id, '' AS label FROM %s t WHERE %s",
			s.HashColumn, s.Kind, s.EntityColumn, s.Table, s.where()))
	}
	var refs []Ref
	if err := db.WithContext(ctx).Raw(strings.Join(branches, "\nUNION ALL\n")).Scan(&refs).Error; err != nil {
		return nil, err
	}
	return refs, nil
}

func CollectByHash(ctx context.Context, db *gorm.DB, hash string) ([]Ref, error) {
	branches := make([]string, 0, len(specs))
	args := make([]any, 0, len(specs))
	for _, s := range specs {
		label := fmt.Sprintf("COALESCE(t.%s, '')", s.LabelColumn)
		join := ""
		if s.LabelTable != "" {
			label = fmt.Sprintf("COALESCE(l.%s, '')", s.LabelColumn)
			join = fmt.Sprintf(" LEFT JOIN %s l ON %s", s.LabelTable, s.LabelJoinOn)
		}
		branches = append(branches, fmt.Sprintf(
			"SELECT t.%s AS hash, '%s' AS kind, t.%s AS entity_id, %s AS label FROM %s t%s WHERE %s",
			s.HashColumn, s.Kind, s.EntityColumn, label, s.Table, join,
			s.where(fmt.Sprintf("t.%s = ?", s.HashColumn))))
		args = append(args, hash)
	}
	q := strings.Join(branches, "\nUNION ALL\n") + "\nORDER BY kind, entity_id"
	var refs []Ref
	if err := db.WithContext(ctx).Raw(q, args...).Scan(&refs).Error; err != nil {
		return nil, err
	}
	return refs, nil
}

func DistinctHashes(ctx context.Context, db *gorm.DB) ([]string, error) {
	branches := make([]string, 0, len(specs))
	for _, s := range specs {
		branches = append(branches, fmt.Sprintf(
			"SELECT t.%s AS hash FROM %s t WHERE %s", s.HashColumn, s.Table, s.where()))
	}
	q := "SELECT DISTINCT hash FROM (\n" + strings.Join(branches, "\nUNION ALL\n") + "\n) u ORDER BY hash"
	var hashes []string
	if err := db.WithContext(ctx).Raw(q).Scan(&hashes).Error; err != nil {
		return nil, err
	}
	return hashes, nil
}

func Detach(ctx context.Context, db *gorm.DB, hash string) (map[string]int64, error) {
	removed := make(map[string]int64, len(specs))
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, s := range specs {
			// Read the fan-out before the write: after the DELETE the rows that
			// name the affected works are gone, so the same query returns none.
			var works []int64
			if s.WorkIDsSQL != "" {
				if err := tx.Raw(s.WorkIDsSQL, hash).Scan(&works).Error; err != nil {
					return fmt.Errorf("detach %s fan-out: %w", s.Kind, err)
				}
			}
			var q string
			if s.DetachSet == "" {
				q = fmt.Sprintf("DELETE FROM %s AS t WHERE %s", s.Table, s.where(fmt.Sprintf("t.%s = ?", s.HashColumn)))
			} else {
				q = fmt.Sprintf("UPDATE %s AS t SET %s = %s WHERE %s",
					s.Table, s.HashColumn, s.DetachSet,
					s.where(fmt.Sprintf("t.%s = ?", s.HashColumn)))
			}
			res := tx.Exec(q, hash)
			if res.Error != nil {
				return fmt.Errorf("detach %s: %w", s.Kind, res.Error)
			}
			removed[s.Kind] = res.RowsAffected
			if res.RowsAffected == 0 {
				continue
			}
			if err := repository.TouchWorks(ctx, tx, works); err != nil {
				return fmt.Errorf("detach %s touch: %w", s.Kind, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return removed, nil
}
