package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"api/internal/infrastructure/database"
	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

const sourceKeyVndb = "vndb"

const defaultMinMirrorRows int64 = 50_000

type stats struct {
	Anchors     int64
	MirrorRows  int64
	MirrorFresh *time.Time
	ToMark      int64
	ToClear     int64
	Marked      int64
	Cleared     int64
}

func run(ctx context.Context, dsn string, apply bool, minMirrorRows int64, w io.Writer) error {
	db, err := database.OpenJob(dsn)
	if err != nil {
		return fmt.Errorf("connect catalog db: %w", err)
	}
	if sqlDB, e := db.DB(); e == nil {
		defer sqlDB.Close()
	}
	st, err := audit(ctx, db, apply, minMirrorRows)
	if err != nil {
		return err
	}
	report(w, st, apply)
	return nil
}

func audit(ctx context.Context, db *gorm.DB, apply bool, minMirrorRows int64) (stats, error) {
	var st stats
	db = db.WithContext(ctx)

	var srcID int16
	if err := db.Raw(`SELECT id FROM catalog_source WHERE key = ?`, sourceKeyVndb).Scan(&srcID).Error; err != nil {
		return st, fmt.Errorf("look up vndb source id: %w", err)
	}
	if srcID == 0 {
		return st, fmt.Errorf("catalog_source has no %q row", sourceKeyVndb)
	}

	if err := db.Raw(`SELECT count(*) FROM src_vndb.vn`).Scan(&st.MirrorRows).Error; err != nil {
		return st, fmt.Errorf("count src_vndb.vn (is the VNDB mirror loaded?): %w", err)
	}
	if err := db.Raw(`SELECT max(ingested_at) FROM src_vndb.vn`).Scan(&st.MirrorFresh).Error; err != nil {
		return st, fmt.Errorf("read src_vndb.vn ingested_at: %w", err)
	}

	if err := db.Raw(`SELECT count(*) FROM catalog_external_ref
		WHERE source_id = ? AND entity_type = ? AND link_kind = ?`,
		srcID, model.EntityTypeWork, model.LinkKindExact).Scan(&st.Anchors).Error; err != nil {
		return st, fmt.Errorf("count anchors: %w", err)
	}

	const markPredicate = `source_id = ? AND entity_type = ? AND link_kind = ? AND dead_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM src_vndb.vn v WHERE v.id = catalog_external_ref.external_id)`
	const clearPredicate = `source_id = ? AND entity_type = ? AND link_kind = ? AND dead_at IS NOT NULL
		AND EXISTS (SELECT 1 FROM src_vndb.vn v WHERE v.id = catalog_external_ref.external_id)`
	args := []any{srcID, model.EntityTypeWork, model.LinkKindExact}

	if err := db.Raw(`SELECT count(*) FROM catalog_external_ref WHERE `+markPredicate, args...).
		Scan(&st.ToMark).Error; err != nil {
		return st, fmt.Errorf("count anchors absent upstream: %w", err)
	}
	if err := db.Raw(`SELECT count(*) FROM catalog_external_ref WHERE `+clearPredicate, args...).
		Scan(&st.ToClear).Error; err != nil {
		return st, fmt.Errorf("count revived anchors: %w", err)
	}
	if !apply {
		return st, nil
	}
	if st.MirrorRows < minMirrorRows {
		return st, fmt.Errorf(
			"refusing to apply: src_vndb.vn holds %d rows, below the --min-mirror-rows floor of %d — "+
				"the mirror looks partially loaded, and applying against it would mark live anchors dead wholesale",
			st.MirrorRows, minMirrorRows)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		var marked []int64
		if err := tx.Raw(`UPDATE catalog_external_ref SET dead_at = now() WHERE `+markPredicate+
			` RETURNING entity_id`, args...).Scan(&marked).Error; err != nil {
			return fmt.Errorf("mark dead: %w", err)
		}
		st.Marked = int64(len(marked))
		var cleared []int64
		if err := tx.Raw(`UPDATE catalog_external_ref SET dead_at = NULL WHERE `+clearPredicate+
			` RETURNING entity_id`, args...).Scan(&cleared).Error; err != nil {
			return fmt.Errorf("clear revived: %w", err)
		}
		st.Cleared = int64(len(cleared))
		return repository.TouchWorks(ctx, tx, append(marked, cleared...))
	})
	return st, err
}

func report(w io.Writer, st stats, apply bool) {
	fresh := "unknown (mirror empty)"
	if st.MirrorFresh != nil {
		fresh = st.MirrorFresh.Format(time.RFC3339)
	}
	fmt.Fprintf(w, "[audit-vndb-anchors] mirror src_vndb.vn rows=%d ingested_at=%s\n", st.MirrorRows, fresh)
	fmt.Fprintf(w, "[audit-vndb-anchors] work-level exact vndb anchors=%d\n", st.Anchors)
	if apply {
		fmt.Fprintf(w, "[audit-vndb-anchors] marked dead=%d cleared (revived)=%d\n", st.Marked, st.Cleared)
		return
	}
	fmt.Fprintf(w, "[audit-vndb-anchors] would mark dead=%d would clear (revived)=%d (dry run — pass --apply to write)\n",
		st.ToMark, st.ToClear)
}
