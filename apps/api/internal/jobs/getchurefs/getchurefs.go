package getchurefs

import (
	"context"
	"fmt"
	"log/slog"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const matchedBy = "rule:vndb-extlink-getchu"

const site = "getchu"

type Opts struct {
	Apply bool
	Limit int
}

type Stats struct {
	Candidates int
	Existing   int
	Planned    int
	Written    int
	Conflict   int
	Errors     int
}

type candidate struct {
	ReleaseID int64  `gorm:"column:release_id"`
	GetchuID  string `gorm:"column:getchu_id"`
}

func Run(ctx context.Context, db *gorm.DB, opts Opts) (*Stats, error) {
	var getchuSource int16
	if err := db.WithContext(ctx).
		Raw(`SELECT id FROM catalog_source WHERE key = ?`, site).Scan(&getchuSource).Error; err != nil {
		return nil, fmt.Errorf("resolve getchu source: %w", err)
	}
	if getchuSource == 0 {
		return nil, fmt.Errorf("catalog_source has no %q row — run the seed first", site)
	}
	var vndbSource int16
	if err := db.WithContext(ctx).
		Raw(`SELECT id FROM catalog_source WHERE key = 'vndb'`).Scan(&vndbSource).Error; err != nil {
		return nil, fmt.Errorf("resolve vndb source: %w", err)
	}

	cands, err := loadCandidates(ctx, db, vndbSource, getchuSource, opts.Limit)
	if err != nil {
		return nil, fmt.Errorf("load candidates: %w", err)
	}
	st := &Stats{Candidates: len(cands)}
	slog.Info("getchu-refs candidates", "candidates", st.Candidates, "apply", opts.Apply)

	for _, c := range cands {
		st.Planned++
		if !opts.Apply {
			continue
		}
		var wrote bool
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).
				Create(&model.CatalogExternalRef{
					EntityType: model.EntityTypeRelease,
					EntityID:   c.ReleaseID,
					SourceID:   getchuSource,
					ExternalID: c.GetchuID,
					LinkKind:   model.LinkKindExact,
					MatchedBy:  matchedBy,
				})
			if res.Error != nil || res.RowsAffected == 0 {
				return res.Error
			}
			wrote = true
			return repository.TouchReleaseWorks(ctx, tx, []int64{c.ReleaseID})
		})
		switch {
		case err != nil:
			st.Errors++
			slog.Warn("write ref", "release", c.ReleaseID, "getchu", c.GetchuID, "err", err)
		case !wrote:
			st.Conflict++
		default:
			st.Written++
		}
	}
	slog.Info("getchu-refs done", "apply", opts.Apply, "candidates", st.Candidates,
		"existing", st.Existing, "planned", st.Planned, "written", st.Written,
		"conflict", st.Conflict, "errors", st.Errors)
	return st, nil
}

func loadCandidates(ctx context.Context, db *gorm.DB, vndbSource, getchuSource int16, limit int) ([]candidate, error) {
	q := `
		SELECT DISTINCT ON (rel.id) rel.id AS release_id, e.value AS getchu_id
		FROM catalog_release rel
		JOIN catalog_external_ref vr ON vr.entity_type = ? AND vr.entity_id = rel.id
			AND vr.source_id = ? AND vr.link_kind = ?
		JOIN src_vndb.releases_extlinks rx ON rx.id = vr.external_id
		JOIN src_vndb.extlinks e ON e.id = rx.link AND e.site = ?
		WHERE rel.deleted_at IS NULL
		  AND NOT EXISTS (
			SELECT 1 FROM catalog_external_ref g
			WHERE g.entity_type = ? AND g.entity_id = rel.id AND g.source_id = ?)
		ORDER BY rel.id, e.value`
	args := []any{
		model.EntityTypeRelease, vndbSource, model.LinkKindExact, site,
		model.EntityTypeRelease, getchuSource,
	}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	var out []candidate
	if err := db.WithContext(ctx).Raw(q, args...).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
