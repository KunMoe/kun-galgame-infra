package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"api/internal/infrastructure/database"
	"api/internal/platform/artifact/model"
	"api/internal/platform/artifact/repository"
	"api/internal/platform/artifact/storage"
	"api/internal/platform/settings/keys"
	"api/pkg/config"
)

type ArtifactGCOpts struct {
	OrphanTTL     time.Duration
	SoftDeleteTTL time.Duration
	MaxPerRun     int
	DryRun        bool
}

func DefaultArtifactGCOpts() ArtifactGCOpts {
	return ArtifactGCOpts{
		OrphanTTL:     time.Duration(keys.ArtifactOrphanTTLHours.Get()) * time.Hour,
		SoftDeleteTTL: time.Duration(keys.ArtifactSoftDeleteTTLHours.Get()) * time.Hour,
		MaxPerRun:     int(keys.ArtifactGCMaxPerRun.Get()),
	}
}

func RunArtifactGC(ctx context.Context, cfg *config.Config, opts ArtifactGCOpts) (Summary, error) {
	if cfg.ArtifactS3.AccessKeyID == "" {
		return Summary{"skipped": "artifact storage not configured"}, nil
	}

	if opts.MaxPerRun <= 0 {
		opts.MaxPerRun = int(keys.ArtifactGCMaxPerRun.Get())
	}

	db, err := database.NewPostgresDB(cfg.ArtifactsDatabase)
	if err != nil {
		return nil, fmt.Errorf("db connect: %w", err)
	}
	defer db.Close()

	store, err := storage.NewClient(cfg.ArtifactCleanupS3())
	if err != nil {
		return nil, fmt.Errorf("s3 init: %w", err)
	}

	repo := repository.NewArtifactRepository(db.DB())
	now := time.Now()

	var orphans, softDeleted, errCount int

	orphanRows, err := repo.FindOrphans(ctx, now.Add(-opts.OrphanTTL), opts.MaxPerRun)
	if err != nil {
		return nil, fmt.Errorf("find orphans: %w", err)
	}
	for i := range orphanRows {
		if err := ctx.Err(); err != nil {
			return partialSummary(orphans, softDeleted, errCount, opts.DryRun), err
		}
		a := &orphanRows[i]
		if opts.DryRun {
			orphans++
			continue
		}
		if a.UploadID != "" {
			if err := store.AbortMultipart(ctx, a.FileKey, a.UploadID); err != nil {
				slog.Warn("artifact-gc: abort multipart", "uuid", a.UUID, "err", err)
			}
		}
		if err := store.Delete(ctx, a.FileKey); err != nil {
			slog.Warn("artifact-gc: delete orphan object", "uuid", a.UUID, "err", err)
		}
		if err := repo.HardDelete(ctx, a.ID); err != nil {
			slog.Error("artifact-gc: drop orphan row", "uuid", a.UUID, "err", err)
			errCount++
			continue
		}
		orphans++
	}

	delRows, err := repo.FindExpiredSoftDeleted(ctx, now.Add(-opts.SoftDeleteTTL), opts.MaxPerRun)
	if err != nil {
		return nil, fmt.Errorf("find soft-deleted: %w", err)
	}
	for i := range delRows {
		if err := ctx.Err(); err != nil {
			return partialSummary(orphans, softDeleted, errCount, opts.DryRun), err
		}
		a := &delRows[i]
		if opts.DryRun {
			softDeleted++
			continue
		}
		if a.UploadID != "" && a.Status == model.StatusUploading {
			if err := store.AbortMultipart(ctx, a.FileKey, a.UploadID); err != nil {
				slog.Warn("artifact-gc: abort multipart (soft-deleted)", "uuid", a.UUID, "err", err)
			}
		}
		if err := store.Delete(ctx, a.FileKey); err != nil {
			slog.Warn("artifact-gc: delete soft-deleted object", "uuid", a.UUID, "err", err)
		}
		if err := repo.HardDelete(ctx, a.ID); err != nil {
			slog.Error("artifact-gc: drop soft-deleted row", "uuid", a.UUID, "err", err)
			errCount++
			continue
		}
		softDeleted++
	}

	return partialSummary(orphans, softDeleted, errCount, opts.DryRun), nil
}

func partialSummary(orphans, softDeleted, errCount int, dryRun bool) Summary {
	return Summary{
		"orphans_reclaimed":   orphans,
		"soft_deleted_purged": softDeleted,
		"errors":              errCount,
		"dry_run":             dryRun,
	}
}
