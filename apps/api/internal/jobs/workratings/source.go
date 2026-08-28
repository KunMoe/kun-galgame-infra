package workratings

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

type registry struct {
	galgameMedium int16
	bangumiSource int16
	egSource      int16
	dlsiteSource  int16
	vndbSource    int16
	hltbSource    int16
}

func resolveRegistry(ctx context.Context, db *gorm.DB) (registry, error) {
	var r registry
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_medium WHERE key = 'galgame'`).Scan(&r.galgameMedium).Error; err != nil {
		return r, fmt.Errorf("resolve galgame medium: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'bangumi'`).Scan(&r.bangumiSource).Error; err != nil {
		return r, fmt.Errorf("resolve bangumi source: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'erogamescape'`).Scan(&r.egSource).Error; err != nil {
		return r, fmt.Errorf("resolve erogamescape source: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'dlsite'`).Scan(&r.dlsiteSource).Error; err != nil {
		return r, fmt.Errorf("resolve dlsite source: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'vndb'`).Scan(&r.vndbSource).Error; err != nil {
		return r, fmt.Errorf("resolve vndb source: %w", err)
	}
	if err := db.WithContext(ctx).Raw(`SELECT id FROM catalog_source WHERE key = 'howlongtobeat'`).Scan(&r.hltbSource).Error; err != nil {
		return r, fmt.Errorf("resolve howlongtobeat source: %w", err)
	}
	if r.galgameMedium == 0 || r.bangumiSource == 0 || r.egSource == 0 || r.dlsiteSource == 0 || r.vndbSource == 0 || r.hltbSource == 0 {
		return r, fmt.Errorf("registry not seeded (galgame medium=%d, bangumi source=%d, erogamescape source=%d, dlsite source=%d, vndb source=%d, howlongtobeat source=%d)",
			r.galgameMedium, r.bangumiSource, r.egSource, r.dlsiteSource, r.vndbSource, r.hltbSource)
	}
	return r, nil
}

type bgmCandidate struct {
	WorkID       int64   `gorm:"column:work_id"`
	SubjectID    int64   `gorm:"column:subject_id"`
	Score        float64 `gorm:"column:score"`
	Rank         int     `gorm:"column:rank"`
	ScoreDetails []byte  `gorm:"column:score_details"`
}

func loadBgmCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]bgmCandidate, error) {
	var out []bgmCandidate
	if err := db.WithContext(ctx).
		Raw(`SELECT DISTINCT ON (w.id) w.id AS work_id, r.external_id::bigint AS subject_id,
				sub.score AS score, sub.rank AS rank, sub.score_details AS score_details
			FROM catalog_work w
			JOIN catalog_external_ref r ON r.entity_type = ? AND r.entity_id = w.id
				AND r.source_id = ? AND r.link_kind = ?
			JOIN src_bangumi.subject sub ON sub.id = r.external_id::bigint
			WHERE w.medium_id = ? AND w.deleted_at IS NULL
			ORDER BY w.id, r.external_id`,
			model.EntityTypeWork, reg.bangumiSource, model.LinkKindExact, reg.galgameMedium).
		Scan(&out).Error; err != nil {
		return nil, err
	}
	return window(out, limit, offset), nil
}

type egCandidate struct {
	WorkID int64
	EgIDs  []int64
}

func loadEgCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]egCandidate, error) {
	var rows []struct {
		WorkID     int64   `gorm:"column:work_id"`
		Site       *string `gorm:"column:site"`
		ExternalID string  `gorm:"column:external_id"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT w.id AS work_id, r.external_id AS external_id
			FROM catalog_external_ref r
			JOIN catalog_work w ON w.id = r.entity_id
			WHERE r.entity_type = ? AND r.source_id = ? AND r.link_kind = ?
				AND w.medium_id = ? AND w.deleted_at IS NULL
			ORDER BY w.id, r.external_id`,
			model.EntityTypeWork, reg.egSource, model.LinkKindExact, reg.galgameMedium).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	byWork := map[int64]*egCandidate{}
	var order []int64
	for _, r := range rows {
		egID, err := strconv.ParseInt(r.ExternalID, 10, 64)
		if err != nil {
			egID = -1
		}
		c := byWork[r.WorkID]
		if c == nil {
			c = &egCandidate{WorkID: r.WorkID}
			byWork[r.WorkID] = c
			order = append(order, r.WorkID)
		}
		c.EgIDs = append(c.EgIDs, egID)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]egCandidate, 0, len(order))
	for _, id := range order {
		out = append(out, *byWork[id])
	}
	return window(out, limit, offset), nil
}

