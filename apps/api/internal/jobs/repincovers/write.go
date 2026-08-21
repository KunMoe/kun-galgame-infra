package repincovers

import (
	"bytes"
	"context"
	"encoding/csv"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"api/internal/platform/catalog/model"
	"api/internal/platform/catalog/repository"
	"api/pkg/imageclient"

	"gorm.io/gorm"
)

const (
	coverPreset      = "catalog_cover"
	uploaderSub      = "system:repin-portrait-covers"
	upscaleSourceKey = "upscale"
	uploadRetries    = 6
	downloadTimeout  = 60 * time.Second
)

var badUpscaleKinds = []string{"pkgback", "pkgmed", "pkgcontent", "pkgside"}

func (r *runner) url(hash string) string { return r.cli.MainURL(hash) }

func writePlanCSV(path string, plans []Plan, url func(string) string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	head := []string{"work_id", "action", "old_kind", "old_source", "old_edge", "old_url",
		"new_kind", "new_source", "new_edge", "new_url"}
	if err := w.Write(head); err != nil {
		return err
	}
	for _, p := range plans {
		if p.Action == ActionNone {
			continue
		}
		rec := []string{strconv.FormatInt(p.WorkID, 10), p.Action.String()}
		if p.Old != nil {
			rec = append(rec, p.Old.Kind, p.Old.SourceKey, strconv.Itoa(p.Old.LongEdge()), url(p.Old.Hash))
		} else {
			rec = append(rec, "", "", "", "")
		}
		rec = append(rec, p.New.Kind, p.New.SourceKey, strconv.Itoa(p.New.LongEdge()), url(p.New.Hash))
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return nil
}

func productName(p Plan) string {
	return fmt.Sprintf("%d__%s.webp", p.WorkID, p.New.Hash)
}

func parseProductName(name string) (workID int64, originHash string, ok bool) {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	id, hash, found := strings.Cut(base, "__")
	if !found {
		return 0, "", false
	}
	workID, err := strconv.ParseInt(id, 10, 64)
	if err != nil || hash == "" {
		return 0, "", false
	}
	return workID, hash, true
}

func (r *runner) export(ctx context.Context, opts Opts, plans []Plan) error {
	if err := os.MkdirAll(opts.ExportDir, 0o755); err != nil {
		return err
	}
	todo := actionable(plans, ActionUpscale, opts.Limit)
	client := &http.Client{Timeout: downloadTimeout}
	for _, p := range todo {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		dst := filepath.Join(opts.ExportDir, productName(p))
		if fi, err := os.Stat(dst); err == nil && fi.Size() > 0 {
			continue
		}
		if err := download(ctx, client, r.url(p.New.Hash), dst); err != nil {
			r.stats.Errors++
			slog.Warn("export cover", "work", p.WorkID, "hash", p.New.Hash, "err", err)
			continue
		}
		r.stats.Exported++
	}
	slog.Info("export complete", "dir", opts.ExportDir, "files", r.stats.Exported, "errors", r.stats.Errors)
	return nil
}

func download(ctx context.Context, client *http.Client, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return fmt.Errorf("GET %s: empty body", url)
	}
	tmp := dst + ".part"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func (r *runner) reinject(ctx context.Context, opts Opts, plans []Plan) error {
	if !opts.Apply {
		slog.Warn("reinject is a DRY listing without --apply")
	}
	srcID, err := r.sourceID(ctx, upscaleSourceKey)
	if err != nil {
		return err
	}
	byWork := map[int64]Plan{}
	for _, p := range plans {
		if p.Action == ActionUpscale {
			byWork[p.WorkID] = p
		}
	}
	entries, err := os.ReadDir(opts.ReinjectDir)
	if err != nil {
		return fmt.Errorf("read reinject dir: %w", err)
	}
	acted := 0
	for _, e := range entries {
		if ctx.Err() != nil || r.stats.QuotaBreak {
			break
		}
		if e.IsDir() {
			continue
		}
		workID, originHash, ok := parseProductName(e.Name())
		if !ok {
			continue
		}
		p, planned := byWork[workID]
		if !planned || p.New.Hash != originHash {
			r.stats.Skipped++
			continue
		}
		if opts.Limit > 0 && acted >= opts.Limit {
			r.stats.Skipped++
			continue
		}
		if !opts.Apply {
			acted++
			continue
		}
		if err := r.injectOne(ctx, filepath.Join(opts.ReinjectDir, e.Name()), p, srcID); err != nil {
			r.stats.Errors++
			slog.Warn("reinject cover", "work", workID, "err", err)
			continue
		}
		acted++
	}
	return r.finish(ctx)
}

func (r *runner) injectOne(ctx context.Context, path string, p Plan, srcID int16) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return fmt.Errorf("empty product file")
	}
	res, err := r.upload(ctx, body, filepath.Base(path))
	if err != nil {
		return err
	}
	r.stats.Uploaded++
	r.fresh = append(r.fresh, res.Hash)

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var next int
		if err := tx.Raw(`SELECT COALESCE(MAX(sort_order), -1) + 1 FROM catalog_work_cover WHERE work_id = ?`,
			p.WorkID).Scan(&next).Error; err != nil {
			return err
		}
		row := model.CatalogWorkCover{
			WorkID: p.WorkID, ImageHash: res.Hash, SortOrder: next,
			Kind: p.New.Kind, PortraitPinned: false,
			Sexual: p.New.Sexual, Violence: 0, SourceID: srcID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return r.repin(tx, p.WorkID, row.ID)
	})
}

