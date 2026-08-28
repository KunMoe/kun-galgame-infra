package workplaytime

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"

	"api/internal/infrastructure/database"
	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

const capMinutes = 1000 * 60

type Opts struct {
	Apply   bool
	DSN     string
	EGDSN   string
	HltbDSN string
	Source  string
}

type Stats struct {
	EGAnchored  int
	EGPlanned   int
	EGRejected  int
	EGWritten   int
	EGUnchanged int

	VndbPlanned   int
	VndbRejected  int
	VndbWritten   int
	VndbUnchanged int

	HltbAnchored  int
	HltbPlanned   int
	HltbRejected  int
	HltbWritten   int
	HltbUnchanged int

	Errors int
}

func Run(ctx context.Context, opts Opts) (*Stats, error) {
	if opts.DSN == "" {
		return nil, fmt.Errorf("catalog DSN is required (--dsn)")
	}
	if opts.Source == "" {
		opts.Source = "all"
	}
	if (opts.Source == "eg" || opts.Source == "all") && opts.EGDSN == "" {
		return nil, fmt.Errorf("EG mirror DSN is required for the eg lane (--eg-dsn)")
	}
	if opts.Source == "hltb" && opts.HltbDSN == "" {
		return nil, fmt.Errorf("HLTB mirror DSN is required for the hltb lane (--hltb-dsn)")
	}
	db, err := openGorm(opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("connect catalog db: %w", err)
	}
	defer closeGorm(db)

	ids, err := resolveIDs(ctx, db)
	if err != nil {
		return nil, err
	}

	st := &Stats{}
	if opts.Source == "eg" || opts.Source == "all" {
		if err := runEG(ctx, db, opts, ids, st); err != nil {
			return nil, err
		}
	}
	if opts.Source == "vndb" || opts.Source == "all" {
		if err := runVndb(ctx, db, opts, ids, st); err != nil {
			return nil, err
		}
	}
	if opts.Source == "hltb" || (opts.Source == "all" && opts.HltbDSN != "") {
		if err := runHltb(ctx, db, opts, ids, st); err != nil {
			return nil, err
		}
	} else if opts.Source == "all" {
		slog.Warn("hltb lane SKIPPED: --hltb-dsn not set")
	}
	slog.Info("workplaytime done", "apply", opts.Apply,
		"eg_anchored", st.EGAnchored, "eg_planned", st.EGPlanned, "eg_rejected", st.EGRejected,
		"eg_written", st.EGWritten, "eg_unchanged", st.EGUnchanged,
		"vndb_planned", st.VndbPlanned, "vndb_rejected", st.VndbRejected,
		"vndb_written", st.VndbWritten, "vndb_unchanged", st.VndbUnchanged,
		"hltb_anchored", st.HltbAnchored, "hltb_planned", st.HltbPlanned, "hltb_rejected", st.HltbRejected,
		"hltb_written", st.HltbWritten, "hltb_unchanged", st.HltbUnchanged, "errors", st.Errors)
	return st, nil
}

type registryIDs struct {
	galgameMedium int16
	egSource      int16
	vndbSource    int16
	hltbSource    int16
}

