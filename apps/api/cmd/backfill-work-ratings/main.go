package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"api/internal/jobs/workratings"
	"api/pkg/config"
	"api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	apply := flag.Bool("apply", false, "write changes (default: dry run, counters + samples only)")
	dsn := flag.String("dsn", "", "catalog DSN (also hosts src_bangumi) — REQUIRED; the rehearsal copy locally, the live catalog only in the acceptance run")
	egDSN := flag.String("eg-dsn", "", "EG mirror DSN (the erogamescape database) — REQUIRED")
	dlsiteDSN := flag.String("dlsite-dsn", "", "DLsite mirror DSN (the dlsite database) — REQUIRED")
	hltbDSN := flag.String("hltb-dsn", "", "HLTB mirror DSN (the howlongtobeat database); empty skips the hltb lane")
	limit := flag.Int("limit", 0, "max candidate works per lane (0 = all)")
	offset := flag.Int("offset", 0, "skip this many candidate works per lane (for chunking)")
	flag.Parse()

	_ = godotenv.Load("apps/api/.env")

	if cfg, err := config.Load(); err == nil {
		logger.Init(cfg.Server.Env)
	}

	st, err := workratings.Run(context.Background(), workratings.Opts{
		Apply:     *apply,
		DSN:       *dsn,
		EGDSN:     *egDSN,
		DlsiteDSN: *dlsiteDSN,
		HltbDSN:   *hltbDSN,
		Limit:     *limit,
		Offset:    *offset,
	})
	if err != nil {
		slog.Error("backfill-work-ratings", "error", err)
		os.Exit(1)
	}
	slog.Info("backfill-work-ratings summary",
		"apply", *apply,
		"bgm_candidates", st.BgmCandidates,
		"bgm_no_score", st.BgmNoScore,
		"bgm_planned", st.BgmPlanned,
		"bgm_written", st.BgmWritten,
		"bgm_unchanged", st.BgmUnchanged,
		"eg_candidates", st.EgCandidates,
		"eg_multi_anchor", st.EgMultiAnchor,
		"eg_missing_mirror", st.EgMissingMirror,
		"eg_no_median", st.EgNoMedian,
		"eg_planned", st.EgPlanned,
		"eg_written", st.EgWritten,
		"eg_unchanged", st.EgUnchanged,
		"eg_distribution", st.EgDistribution,
		"eg_no_reviews", st.EgNoReviews,
		"dl_candidates", st.DlCandidates,
		"dl_missing_mirror", st.DlMissingMirror,
		"dl_no_rating", st.DlNoRating,
		"dl_rating_planned", st.DlRatingPlanned,
		"dl_rating_written", st.DlRatingWritten,
		"dl_rating_unchanged", st.DlRatingUnchanged,
		"vndb_candidates", st.VndbCandidates,
		"vndb_multi_anchor", st.VndbMultiAnchor,
		"vndb_missing_mirror", st.VndbMissingMirror,
		"vndb_no_score", st.VndbNoScore,
		"vndb_planned", st.VndbPlanned,
		"vndb_written", st.VndbWritten,
		"vndb_unchanged", st.VndbUnchanged,
		"vndb_stats", st.VndbStats,
		"vndb_distribution", st.VndbDistribution,
		"vndb_no_votes", st.VndbNoVotes,
		"pop_planned", st.PopPlanned,
		"pop_written", st.PopWritten,
		"pop_unchanged", st.PopUnchanged,
		"hltb_candidates", st.HltbCandidates,
		"hltb_multi_anchor", st.HltbMultiAnchor,
		"hltb_missing_mirror", st.HltbMissingMirror,
		"hltb_no_score", st.HltbNoScore,
		"hltb_planned", st.HltbPlanned,
		"hltb_written", st.HltbWritten,
		"hltb_unchanged", st.HltbUnchanged,
		"hltb_distribution", st.HltbDistribution,
		"errors", st.Errors,
	)
	if !*apply {
		slog.Info("DRY RUN — nothing written; re-run with --apply")
	}
	if st.Errors > 0 {
		os.Exit(1)
	}
}
