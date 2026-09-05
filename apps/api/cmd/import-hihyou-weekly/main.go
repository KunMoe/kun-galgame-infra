// import-hihyou-weekly segments the harvested Gal周报 corpus into individual
// news items and, with --apply, writes them into kun_news as status=pending.
// -publish-actor releases the source's pending rows at the end of the run,
// per the 2026-09-05 standing adjudication (bilibili already moderates the
// column); run it as the uid making that call.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"api/internal/jobs/hihyou"
	"api/pkg/config"
	"api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	dir := flag.String("dir", "", "corpus directory written by harvest-hihyou")
	apply := flag.Bool("apply", false, "write to kun_news (default: dry-run forecast only)")
	segmentOnly := flag.Bool("segment-only", false, "report the segmentation without opening the database")
	noImages := flag.Bool("no-images", false, "skip picture download/upload entirely (text rows only)")
	only := flag.String("only", "", "comma-separated issue numbers to process")
	dsn := flag.String("dsn", "", "kun_news DSN override (default: KUN_NEWS_PG_*)")
	imageBase := flag.String("image-base", "", "image_service base URL override (local dev)")
	concurrency := flag.Int("concurrency", 8, "parallel picture fetch/upload (database writes stay serial)")
	publishActor := flag.Int64("publish-actor", 0, "uid recorded on release decisions; publishes the source's pending rows after an -apply run (0 = leave pending)")
	flag.Parse()

	_ = godotenv.Load("apps/api/.env")
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}
	logger.Init(cfg.Server.Env)

	var issues []int
	for _, s := range strings.Split(*only, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			slog.Error("import-hihyou-weekly: --only wants issue numbers", "value", s)
			os.Exit(1)
		}
		issues = append(issues, n)
	}

	sum, err := hihyou.Import(context.Background(), cfg, hihyou.Opts{
		Dir:          *dir,
		Apply:        *apply,
		SegmentOnly:  *segmentOnly,
		NoImages:     *noImages,
		Only:         issues,
		DSN:          *dsn,
		ImageBase:    *imageBase,
		Concurrency:  *concurrency,
		PublishActor: *publishActor,
	})
	if sum != nil {
		b, _ := json.MarshalIndent(sum, "", "  ")
		os.Stdout.Write(append(b, '\n'))
	}
	if err != nil {
		slog.Error("import-hihyou-weekly", "error", err)
		os.Exit(1)
	}
	if sum != nil && sum.ImagesFailed > 0 {
		os.Exit(1)
	}
}
