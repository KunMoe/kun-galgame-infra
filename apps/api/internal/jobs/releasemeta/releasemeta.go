package releasemeta

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"api/internal/infrastructure/database"

	"gorm.io/gorm"
)

const maxSamples = 8

const minYear = 1950

type Opts struct {
	Apply     bool
	DSN       string
	DlsiteDSN string
	EGDSN     string
	Limit     int
	Offset    int
}

type DateSample struct {
	WorkID    int64
	ReleaseID int64
	Ext       string
	Y         int16
	M, D      *int16
}

type RatingSample struct {
	WorkID int64
	Source string
	Ext    string
	Rating int16
}

type Stats struct {
	DlDateCandidates      int
	DlDateMissingMirror   int
	DlDateNoRegist        int
	DlDateOutOfRange      int
	DlDatePlanned         int
	DlDateFilled          int
	DlDateSkippedNonEmpty int

	EgDateCandidates      int
	EgDateCovered         int
	EgDateMultiAnchor     int
	EgDateMissingMirror   int
	EgDateBadDate         int
	EgDatePlanned         int
	EgDateFilled          int
	EgDateSkippedNonEmpty int

	BgmDateCandidates      int
	BgmDateCovered         int
	BgmDateNoDate          int
	BgmDateBadDate         int
	BgmDatePartial         int
	BgmDatePlanned         int
	BgmDateFilled          int
	BgmDateSkippedNonEmpty int

	RatingCandidates      int
	RatingVndbR18         int
	RatingDlR18           int
	RatingDlSensitive     int
	RatingDlAllAges       int
	RatingEgR18           int
	RatingBgmR18          int
	RatingNoVerdict       int
	RatingPlanned         int
	RatingFilled          int
	RatingSkippedNonEmpty int
	RatingCuratedOverride int

	Errors int

	DlDateSamples  []DateSample
	EgDateSamples  []DateSample
	BgmDateSamples []DateSample
	RatingSamples  []RatingSample
}

func Run(ctx context.Context, opts Opts) (*Stats, error) {
	if opts.DSN == "" {
		return nil, fmt.Errorf("catalog DSN is required (--dsn); refusing to guess — pass the rehearsal copy locally, the live catalog only in the acceptance run")
	}
	if opts.DlsiteDSN == "" {
		return nil, fmt.Errorf("DLsite mirror DSN is required (--dlsite-dsn); refusing to guess")
	}
	if opts.EGDSN == "" {
		return nil, fmt.Errorf("EG mirror DSN is required (--eg-dsn); refusing to guess")
	}
	db, err := openGorm(opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("connect catalog db: %w", err)
	}
	defer closeDB(db)
	dlDB, err := openGorm(opts.DlsiteDSN)
	if err != nil {
		return nil, fmt.Errorf("connect DLsite mirror db: %w", err)
	}
	defer closeDB(dlDB)
	egDB, err := openGorm(opts.EGDSN)
	if err != nil {
		return nil, fmt.Errorf("connect EG mirror db: %w", err)
	}
	defer closeDB(egDB)

	reg, err := resolveRegistry(ctx, db)
	if err != nil {
		return nil, err
	}
	st := &Stats{}
	w := &writer{db: db, stats: st}
	maxYear := time.Now().Year() + 3
	planned := map[int64]bool{}

	if err := runDlsiteDateLane(ctx, db, dlDB, w, reg, opts, maxYear, planned); err != nil {
		return nil, err
	}
	if err := runEgDateLane(ctx, db, egDB, w, reg, opts, maxYear, planned); err != nil {
		return nil, err
	}
	if err := runBgmDateLane(ctx, db, w, reg, opts, maxYear, planned); err != nil {
		return nil, err
	}
	if err := runRatingLane(ctx, db, dlDB, egDB, w, reg, opts); err != nil {
		return nil, err
	}
	if err := w.touch(ctx); err != nil {
		return nil, fmt.Errorf("touch works: %w", err)
	}

	slog.Info("backfill-release-meta done", "apply", opts.Apply,
		"dl_date_candidates", st.DlDateCandidates, "dl_date_missing_mirror", st.DlDateMissingMirror,
		"dl_date_no_regist", st.DlDateNoRegist, "dl_date_out_of_range", st.DlDateOutOfRange,
		"dl_date_planned", st.DlDatePlanned, "dl_date_filled", st.DlDateFilled,
		"dl_date_skipped_non_empty", st.DlDateSkippedNonEmpty,
		"eg_date_candidates", st.EgDateCandidates, "eg_date_covered", st.EgDateCovered,
		"eg_date_multi_anchor", st.EgDateMultiAnchor, "eg_date_missing_mirror", st.EgDateMissingMirror,
		"eg_date_bad_date", st.EgDateBadDate, "eg_date_planned", st.EgDatePlanned,
		"eg_date_filled", st.EgDateFilled, "eg_date_skipped_non_empty", st.EgDateSkippedNonEmpty,
		"bgm_date_candidates", st.BgmDateCandidates, "bgm_date_covered", st.BgmDateCovered,
		"bgm_date_no_date", st.BgmDateNoDate, "bgm_date_bad_date", st.BgmDateBadDate,
		"bgm_date_partial", st.BgmDatePartial, "bgm_date_planned", st.BgmDatePlanned,
		"bgm_date_filled", st.BgmDateFilled, "bgm_date_skipped_non_empty", st.BgmDateSkippedNonEmpty,
		"rating_candidates", st.RatingCandidates,
		"rating_vndb_r18", st.RatingVndbR18,
		"rating_dl_r18", st.RatingDlR18, "rating_dl_sensitive", st.RatingDlSensitive,
		"rating_dl_all_ages", st.RatingDlAllAges,
		"rating_eg_r18", st.RatingEgR18,
		"rating_bgm_r18", st.RatingBgmR18,
		"rating_no_verdict", st.RatingNoVerdict, "rating_planned", st.RatingPlanned,
		"rating_filled", st.RatingFilled, "rating_skipped_non_empty", st.RatingSkippedNonEmpty,
		"rating_curated_override", st.RatingCuratedOverride,
		"errors", st.Errors)
	logDateSamples("dlsite", st.DlDateSamples)
	logDateSamples("eg", st.EgDateSamples)
	logDateSamples("bgm", st.BgmDateSamples)
	for _, s := range st.RatingSamples {
		slog.Info("backfill-release-meta rating sample",
			"work_id", s.WorkID, "source", s.Source, "ext", s.Ext, "content_rating", s.Rating)
	}
	return st, nil
}

func logDateSamples(lane string, samples []DateSample) {
	for _, s := range samples {
		slog.Info("backfill-release-meta date sample", "lane", lane,
			"work_id", s.WorkID, "release_id", s.ReleaseID, "ext", s.Ext,
			"y", s.Y, "m", derefOr(s.M, 0), "d", derefOr(s.D, 0))
	}
}

func collectDate(dst *[]DateSample, s DateSample) {
	if len(*dst) < maxSamples {
		*dst = append(*dst, s)
	}
}

func collectRating(dst *[]RatingSample, s RatingSample) {
	if len(*dst) < maxSamples {
		*dst = append(*dst, s)
	}
}

func derefOr(p *int16, fallback int16) int16 {
	if p == nil {
		return fallback
	}
	return *p
}

func window[T any](in []T, limit, offset int) []T {
	if offset > 0 {
		if offset >= len(in) {
			return nil
		}
		in = in[offset:]
	}
	if limit > 0 && limit < len(in) {
		in = in[:limit]
	}
	return in
}

func openGorm(dsn string) (*gorm.DB, error) {
	return database.OpenJob(dsn)
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}
