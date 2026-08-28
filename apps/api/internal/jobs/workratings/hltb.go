package workratings

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"api/internal/platform/catalog/model"

	"gorm.io/gorm"
)

type hltbCandidate struct {
	WorkID  int64
	HltbIDs []int64
}

func loadHltbCandidates(ctx context.Context, db *gorm.DB, reg registry, limit, offset int) ([]hltbCandidate, error) {
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		ExternalID string `gorm:"column:external_id"`
	}
	if err := db.WithContext(ctx).
		Raw(`SELECT w.id AS work_id, r.external_id AS external_id
			FROM catalog_external_ref r
			JOIN catalog_work w ON w.id = r.entity_id
			WHERE r.entity_type = ? AND r.source_id = ? AND r.link_kind IN (?, ?)
				AND r.dead_at IS NULL
				AND w.medium_id = ? AND w.deleted_at IS NULL
			ORDER BY w.id, r.external_id`,
			model.EntityTypeWork, reg.hltbSource, model.LinkKindExact, model.LinkKindProbable,
			reg.galgameMedium).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	byWork := map[int64]*hltbCandidate{}
	var order []int64
	for _, r := range rows {
		id, err := strconv.ParseInt(r.ExternalID, 10, 64)
		if err != nil {
			continue
		}
		c := byWork[r.WorkID]
		if c == nil {
			c = &hltbCandidate{WorkID: r.WorkID}
			byWork[r.WorkID] = c
			order = append(order, r.WorkID)
		}
		c.HltbIDs = append(c.HltbIDs, id)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]hltbCandidate, 0, len(order))
	for _, id := range order {
		out = append(out, *byWork[id])
	}
	return window(out, limit, offset), nil
}

type hltbData struct {
	score   int
	votes   int
	reviews []byte
}

func loadHltbMirror(ctx context.Context, hltbDB *gorm.DB, ids []int64) (map[int64]hltbData, error) {
	out := map[int64]hltbData{}
	for start := 0; start < len(ids); start += 1000 {
		end := min(start+1000, len(ids))
		var batch []struct {
			ID      int64  `gorm:"column:id"`
			Score   int    `gorm:"column:score"`
			Reviews []byte `gorm:"column:reviews"`
		}
		if err := hltbDB.WithContext(ctx).Table("games").Select(`hltb_id AS id,
				coalesce((raw->'data'->'game'->0->>'review_score')::int, 0) AS score,
				raw->'data'->'userReviews' AS reviews`).
			Where("hltb_id IN ?", ids[start:end]).Scan(&batch).Error; err != nil {
			return nil, err
		}
		for _, r := range batch {
			buckets := hltbBuckets(r.Reviews)
			out[r.ID] = hltbData{score: r.Score, votes: hltbReviewCount(r.Reviews), reviews: marshalBuckets(buckets)}
		}
	}
	return out, nil
}

// The userReviews payload mixes types: review_count is a JSON number, but the
// bucket counts are strings ("66") with null standing for zero — parsed here
// value-by-value rather than into a typed map, which json.Unmarshal would
// reject wholesale on the first string.
func hltbBuckets(raw []byte) map[string]int {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		if k == "review_count" {
			continue
		}
		switch n := v.(type) {
		case string:
			if i, err := strconv.Atoi(n); err == nil && i > 0 {
				out[k] = i
			}
		case float64:
			if n > 0 {
				out[k] = int(n)
			}
		}
	}
	return out
}

func hltbReviewCount(raw []byte) int {
	if len(raw) == 0 {
		return 0
	}
	var m struct {
		ReviewCount int `json:"review_count"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0
	}
	return m.ReviewCount
}

func pickBestHltb(ids []int64, mirror map[int64]hltbData) int64 {
	best := ids[0]
	for _, id := range ids[1:] {
		if hltbWorse(best, id, mirror) {
			best = id
		}
	}
	return best
}

func hltbWorse(a, b int64, mirror map[int64]hltbData) bool {
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
	if da.score != db.score {
		return da.score < db.score
	}
	return a < b
}

func runHltbLane(ctx context.Context, db, hltbDB *gorm.DB, w *writer, reg registry, opts Opts) error {
	cands, err := loadHltbCandidates(ctx, db, reg, opts.Limit, opts.Offset)
	if err != nil {
		return fmt.Errorf("load HLTB candidates: %w", err)
	}
	st := w.stats
	st.HltbCandidates = len(cands)

	idSet := map[int64]bool{}
	for _, c := range cands {
		for _, id := range c.HltbIDs {
			idSet[id] = true
		}
	}
	mirror, err := loadHltbMirror(ctx, hltbDB, keysOf(idSet))
	if err != nil {
		return fmt.Errorf("load HLTB mirror games: %w", err)
	}

	for _, c := range cands {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		chosen := pickBestHltb(c.HltbIDs, mirror)
		if len(c.HltbIDs) > 1 {
			st.HltbMultiAnchor += len(c.HltbIDs) - 1
		}
		h, ok := mirror[chosen]
		if !ok {
			st.HltbMissingMirror++
			continue
		}
		if h.score <= 0 || h.votes <= 0 {
			st.HltbNoScore++
			continue
		}
		if h.reviews != nil {
			st.HltbDistribution++
		}
		st.HltbPlanned++
		collect(&st.HltbSamples, Sample{WorkID: c.WorkID, ExternalID: chosen, Score: float64(h.score), VoteCount: h.votes})
		w.write(ctx, plannedRow{
			WorkID: c.WorkID, SourceID: reg.hltbSource,
			Score: float64(h.score), VoteCount: h.votes, Distribution: h.reviews,
		}, opts.Apply, &st.HltbWritten, &st.HltbUnchanged)
	}
	return nil
}