func resolveIDs(ctx context.Context, db *gorm.DB) (registryIDs, error) {
	var r registryIDs
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_medium WHERE key = 'galgame'`).Scan(&r.galgameMedium).Error; err != nil {
		return r, fmt.Errorf("resolve galgame medium: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'erogamescape'`).Scan(&r.egSource).Error; err != nil {
		return r, fmt.Errorf("resolve erogamescape source: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'vndb'`).Scan(&r.vndbSource).Error; err != nil {
		return r, fmt.Errorf("resolve vndb source: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'howlongtobeat'`).Scan(&r.hltbSource).Error; err != nil {
		return r, fmt.Errorf("resolve howlongtobeat source: %w", err)
	}
	if r.galgameMedium == 0 || r.egSource == 0 || r.vndbSource == 0 || r.hltbSource == 0 {
		return r, fmt.Errorf("registry not seeded (medium=%d eg=%d vndb=%d howlongtobeat=%d)",
			r.galgameMedium, r.egSource, r.vndbSource, r.hltbSource)
	}
	return r, nil
}

var numRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

func runEG(ctx context.Context, db *gorm.DB, opts Opts, ids registryIDs, st *Stats) error {
	anchors, err := loadAnchors(ctx, db, ids.galgameMedium, ids.egSource)
	if err != nil {
		return fmt.Errorf("load eg anchors: %w", err)
	}
	egDB, err := openGorm(opts.EGDSN)
	if err != nil {
		return fmt.Errorf("connect eg mirror: %w", err)
	}
	defer closeGorm(egDB)

	gameIDs := make([]int64, 0, len(anchors))
	for _, a := range anchors {
		if n, err := strconv.ParseInt(a.ExternalID, 10, 64); err == nil {
			gameIDs = append(gameIDs, n)
		}
	}
	playtime := map[int64]float64{}
	for _, chunk := range chunkInt64(gameIDs, 10000) {
		var rows []struct {
			ID int64  `gorm:"column:id"`
			PT string `gorm:"column:pt"`
		}
		if err := egDB.WithContext(ctx).
			Raw(`SELECT id, raw->>'total_play_time_median' AS pt FROM games
				WHERE id IN ? AND raw->>'total_play_time_median' IS NOT NULL`, chunk).
			Scan(&rows).Error; err != nil {
			return fmt.Errorf("load eg playtimes: %w", err)
		}
		for _, r := range rows {
			if !numRe.MatchString(r.PT) {
				continue
			}
			if h, err := strconv.ParseFloat(r.PT, 64); err == nil {
				playtime[r.ID] = h
			}
		}
	}

	var touched []int64
	for _, a := range anchors {
		n, err := strconv.ParseInt(a.ExternalID, 10, 64)
		if err != nil {
			continue
		}
		h, ok := playtime[n]
		if !ok {
			continue
		}
		st.EGAnchored++
		minutes := int(math.Round(h * 60))
		if minutes <= 0 {
			continue
		}
		if minutes > capMinutes {
			st.EGRejected++
			continue
		}
		st.EGPlanned++
		if !opts.Apply {
			continue
		}
		if upsert(ctx, db, a.WorkID, ids.egSource, minutes, 0, &st.EGWritten, &st.EGUnchanged, &st.Errors) {
			touched = append(touched, a.WorkID)
		}
	}
	return repository.TouchWorks(ctx, db, touched)
}

func runVndb(ctx context.Context, db *gorm.DB, opts Opts, ids registryIDs, st *Stats) error {
	var rows []struct {
		WorkID    int64 `gorm:"column:work_id"`
		Minutes   int   `gorm:"column:minutes"`
		VoteCount int   `gorm:"column:vote_count"`
	}
	if err := db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (w.id) w.id AS work_id, v.c_length AS minutes, v.c_lengthnum AS vote_count
		FROM catalog_work w
		JOIN catalog_external_ref r ON r.entity_type = 5 AND r.entity_id = w.id
			AND r.source_id = ? AND r.link_kind = 0
		JOIN src_vndb.vn v ON v.id = r.external_id
		WHERE w.medium_id = ? AND w.deleted_at IS NULL AND v.c_length IS NOT NULL
		ORDER BY w.id, r.external_id`, ids.vndbSource, ids.galgameMedium).
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load vndb playtimes: %w", err)
	}
	var touched []int64
	for _, r := range rows {
		if r.Minutes <= 0 {
			continue
		}
		if r.Minutes > capMinutes {
			st.VndbRejected++
			continue
		}
		st.VndbPlanned++
		if !opts.Apply {
			continue
		}
		if upsert(ctx, db, r.WorkID, ids.vndbSource, r.Minutes, r.VoteCount, &st.VndbWritten, &st.VndbUnchanged, &st.Errors) {
			touched = append(touched, r.WorkID)
		}
	}
	return repository.TouchWorks(ctx, db, touched)
}

type anchor struct {
	WorkID     int64  `gorm:"column:work_id"`
	ExternalID string `gorm:"column:external_id"`
}

func loadAnchors(ctx context.Context, db *gorm.DB, medium, source int16) ([]anchor, error) {
	var out []anchor
	if err := db.WithContext(ctx).Raw(`
		SELECT DISTINCT ON (w.id) w.id AS work_id, r.external_id
		FROM catalog_work w
		JOIN catalog_external_ref r ON r.entity_type = 5 AND r.entity_id = w.id
			AND r.source_id = ? AND r.link_kind = 0
		WHERE w.medium_id = ? AND w.deleted_at IS NULL
		ORDER BY w.id, r.external_id`, source, medium).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func upsert(ctx context.Context, db *gorm.DB, workID int64, sourceID int16, minutes, votes int, written, unchanged, errors *int) bool {
	res := db.WithContext(ctx).Exec(`
		INSERT INTO catalog_work_playtime (work_id, source_id, minutes, vote_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (work_id, source_id) DO UPDATE
			SET minutes = EXCLUDED.minutes, vote_count = EXCLUDED.vote_count, updated_at = now()
			WHERE (catalog_work_playtime.minutes, catalog_work_playtime.vote_count)
				IS DISTINCT FROM (EXCLUDED.minutes, EXCLUDED.vote_count)`,
		workID, sourceID, minutes, votes)
	if res.Error != nil {
		*errors++
		slog.Warn("playtime upsert", "work", workID, "source", sourceID, "err", res.Error)
		return false
	}
	if res.RowsAffected == 1 {
		*written++
		return true
	}
	*unchanged++
	return false
}

func chunkInt64(in []int64, size int) [][]int64 {
	var out [][]int64
	for len(in) > size {
		out = append(out, in[:size])
		in = in[size:]
	}
	if len(in) > 0 {
		out = append(out, in)
	}
	return out
}

func openGorm(dsn string) (*gorm.DB, error) {
	return database.OpenJob(dsn)
}

func closeGorm(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}
