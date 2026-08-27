package vndbcovers

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"time"

	"api/internal/platform/catalog/model"
	"api/pkg/imageclient"
	"api/pkg/imageshrink"

	"gorm.io/gorm/clause"
)

const (
	coverPreset = "catalog_cover"

	uploaderSub = "system:vndb-cover-backfill"

	coverKind = "main"

	uploadRetries = 6

	downloadRetries = 3

	maxImageBytes = 16 << 20

	downloadTimeout = 60 * time.Second

	defaultCoverFilename = "cover.jpg"
)

func (r *runner) fill(ctx context.Context, row planRow) {
	body, filename, err := r.download(ctx, row.Img.URL)
	if err != nil {
		r.stats.Errors++
		slog.Warn("download vndb cover", "work", row.WorkID, "vn", row.VNDBID, "url", row.Img.URL, "err", err)
		return
	}
	body, filename, err = imageshrink.Shrink(body, filename)
	if err != nil {
		r.stats.Errors++
		slog.Warn("shrink vndb cover", "work", row.WorkID, "vn", row.VNDBID, "err", err)
		return
	}
	res, err := r.upload(ctx, body, filename)
	if err != nil {
		if r.classify(err, row) {
			r.stats.Quota = true
		}
		return
	}
	tx := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "work_id"}, {Name: "image_hash"}},
		DoNothing: true,
	}).Create(&model.CatalogWorkCover{
		WorkID: row.WorkID, ImageHash: res.Hash, SortOrder: 0, Kind: coverKind,
		PortraitPinned: portrait(row.Img.Dims),
		Sexual:         ratingLevel(row.Img.Sexual),
		Violence:       ratingLevel(row.Img.Violence),
		SourceID:       r.sourceID,
	})
	if tx.Error != nil {
		r.stats.Errors++
		slog.Warn("write vndb cover row", "work", row.WorkID, "vn", row.VNDBID, "err", tx.Error)
		return
	}
	r.pingHashes = append(r.pingHashes, res.Hash)
	if tx.RowsAffected == 0 {
		r.stats.Dedup++
		return
	}
	r.touched = append(r.touched, row.WorkID)
	r.stats.Uploaded++
}

func (r *runner) download(ctx context.Context, src string) ([]byte, string, error) {
	client := &http.Client{Timeout: downloadTimeout}
	var lastErr error
	for attempt := 0; attempt < downloadRetries; attempt++ {
		if attempt > 0 && !sleepCtx(ctx, time.Duration(5<<attempt)*time.Second) {
			return nil, "", ctx.Err()
		}
		body, err := fetch(ctx, client, src)
		if err == nil {
			return body, coverFilename(src), nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
	}
	return nil, "", lastErr
}

func fetch(ctx context.Context, client *http.Client, src string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image cdn %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("image cdn returned an empty body")
	}
	if len(body) > maxImageBytes {
		return nil, fmt.Errorf("image exceeds %d bytes", maxImageBytes)
	}
	return body, nil
}

func coverFilename(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return defaultCoverFilename
	}
	base := path.Base(u.Path)
	if base == "." || base == "/" || base == "" || path.Ext(base) == "" {
		return defaultCoverFilename
	}
	return base
}

func (r *runner) upload(ctx context.Context, body []byte, filename string) (*imageclient.UploadResult, error) {
	var lastErr error
	for attempt := 0; attempt < uploadRetries; attempt++ {
		if r.gap > 0 && !sleepCtx(ctx, r.gap) {
			return nil, ctx.Err()
		}
		res, err := r.cli.UploadWithSub(ctx, bytes.NewReader(body), filename, coverPreset, uploaderSub)
		if err == nil {
			return res, nil
		}
		if stderrors.Is(err, imageclient.ErrQuotaExceeded) || stderrors.Is(err, imageclient.ErrModerationRejected) {
			return nil, err
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt < uploadRetries-1 && !sleepCtx(ctx, time.Duration(min(5<<attempt, 30))*time.Second) {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func (r *runner) classify(err error, row planRow) (quota bool) {
	switch {
	case stderrors.Is(err, imageclient.ErrQuotaExceeded):
		slog.Warn("daily image quota exhausted — stopping", "work", row.WorkID)
		return true
	case stderrors.Is(err, imageclient.ErrModerationRejected):
		r.stats.Rejected++
		slog.Warn("vndb cover rejected by moderation", "work", row.WorkID, "vn", row.VNDBID)
		return false
	default:
		r.stats.Errors++
		slog.Warn("upload vndb cover", "work", row.WorkID, "vn", row.VNDBID, "err", err)
		return false
	}
}
