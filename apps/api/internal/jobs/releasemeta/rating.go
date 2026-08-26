package releasemeta

import (
	"context"
	"fmt"
	"strconv"

	"api/internal/platform/catalog/editspec"
	"api/internal/platform/catalog/model"
	"api/internal/platform/editing"

	"gorm.io/gorm"
)

func runRatingLane(ctx context.Context, db, dlDB, egDB *gorm.DB, w *writer, reg registry, opts Opts) error {
	cands, err := loadRatingCandidates(ctx, db, opts.Limit, opts.Offset)
	if err != nil {
		return fmt.Errorf("load rating candidates: %w", err)
	}
	st := w.stats
	st.RatingCandidates = len(cands)
	if len(cands) == 0 {
		return nil
	}

	dlAnchors, err := loadRatingDlsiteAnchors(ctx, db, reg)
	if err != nil {
		return fmt.Errorf("load rating dlsite anchors: %w", err)
	}
	vndbR18, err := loadRatingVndbR18(ctx, db, reg)
	if err != nil {
		return fmt.Errorf("load rating vndb r18: %w", err)
	}
	egAnchors, err := loadRatingEgAnchors(ctx, db, reg)
	if err != nil {
		return fmt.Errorf("load rating eg anchors: %w", err)
	}
	bgmR18, err := loadRatingBgmR18(ctx, db, reg)
	if err != nil {
		return fmt.Errorf("load rating bgm r18: %w", err)
	}

	worknoSet := map[string]bool{}
	egIDSet := map[int64]bool{}
	for _, c := range cands {
		if wn, ok := dlAnchors[c.WorkID]; ok {
			worknoSet[wn] = true
		}
		for _, id := range egAnchors[c.WorkID] {
			egIDSet[id] = true
		}
	}
	worknos := make([]string, 0, len(worknoSet))
	for wn := range worknoSet {
		worknos = append(worknos, wn)
	}
	dlAges, err := loadDlsiteAges(ctx, dlDB, worknos)
	if err != nil {
		return fmt.Errorf("load dlsite mirror ages: %w", err)
	}
	egIDs := make([]int64, 0, len(egIDSet))
	for id := range egIDSet {
		egIDs = append(egIDs, id)
	}
	egErogame, err := loadEgErogame(ctx, egDB, egIDs)
	if err != nil {
		return fmt.Errorf("load eg mirror erogame: %w", err)
	}

	ratingWorkIDs := make([]int64, 0, len(cands))
	for _, c := range cands {
		ratingWorkIDs = append(ratingWorkIDs, c.WorkID)
	}
	edited, err := editing.EditedEntities(ctx, db, editspec.TypeWork, editspec.FieldWorkContentRating, ratingWorkIDs)
	if err != nil {
		return fmt.Errorf("load curated content_rating overrides: %w", err)
	}

	for _, c := range cands {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if edited[c.WorkID] {
			st.RatingCuratedOverride++
			continue
		}
		rating, source, ext, ok := decideRating(c, dlAnchors, dlAges, vndbR18, egAnchors, egErogame, bgmR18, st)
		if !ok {
			st.RatingNoVerdict++
			continue
		}
		collectRating(&st.RatingSamples, RatingSample{WorkID: c.WorkID, Source: source, Ext: ext, Rating: rating})
		if rating == model.ContentRatingAllAges {
			continue
		}
		st.RatingPlanned++
		w.fillRating(ctx, c.WorkID, rating, opts.Apply)
	}
	return nil
}

// VNDB outranks the DLsite storefront age: the dlsite anchor is release-level,
// so a 全年齢版 SKU's age "1" describes that edition, while the vndb verdict is
// the work-level "an 18+ release exists". With dlsite first, such a work took
// an all-ages verdict (written as nothing) and stayed 0 forever.
func decideRating(c ratingCandidate, dlAnchors map[int64]string, dlAges map[string]string,
	vndbR18 map[int64]bool, egAnchors map[int64][]int64, egErogame map[int64]bool,
	bgmR18 map[int64]bool, st *Stats) (int16, string, string, bool) {

	if vndbR18[c.WorkID] {
		st.RatingVndbR18++
		return model.ContentRatingR18, "vndb", "", true
	}

	if wn, ok := dlAnchors[c.WorkID]; ok {
		switch dlAges[wn] {
		case "3":
			st.RatingDlR18++
			return model.ContentRatingR18, "dlsite", wn, true
		case "2":
			st.RatingDlSensitive++
			return model.ContentRatingSensitive, "dlsite", wn, true
		case "1":
			st.RatingDlAllAges++
			return model.ContentRatingAllAges, "dlsite", wn, true
		}
	}

	// The wiki's editorial age_limit lane used to sit here, read from
	// galgame.age_limit for claimed works. Wave 149 dropped that table, so the
	// lane is gone rather than merely unfilled. It had in fact stopped
	// producing verdicts earlier: its claim test pinned site == "galgame_wiki",
	// the literal wave 161 renamed, so it silently matched nothing from then on.

	for _, id := range egAnchors[c.WorkID] {
		if egErogame[id] {
			st.RatingEgR18++
			return model.ContentRatingR18, "erogamescape", strconv.FormatInt(id, 10), true
		}
	}

	if bgmR18[c.WorkID] {
		st.RatingBgmR18++
		return model.ContentRatingR18, "bangumi", "", true
	}
	return 0, "", "", false
}
