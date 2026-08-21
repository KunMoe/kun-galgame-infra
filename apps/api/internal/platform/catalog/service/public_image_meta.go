package service

import (
	"context"
	"sync"

	"api/internal/platform/catalog/dto"
)

type ImageMeta struct {
	Width     int
	Height    int
	Thumbhash string
	Sexual    *int16
}

// An entry is cacheable only once every field it can ever carry is in it. The
// thumbhash backfill and the nightly grader both fill in behind a read, and a
// partial entry pinned in this cache never self-heals: the face keeps serving
// the gap for the process's lifetime.
func (m ImageMeta) complete() bool { return m.Thumbhash != "" && m.Sexual != nil }

type ImageMetaFunc func(ctx context.Context, hashes []string) (map[string]ImageMeta, error)

const imageMetaCacheCap = 200_000

func (s *PublicService) WithImageMeta(fn ImageMetaFunc) *PublicService {
	s.imageMeta = fn
	if fn != nil && s.metaCache == nil {
		s.metaCache = newImageMetaCache(imageMetaCacheCap)
	}
	return s
}

type imageMetaCache struct {
	mu  sync.RWMutex
	m   map[string]ImageMeta
	cap int
}

func newImageMetaCache(capacity int) *imageMetaCache {
	return &imageMetaCache{m: make(map[string]ImageMeta), cap: capacity}
}

func (c *imageMetaCache) get(hash string) (ImageMeta, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.m[hash]
	return v, ok
}

func (c *imageMetaCache) put(hash string, meta ImageMeta) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.cap {
		return
	}
	c.m[hash] = meta
}

func (s *PublicService) resolveImageMeta(ctx context.Context, hashes []string) map[string]ImageMeta {
	out := make(map[string]ImageMeta, len(hashes))
	if len(hashes) == 0 {
		return out
	}

	misses := make([]string, 0, len(hashes))
	seen := make(map[string]struct{}, len(hashes))
	for _, h := range hashes {
		if h == "" {
			continue
		}
		if _, dup := seen[h]; dup {
			continue
		}
		seen[h] = struct{}{}
		if s.metaCache != nil {
			if m, ok := s.metaCache.get(h); ok {
				out[h] = m
				continue
			}
		}
		misses = append(misses, h)
	}

	if len(misses) == 0 || s.imageMeta == nil {
		return out
	}

	for _, chunk := range chunkHashes(misses, 1000) {
		fetched, err := s.imageMeta(ctx, chunk)
		if err != nil {
			break
		}
		for h, m := range fetched {
			out[h] = m
			if s.metaCache != nil && m.complete() {
				s.metaCache.put(h, m)
			}
		}
	}
	return out
}

func (s *PublicService) coverMetaFor(ctx context.Context, rows []WorkCoverRow) map[string]ImageMeta {
	return s.workMediaMetaFor(ctx, rows, nil)
}

func (s *PublicService) workMediaMetaFor(ctx context.Context, covers []WorkCoverRow, shots []WorkScreenshotRow, extra ...string) map[string]ImageMeta {
	if s.imageMeta == nil || (len(covers) == 0 && len(shots) == 0 && len(extra) == 0) {
		return nil
	}
	hashes := make([]string, 0, len(covers)+len(shots)+len(extra))
	for _, c := range covers {
		hashes = append(hashes, c.ImageHash)
	}
	for _, sc := range shots {
		hashes = append(hashes, sc.ImageHash)
	}
	hashes = append(hashes, extra...)
	return s.resolveImageMeta(ctx, hashes)
}

func (s *PublicService) entityMetaFor(ctx context.Context, hashes ...string) map[string]ImageMeta {
	if s.imageMeta == nil || len(hashes) == 0 {
		return nil
	}
	return s.resolveImageMeta(ctx, hashes)
}

func publicImageMeta(meta map[string]ImageMeta, hash string) *dto.PublicImageMeta {
	if hash == "" {
		return nil
	}
	m, ok := meta[hash]
	if !ok {
		return nil
	}
	return &dto.PublicImageMeta{
		Width: m.Width, Height: m.Height, Thumbhash: m.Thumbhash, Sexual: m.Sexual,
	}
}

func derefHashes(ptrs ...*string) []string {
	out := make([]string, 0, len(ptrs))
	for _, p := range ptrs {
		if p != nil {
			out = append(out, *p)
		}
	}
	return out
}

func rosterImageHashes(rows []WorkCharacterRow) []string {
	hashes := make([]string, 0, 2*len(rows))
	for _, ch := range rows {
		if ch.ImageHash != nil {
			hashes = append(hashes, *ch.ImageHash)
		}
		if ch.FigureHash != nil {
			hashes = append(hashes, *ch.FigureHash)
		}
	}
	return hashes
}

func chunkHashes(s []string, size int) [][]string {
	if size <= 0 || len(s) <= size {
		return [][]string{s}
	}
	out := make([][]string, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i += size {
		out = append(out, s[i:min(i+size, len(s))])
	}
	return out
}
