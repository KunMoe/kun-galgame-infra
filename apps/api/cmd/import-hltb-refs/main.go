package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api/internal/jobs/hltbrefs"
	"api/pkg/logger"
)

func main() {
	dsn := flag.String("dsn", "", "catalog DSN (REQUIRED)")
	hltbDSN := flag.String("hltb-dsn", "", "HLTB mirror DSN (REQUIRED)")
	apply := flag.Bool("apply", false, "write changes (default dry)")
	flag.Parse()

	logger.Init("development")
	st, err := hltbrefs.Run(context.Background(), hltbrefs.Opts{
		Apply: *apply, DSN: *dsn, HltbDSN: *hltbDSN,
	})
	if err != nil {
		slog.Error("import-hltb-refs failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("\n=== import-hltb-refs %s ===\n", mode(*apply))
	fmt.Printf("steam_anchored=%d mirror_matched=%d ambiguous=%d\n", st.SteamAnchored, st.MirrorMatched, st.AmbiguousSteam)
	fmt.Printf("planned=%d written=%d exists=%d rejected=%d errors=%d\n", st.Planned, st.Written, st.Exists, st.Rejected, st.Errors)
	if st.Errors > 0 {
		os.Exit(1)
	}
}

func mode(apply bool) string {
	if apply {
		return "APPLY"
	}
	return "DRY"
}