func (r *runner) directPin(ctx context.Context, opts Opts, plans []Plan) error {
	todo := actionable(plans, ActionDirectPin, opts.Limit)
	for _, p := range todo {
		if ctx.Err() != nil {
			break
		}
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return r.repin(tx, p.WorkID, p.New.ID)
		})
		if err != nil {
			r.stats.Errors++
			slog.Warn("direct pin", "work", p.WorkID, "err", err)
		}
	}
	return r.finish(ctx)
}

func (r *runner) repin(tx *gorm.DB, workID, coverID int64) error {
	if err := tx.Exec(`UPDATE catalog_work_cover SET portrait_pinned = false, updated_at = now()
		WHERE work_id = ? AND portrait_pinned AND id <> ?`, workID, coverID).Error; err != nil {
		return err
	}
	res := tx.Exec(`UPDATE catalog_work_cover SET portrait_pinned = true, updated_at = now()
		WHERE id = ? AND work_id = ?`, coverID, workID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("cover %d is not on work %d", coverID, workID)
	}
	r.stats.Repinned++
	r.touched = append(r.touched, workID)
	return nil
}

func (r *runner) purge(ctx context.Context, opts Opts) error {
	var rows []coverRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT c.id, c.work_id, c.image_hash, c.kind, s.key AS source_key,
		       c.sexual, c.sort_order, c.portrait_pinned
		FROM catalog_work_cover c
		JOIN catalog_source s ON s.id = c.source_id
		WHERE s.key = ? AND c.kind IN ?
		ORDER BY c.work_id`, upscaleSourceKey, badUpscaleKinds).Scan(&rows).Error; err != nil {
		return fmt.Errorf("load bad upscales: %w", err)
	}
	ids := make([]int64, 0, len(rows))
	works := make([]int64, 0, len(rows))
	held := 0
	for _, row := range rows {
		slog.Info("bad upscale", "cover_id", row.ID, "work", row.WorkID, "kind", row.Kind,
			"pinned", row.Pinned, "held", row.Pinned, "url", r.urlFor(row.Hash))
		if row.Pinned {
			held++
			continue
		}
		ids = append(ids, row.ID)
		works = append(works, row.WorkID)
	}
	r.stats.Skipped = held
	if len(ids) == 0 || !opts.Apply {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Exec(`DELETE FROM catalog_work_cover WHERE id IN ?`, ids)
		if res.Error != nil {
			return res.Error
		}
		r.stats.Purged = int(res.RowsAffected)
		if res.RowsAffected == 0 {
			return nil
		}
		return repository.TouchWorks(ctx, tx, works)
	})
}

func (r *runner) urlFor(hash string) string {
	if r.cli == nil {
		return hash
	}
	return r.cli.MainURL(hash)
}

func isQuota(err error) bool    { return stderrors.Is(err, imageclient.ErrQuotaExceeded) }
func isRejected(err error) bool { return stderrors.Is(err, imageclient.ErrModerationRejected) }

func (r *runner) upload(ctx context.Context, body []byte, filename string) (*imageclient.UploadResult, error) {
	var lastErr error
	for attempt := 0; attempt < uploadRetries; attempt++ {
		if r.gap > 0 {
			time.Sleep(r.gap)
		}
		res, err := r.cli.UploadWithSub(ctx, bytes.NewReader(body), filename, coverPreset, uploaderSub)
		if err == nil {
			return res, nil
		}
		if isQuota(err) {
			r.stats.QuotaBreak = true
			return nil, err
		}
		if isRejected(err) {
			return nil, err
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt < uploadRetries-1 {
			time.Sleep(time.Duration(min(5<<attempt, 30)) * time.Second)
		}
	}
	return nil, lastErr
}

func (r *runner) finish(ctx context.Context) error {
	if err := repository.TouchWorks(ctx, r.db, r.touched); err != nil {
		return fmt.Errorf("touch works: %w", err)
	}
	if r.cli == nil || len(r.fresh) == 0 {
		return nil
	}
	for i := 0; i < len(r.fresh); i += 1000 {
		batch := r.fresh[i:min(i+1000, len(r.fresh))]
		if _, err := r.cli.ReferencePing(ctx, batch); err != nil {
			slog.Warn("refping fresh hashes", "err", err)
			return nil
		}
	}
	return nil
}

func (r *runner) sourceID(ctx context.Context, key string) (int16, error) {
	var id int16
	if err := r.db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = ?`, key).Scan(&id).Error; err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, fmt.Errorf("catalog_source %q is not seeded", key)
	}
	return id, nil
}