type egData struct {
	median *int
	votes  int
	stats  ratingStats
}

// The mirror generates typed columns for only a handful of `raw` keys, and the
// spread four are not among them — hence the casts. They are read from the same
// row as the median on purpose: EG publishes no histogram, and one computed
// from the mirror's `reviews` table would be a different sync generation than
// the aggregate shown beside it.
//
// loadEGReviewBuckets computes that histogram anyway, as a deliberate
// exception: a shape of the vote spread is worth more to a reader than the
// guarantee that its bars add up to the vote_count printed next to them. The
// two bases stay separate — the score, vote_count and spread still come from
// this `games` row, only the bars come from `reviews` — and the basis mismatch
// is documented in the catalog contract rather than papered over here.
func loadEGMirror(ctx context.Context, egDB *gorm.DB, ids []int64) (map[int64]egData, error) {
	out := map[int64]egData{}
	type row struct {
		ID      int64    `gorm:"column:id"`
		Median  *int     `gorm:"column:median"`
		Count2  *int     `gorm:"column:count2"`
		Average *float64 `gorm:"column:average"`
		Stdev   *float64 `gorm:"column:stdev"`
		Min     *float64 `gorm:"column:min"`
		Max     *float64 `gorm:"column:max"`
	}
	for start := 0; start < len(ids); start += 1000 {
		end := min(start+1000, len(ids))
		var batch []row
		if err := egDB.WithContext(ctx).Table("games").Select(`id, median, count2,
				nullif(raw->>'average2', '')::float8 AS average,
				nullif(raw->>'stdev', '')::float8    AS stdev,
				nullif(raw->>'min2', '')::float8     AS min,
				nullif(raw->>'max2', '')::float8     AS max`).
			Where("id IN ?", ids[start:end]).Scan(&batch).Error; err != nil {
			return nil, err
		}
		for _, r := range batch {
			votes := 0
			if r.Count2 != nil {
				votes = *r.Count2
			}
			out[r.ID] = egData{
				median: r.Median, votes: votes,
				stats: ratingStats{Average: r.Average, Stdev: r.Stdev, Min: r.Min, Max: r.Max},
			}
		}
	}
	return out, nil
}

func loadEGReviewBuckets(ctx context.Context, egDB *gorm.DB) (map[int64]map[string]int, error) {
	var rows []struct {
		Game   int64 `gorm:"column:game"`
		Bucket int   `gorm:"column:bucket"`
		N      int   `gorm:"column:n"`
	}
	if err := egDB.WithContext(ctx).Raw(
		`SELECT game, (tokuten / 10) * 10 AS bucket, count(*) AS n
		 FROM reviews WHERE tokuten IS NOT NULL AND game IS NOT NULL
		 GROUP BY 1, 2`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]map[string]int, len(rows)/8+1)
	for _, r := range rows {
		b := out[r.Game]
		if b == nil {
			b = map[string]int{}
			out[r.Game] = b
		}
		b[strconv.Itoa(r.Bucket)] += r.N
	}
	return out, nil
}

func pickBest(egIDs []int64, mirror map[int64]egData) int64 {
	best := egIDs[0]
	for _, id := range egIDs[1:] {
		if worse(best, id, mirror) {
			best = id
		}
	}
	return best
}

func worse(a, b int64, mirror map[int64]egData) bool {
	da, oka := mirror[a]
	db, okb := mirror[b]
	if oka != okb {
		return !oka
	}
	if !oka {
		return a < b
	}
	if da.votes != db.votes {
		return da.votes < db.votes
	}
	ma, mb := derefOr(da.median, -1), derefOr(db.median, -1)
	if ma != mb {
		return ma < mb
	}
	return a < b
}

func derefOr(p *int, fallback int) int {
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

func keysOf(m map[int64]bool) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
