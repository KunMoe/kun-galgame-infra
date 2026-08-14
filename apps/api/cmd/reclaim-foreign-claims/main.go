package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api/internal/infrastructure/database"
	"api/pkg/logger"

	"gorm.io/gorm"
)

// reclaim-foreign-claims re-claims catalog works that are still claimed by a
// foreign site (default "moyu") back to kungal, driven by the
// galgame_migrations mapping (source_id -> galgame_id) written during the
// moyu→kungal migration. Default is a dry run; --run writes.
//
// Each foreign-claimed LIVE work is classified:
//   - unmapped   : no galgame_migrations row for its product_work_id — there is
//                  no forum galgame to point it at (leave; needs a vndb lookup
//                  or a manual decision).
//   - duplicate  : the target forum gid is already held by a kungal-claimed
//                  work — two catalog rows point at one forum game; merge them
//                  separately (merge-work-dups), do not UPDATE here.
//   - reclaim    : clean — UPDATE site/product_work_id to kungal/<gid>.

type foreignWork struct {
	ID            int64
	DisplayName   string
	ProductWorkID *int64
}

type migrationRow struct {
	SourceID  int64
	GalgameID int64
}

func main() {
	dsn := flag.String("dsn", "", "catalog DSN (REQUIRED; e.g. host=... dbname=kun_catalog)")
	site := flag.String("site", "moyu", "foreign claim site to reconcile")
	run := flag.Bool("run", false, "write (default dry-run)")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "--dsn is required")
		os.Exit(2)
	}
	logger.Init("development")

	db, err := database.OpenJob(*dsn)
	if err != nil {
		slog.Error("catalog db connect", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()

	run_ := reconcile(ctx, db, *site, *run)
	fmt.Printf("\nsummary: reclaim=%d duplicate=%d unmapped=%d (run=%v)\n",
		run_.reclaim, run_.duplicate, run_.unmapped, *run)
}

type stats struct{ reclaim, duplicate, unmapped int }

func reconcile(ctx context.Context, db *gorm.DB, site string, run bool) stats {
	var s stats

	var works []foreignWork
	if err := db.WithContext(ctx).Raw(`
		SELECT id, display_name, product_work_id
		FROM catalog_work
		WHERE deleted_at IS NULL AND site = ?`, site).Scan(&works).Error; err != nil {
		slog.Error("load foreign works", "error", err)
		os.Exit(1)
	}

	var migs []migrationRow
	if err := db.WithContext(ctx).Raw(`
		SELECT source_id, galgame_id FROM galgame_migrations WHERE source_db = ?`, site).
		Scan(&migs).Error; err != nil {
		slog.Error("load galgame_migrations", "error", err)
		os.Exit(1)
	}
	gidBySource := make(map[int64]int64, len(migs))
	for _, m := range migs {
		gidBySource[m.SourceID] = m.GalgameID
	}

	for _, w := range works {
		if w.ProductWorkID == nil {
			s.unmapped++
			fmt.Printf("UNMAPPED  work=%d %q (product_work_id is NULL)\n", w.ID, w.DisplayName)
			continue
		}
		gid, ok := gidBySource[*w.ProductWorkID]
		if !ok {
			s.unmapped++
			fmt.Printf("UNMAPPED  work=%d %q (product_work_id=%d has no galgame_migrations row)\n",
				w.ID, w.DisplayName, *w.ProductWorkID)
			continue
		}

		var dup int64
		if err := db.WithContext(ctx).Raw(`
			SELECT count(*) FROM catalog_work
			WHERE deleted_at IS NULL
			  AND site IN ('kungal','galgame_wiki')
			  AND product_work_id = ?`, gid).Scan(&dup).Error; err != nil {
			slog.Error("duplicate check", "error", err)
			os.Exit(1)
		}
		if dup > 0 {
			s.duplicate++
			fmt.Printf("DUPLICATE work=%d %q -> forum gid %d (already claimed by a kungal work; merge separately)\n",
				w.ID, w.DisplayName, gid)
			continue
		}

		s.reclaim++
		fmt.Printf("RECLAIM   work=%d %q -> site=kungal product_work_id=%d\n", w.ID, w.DisplayName, gid)
		if run {
			if err := db.WithContext(ctx).Exec(`
				UPDATE catalog_work SET site = 'kungal', product_work_id = ?, updated_at = now()
				WHERE id = ?`, gid, w.ID).Error; err != nil {
				slog.Error("reclaim update", "work", w.ID, "error", err)
				os.Exit(1)
			}
		}
	}
	return s
}
