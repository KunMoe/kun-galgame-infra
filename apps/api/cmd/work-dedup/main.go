package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api/internal/infrastructure/database"
	"api/internal/platform/catalog/repository"
	"api/internal/platform/catalog/service"
	"api/pkg/config"
	"api/pkg/logger"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const waveTagW1 = "rule:work-dedup w1"

// exitNewPairs is watch's -fail-on-new exit code, distinct from 1 (error) so
// the cron wrapper can tell "the detector found work" from "the detector broke".
const exitNewPairs = 3

func main() {
	mode := flag.String("mode", "census", "census | seed | propose | execute | watch")
	actor := flag.Int64("actor", 0, "operator user id recorded on candidates/proposals (required for seed/propose/execute)")
	run := flag.Bool("run", false, "write (default: dry-run preview)")
	limit := flag.Int("limit", 0, "propose: max merge groups this run; execute: max proposals this run (0 = all)")
	note := flag.String("note", waveTagW1, "wave note tag stamped on proposals and matched by -mode execute")
	csvPath := flag.String("csv", "", "census: also export the full pair dossier to this CSV path")
	dsn := flag.String("dsn", "", "catalog DSN override (default: KUN_CATALOG_PG_* env)")
	failOnNew := flag.Bool("fail-on-new", false, fmt.Sprintf("watch: exit %d when undecided new pairs exist", exitNewPairs))
	flag.Parse()

	writes := *mode == "seed" || *mode == "propose" || *mode == "execute"
	if writes && *actor <= 0 {
		fmt.Fprintln(os.Stderr, "-actor <user-id> is required for seed/propose/execute")
		os.Exit(2)
	}

	db, err := openDB(*dsn)
	if err != nil {
		slog.Error("catalog db connect", "error", err)
		os.Exit(1)
	}
	ctx := context.Background()
	resolve := service.NewResolveService(repository.NewRedirectRepository(db))
	merge := service.NewMergeService(db, resolve,
		repository.NewProposalRepository(db), repository.NewRevisionRepository(db))

	switch *mode {
	case "census":
		err = runCensus(ctx, db, os.Stdout, *csvPath)
	case "seed":
		err = runSeed(ctx, db, os.Stdout, *actor, *run)
	case "propose":
		err = runPropose(ctx, db, os.Stdout, merge, *actor, *note, *limit, *run)
	case "execute":
		err = runExecute(ctx, db, os.Stdout, merge, resolve, *actor, *note, *limit, *run)
	case "watch":
		var fresh int
		fresh, err = runWatch(ctx, db, os.Stdout)
		if err == nil && *failOnNew && fresh > 0 {
			os.Exit(exitNewPairs)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", *mode)
		os.Exit(2)
	}
	if err != nil {
		slog.Error("work-dedup failed", "mode", *mode, "error", err)
		os.Exit(1)
	}
}

func openDB(dsn string) (*gorm.DB, error) {
	if dsn != "" {
		logger.Init("development")
		return database.OpenJob(dsn)
	}
	_ = godotenv.Load("apps/api/.env")
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	logger.Init(cfg.Server.Env)
	catalogDB, err := database.NewPostgresDB(cfg.CatalogDatabase)
	if err != nil {
		return nil, err
	}
	return catalogDB.DB(), nil
}
