package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"api/internal/infrastructure/database"
	"api/internal/jobs/storestats"
	"api/internal/platform/store/shortener"
	"api/pkg/config"
)

// RunStoreStatsSync soft-skips an unconfigured deployment rather than failing
// hourly before the shortener exists, the same call the image refping jobs make.
func RunStoreStatsSync(ctx context.Context, cfg *config.Config, opts storestats.Opts) (Summary, error) {
	if cfg.Store.ShortlinkBaseURL == "" || cfg.Store.ShortlinkAPIKey == "" {
		slog.Warn("store-stats-sync: link shortener not configured — skipping (set KUN_STORE_SHORTLINK_BASE_URL/API_KEY)")
		return Summary{"skipped": "store shortener not configured"}, nil
	}

	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	defer func() { _ = db.Close() }()

	res, err := storestats.Run(ctx, db.DB(), shortener.New(cfg.Store.ShortlinkBaseURL, cfg.Store.ShortlinkAPIKey), opts)
	if err != nil {
		return nil, err
	}
	return Summary{
		"aliases": res.Aliases, "calls": res.Calls, "rows": res.Rows,
		"from": res.From, "to": res.To, "full_pull": res.FullPull,
	}, nil
}
