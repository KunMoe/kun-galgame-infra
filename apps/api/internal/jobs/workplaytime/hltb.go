package workplaytime

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"

	"api/internal/platform/catalog/repository"

	"gorm.io/gorm"
)

type hltbGame struct {
	seconds int64
	votes   int
}

func runHltb(ctx context.Context, db *gorm.DB, opts Opts, ids registryIDs, st *Stats) error {
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		ExternalID string `gorm:"column:external_id"`
	}
	if err := db.WithContext(ctx).Raw(`
		SELECT w.id AS work_id, r.external_id
		FROM catalog_work w
		JOIN catalog_external_ref r ON r.entity_type = 5 AND r.entity_id = w.id
			AND r.source_id = ? AND r.link_kind IN (0, 1) AND r.dead_at IS NULL
		WHERE w.medium_id = ? AND w.deleted_at IS NULL
		ORDER BY w.id, r.external_id`, ids.hltbSource, ids.galgameMedium).
		Scan(&rows).Error; err != nil {
		return fmt.Errorf("load hltb anchors: %w", err)
	}
	byWork := map[int64][]int64{}
	var order []int64
	for _, r := range rows {
		id, err := strconv.ParseInt(r.ExternalID, 10, 64)
		if err != nil {
			continue
		}
		if _, seen := byWork[r.WorkID]; !seen {
			order = append(order, r.WorkID)
		}
		byWork[r.WorkID] = append(byWork[r.WorkID], id)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	hltbDB, err := openGorm(opts.HltbDSN)
	if err != nil {
		return fmt.Errorf("connect hltb mirror: %w", err)
	}
	defer closeGorm(hltbDB)

	idSet := map[int64]bool{}
	for _, hltbIDs := range byWork {
		for _, id := range hltbIDs {
			idSet[id] = true
		}
	}
	allIDs := make([]int64, 0, len(idSet))
	for id := range idSet {
		allIDs = append(allIDs, id)
	}
	games := map[int64]hltbGame{}
	for _, chunk := range chunkInt64(allIDs, 10000) {
		var batch []struct {
			ID      int64 `gorm:"column:id"`
			Seconds int64 `gorm:"column:seconds"`
			Votes   int   `gorm:"column:votes"`
		}
		if err := hltbDB.WithContext(ctx).Raw(`
			SELECT hltb_id AS id,
				coalesce(nullif((raw->'data'->'game'->0->>'comp_main_med')::bigint, 0),
					(raw->'data'->'game'->0->>'comp_main')::bigint, 0) AS seconds,
				coalesce((raw->'data'->'game'->0->>'comp_main_count')::int, 0) AS votes
			FROM games WHERE hltb_id IN ?`, chunk).
			Scan(&batch).Error; err != nil {
			return fmt.Errorf("load hltb playtimes: %w", err)
		}
		for _, g := range batch {
			games[g.ID] = hltbGame{seconds: g.Seconds, votes: g.Votes}
		}
	}

	var touched []int64
	for _, workID := range order {
		best, ok := pickHltb(byWork[workID], games)
		if !ok {
			continue
		}
		g := games[best]
		minutes := int(math.Round(float64(g.seconds) / 60))
		if minutes <= 0 || g.votes <= 0 {
			continue
		}
		st.HltbAnchored++
		if minutes > capMinutes {
			st.HltbRejected++
			continue
		}
		st.HltbPlanned++
		if !opts.Apply {
			continue
		}
		if upsert(ctx, db, workID, ids.hltbSource, minutes, g.votes, &st.HltbWritten, &st.HltbUnchanged, &st.Errors) {
			touched = append(touched, workID)
		}
	}
	return repository.TouchWorks(ctx, db, touched)
}

func pickHltb(ids []int64, games map[int64]hltbGame) (int64, bool) {
	best, found := int64(0), false
	for _, id := range ids {
		g, ok := games[id]
		if !ok {
			continue
		}
		if !found {
			best, found = id, true
			continue
		}
		b := games[best]
		if g.votes > b.votes || (g.votes == b.votes && id < best) {
			best = id
		}
	}
	return best, found
}
