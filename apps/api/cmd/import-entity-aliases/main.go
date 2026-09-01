package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api/internal/infrastructure/database"
	"api/pkg/config"
	"api/pkg/logger"

	"github.com/joho/godotenv"
)

const (
	sourceBangumi int16 = 3
	sourceEG      int16 = 5
)

func main() {
	apply := flag.Bool("run", false, "write changes (default: dry-run preview)")
	hints := flag.Bool("hints", false, "run leg A only (search-hint ingestion)")
	candidates := flag.Bool("candidates", false, "run leg B only (alias_declared candidates)")
	flag.Parse()

	runA, runB := *hints, *candidates
	if !runA && !runB {
		runA, runB = true, true
	}

	_ = godotenv.Load("apps/api/.env")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger.Init(cfg.Server.Env)

	catalogDB, err := database.NewPostgresDB(cfg.CatalogDatabase)
	if err != nil {
		slog.Error("catalog db connect", "error", err)
		os.Exit(1)
	}
	defer catalogDB.Close()
	db := catalogDB.DB()

	if runA {
		if _, err := runHints(db, os.Stdout, *apply); err != nil {
			slog.Error("leg A failed", "error", err)
			os.Exit(1)
		}
	}
	if runB {
		if _, err := runCandidates(db, os.Stdout, *apply); err != nil {
			slog.Error("leg B failed", "error", err)
			os.Exit(1)
		}
	}

	if *apply && runA {
		fmt.Fprintln(os.Stdout, "note: run `go run ./cmd/reindex-catalog` to project the new search hints into OpenSearch.")
	} else if !*apply {
		fmt.Fprintln(os.Stdout, "DRY-RUN — nothing written; re-run with --run.")
	}
}
