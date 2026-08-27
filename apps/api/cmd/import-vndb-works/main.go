package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api/internal/infrastructure/database"
	"api/internal/platform/catalog/importer"
	"api/pkg/config"
	"api/pkg/logger"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	apply := flag.Bool("apply", false, "write (default: dry run — plan lines and counts only)")
	limit := flag.Int("limit", 0, "cap works minted per wave (0 = all); the cap applies to the DRY plan too")
	dsn := flag.String("dsn", "", "catalog DSN (also hosts src_vndb); default: configured CatalogDatabase — pass the rehearsal copy locally")
	flag.Parse()

	_ = godotenv.Load("apps/api/.env")
	cfg, cfgErr := config.Load()
	if cfgErr == nil {
		logger.Init(cfg.Server.Env)
	}

	var db *gorm.DB
	switch {
	case *dsn != "":
		g, err := database.OpenJob(*dsn)
		if err != nil {
			slog.Error("catalog db connect (--dsn)", "error", err)
			os.Exit(1)
		}
		if sqlDB, err := g.DB(); err == nil {
			defer sqlDB.Close()
		}
		db = g
	case cfgErr == nil:
		catalogDB, err := database.NewPostgresDB(cfg.CatalogDatabase)
		if err != nil {
			slog.Error("catalog db connect", "error", err)
			os.Exit(1)
		}
		defer catalogDB.Close()
		db = catalogDB.DB()
	default:
		slog.Error("no --dsn given and config.Load failed", "error", cfgErr)
		os.Exit(1)
	}

	im := importer.New(db, nil, importer.Options{Source: "vndb", DryRun: !*apply, Limit: *limit})
	st, err := im.RunVNDBWorks()
	if err != nil {
		slog.Error("vndb works import failed", "error", err)
		os.Exit(1)
	}

	if !*apply {
		for _, p := range st.Plans {
			fmt.Printf("plan %s olang=%s r18=%t title=%q en=%q\n", p.VID, p.OLang, p.R18, p.Title, p.ENTitle)
		}
		fmt.Printf("vndb-works plans %d new works\n", st.Planned)
	}
	slog.Info("import-vndb-works summary",
		"source", "vndb",
		"apply", *apply,
		"pool_unanchored", st.PoolUnanchored,
		"planned", st.Planned,
		"planned_r18", st.PlannedR18,
		"works_created", st.WorksCreated,
		"titles_created", st.TitlesCreated,
		"anchors_created", st.AnchorsCreated,
		"revisions_created", st.RevisionsCreated,
		"skipped_cancelled", st.SkippedCancelled,
		"skipped_no_olang_title", st.SkippedNoOLangTitle,
		"skipped_over_limit", st.CappedByLimit,
		"errors", st.Errors,
	)

	if !*apply {
		slog.Info("DRY RUN — nothing written; re-run with --apply")
	}
	if st.Errors > 0 {
		os.Exit(1)
	}
}
