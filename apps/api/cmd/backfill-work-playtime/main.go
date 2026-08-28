package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"api/internal/jobs/workplaytime"
	"api/pkg/logger"
)

func main() {
	dsn := flag.String("dsn", "", "catalog DSN (REQUIRED; hosts src_vndb)")
	egDSN := flag.String("eg-dsn", "", "EG mirror DSN (REQUIRED for eg/all)")
	hltbDSN := flag.String("hltb-dsn", "", "HLTB mirror DSN (REQUIRED for hltb; empty skips the hltb lane under all)")
	source := flag.String("source", "all", "lane: eg | vndb | hltb | all")
	apply := flag.Bool("apply", false, "write changes (default dry)")
	flag.Parse()

	logger.Init("development")
	st, err := workplaytime.Run(context.Background(), workplaytime.Opts{
		Apply: *apply, DSN: *dsn, EGDSN: *egDSN, HltbDSN: *hltbDSN, Source: *source,
	})
	if err != nil {
		slog.Error("backfill-work-playtime failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("\n=== backfill-work-playtime %s ===\n", mode(*apply))
	fmt.Printf("eg: anchored=%d planned=%d rejected=%d written=%d unchanged=%d\n",
		st.EGAnchored, st.EGPlanned, st.EGRejected, st.EGWritten, st.EGUnchanged)
	fmt.Printf("vndb: planned=%d rejected=%d written=%d unchanged=%d\n",
		st.VndbPlanned, st.VndbRejected, st.VndbWritten, st.VndbUnchanged)
	fmt.Printf("hltb: anchored=%d planned=%d rejected=%d written=%d unchanged=%d\n",
		st.HltbAnchored, st.HltbPlanned, st.HltbRejected, st.HltbWritten, st.HltbUnchanged)
	fmt.Printf("errors=%d\n", st.Errors)
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
