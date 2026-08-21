package service

import (
	"context"
	"slices"
	"strings"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/model"
)

type WorksListInclude struct {
	Names   bool
	Intros  bool
	Labels  bool
	Ratings bool
	Covers  bool
	Refs    bool
}

func (i WorksListInclude) any() bool {
	return i.Names || i.Intros || i.Labels || i.Ratings || i.Covers || i.Refs
}

// intersect is the "fields acts AFTER include" rule: a block is loaded only
// when the caller asked for it AND its keys can still reach the wire. names
// carries two keys, so either one keeps it.
func (i WorksListInclude) intersect(sel PublicFields) WorksListInclude {
	return WorksListInclude{
		Names:   i.Names && sel.Wants("latin", "localized"),
		Intros:  i.Intros && sel.Wants("intros"),
		Labels:  i.Labels && sel.Wants("labels"),
		Ratings: i.Ratings && sel.Wants("ratings"),
		Covers:  i.Covers && sel.Wants("covers"),
		Refs:    i.Refs && sel.Wants("refs"),
	}
}

func ParseWorksListInclude(raw string) WorksListInclude {
	var inc WorksListInclude
	for _, tok := range strings.Split(raw, ",") {
		switch strings.TrimSpace(tok) {
		case "names":
			inc.Names = true
		case "intros":
			inc.Intros = true
		case "labels":
			inc.Labels = true
		case "ratings":
			inc.Ratings = true
		case "covers":
			inc.Covers = true
		case "refs":
			inc.Refs = true
		}
	}
	return inc
}

func (s *PublicService) attachWorkListBlocks(
	ctx context.Context, items []dto.PublicWorkListItem, rows []workListSourceRow,
	subjects []claimSubject, covers map[int64][]WorkCoverRow, inc WorksListInclude, nsfw bool,
	displayNSFW map[int64]bool,
) error {
	if !inc.any() || len(items) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}

	if inc.Names {
		titles, err := s.read.loadWorkTitles(ctx, subjects)
		if err != nil {
			return err
		}
		for i, r := range rows {
			items[i].Latin = workLatin(titles[r.ID], items[i].DisplayName)
			items[i].Localized = workLocalized(titles[r.ID])
		}
	}
	if inc.Intros {
		intros, err := s.read.loadWorkIntros(ctx, subjects)
		if err != nil {
			return err
		}
		for i, r := range rows {
			items[i].Intros = s.workIntros(intros[r.ID])
		}
	}
	if inc.Labels {
		labels, err := s.read.loadWorkLabels(ctx, ids)
		if err != nil {
			return err
		}
		blocks := make([][]dto.PublicWorkLabel, len(rows))
		for i, r := range rows {
			items[i].Labels = publicWorkLabels(labels[r.ID])
			blocks[i] = items[i].Labels
		}
		if err := s.fillWorkLabelCounts(ctx, blocks, nsfw); err != nil {
			return err
		}
		if err := s.fillLabelLocalized(ctx, blocks...); err != nil {
			return err
		}
	}
	if inc.Ratings {
		ratings, err := s.read.loadWorkRatings(ctx, subjects)
		if err != nil {
			return err
		}
		for i, r := range rows {
			items[i].Ratings = s.publicRatings(ratings[r.ID])
		}
	}
	if inc.Covers {
		all := make([]WorkCoverRow, 0, len(rows))
		for _, r := range rows {
			all = append(all, covers[r.ID]...)
		}
		meta := s.coverMetaFor(ctx, all)
		for i, r := range rows {
			items[i].Covers = s.pickCoverSlots(covers[r.ID], meta, nsfw && displayNSFW[r.ID])
		}
	}
	if inc.Refs {
		refs, err := s.workListRefs(ctx, ids)
		if err != nil {
			return err
		}
		for i, r := range rows {
			items[i].Refs = refs[r.ID]
		}
	}
	return nil
}

func (s *PublicService) workListRefs(ctx context.Context, ids []int64) (map[int64][]dto.PublicCatalogRef, error) {
	out := make(map[int64][]dto.PublicCatalogRef, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		WorkID     int64  `gorm:"column:work_id"`
		Source     string `gorm:"column:source"`
		ExternalID string `gorm:"column:external_id"`
	}
	if err := s.db.WithContext(ctx).Raw(`
		SELECT r.entity_id AS work_id, src.key AS source, r.external_id
		FROM catalog_external_ref r JOIN catalog_source src ON src.id = r.source_id
		WHERE r.entity_type = ? AND r.entity_id IN ? AND r.link_kind = ? AND r.dead_at IS NULL
		UNION ALL
		SELECT rel.work_id, src.key AS source, r.external_id
		FROM catalog_external_ref r
		JOIN catalog_release rel ON rel.id = r.entity_id AND rel.deleted_at IS NULL
		JOIN catalog_source src ON src.id = r.source_id
		WHERE r.entity_type = ? AND rel.work_id IN ? AND r.link_kind = ? AND r.dead_at IS NULL
		ORDER BY work_id, source, external_id`,
		model.EntityTypeWork, ids, model.LinkKindExact,
		model.EntityTypeRelease, ids, model.LinkKindExact).Scan(&rows).Error; err != nil {
		return nil, err
	}
	seen := make(map[[2]any]struct{}, len(rows))
	for _, r := range rows {
		key := [2]any{r.WorkID, r.Source + "\x00" + r.ExternalID}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out[r.WorkID] = append(out[r.WorkID], dto.PublicCatalogRef{
			Source: r.Source, ExternalID: r.ExternalID,
		})
	}
	return out, nil
}

func (s *PublicService) workIntros(rows []WorkIntroRow) []dto.PublicIntro {
	out := make([]dto.PublicIntro, 0, len(rows))
	for _, in := range rows {
		out = append(out, dto.PublicIntro{
			Lang: in.Lang, Intro: in.Intro, Source: s.sourceKey(in.SourceID), Machine: in.Machine,
		})
	}
	return out
}

func publicWorkLabels(rows []LabelAttribution) []dto.PublicWorkLabel {
	if len(rows) == 0 {
		return nil
	}
	out := make([]dto.PublicWorkLabel, 0, len(rows))
	at := make(map[int64]int, len(rows))
	primary := make(map[int64]int16, len(rows))
	for _, l := range rows {
		kind := workLabelKindKey(l.Kind)
		if i, ok := at[l.LabelID]; ok {
			out[i].Kinds = append(out[i].Kinds, kind)
			if workLabelKindRank(l.Kind) < workLabelKindRank(primary[l.LabelID]) {
				primary[l.LabelID] = l.Kind
				out[i].Kind = kind
			}
			continue
		}
		at[l.LabelID] = len(out)
		primary[l.LabelID] = l.Kind
		out = append(out, dto.PublicWorkLabel{
			ID: l.LabelID, DisplayName: l.DisplayName,
			LabelKind: labelKindKey(l.LabelKind), Kind: kind, Kinds: []string{kind}, Lang: l.Lang,
			LogoHash: l.LogoHash,
		})
	}
	for i := range out {
		slices.Sort(out[i].Kinds)
		out[i].Kinds = slices.Compact(out[i].Kinds)
	}
	return out
}

func workLabelKindRank(k int16) int {
	switch k {
	case model.WorkLabelKindBrand:
		return 0
	case model.WorkLabelKindCircle:
		return 1
	case model.WorkLabelKindDeveloper:
		return 2
	case model.WorkLabelKindPublisher:
		return 3
	default:
		return 4
	}
}

// The list block deliberately drops distribution/stats that the detail face
// carries: they exist for a per-source modal opened on one work, and a 50-item
// page with ?include=ratings would pay for 50 of them to render none.
func (s *PublicService) publicRatings(rows []WorkRatingRow) []dto.PublicRating {
	if len(rows) == 0 {
		return nil
	}
	out := make([]dto.PublicRating, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.PublicRating{
			Source: s.sourceKey(r.SourceID), Score: r.Score, VoteCount: r.VoteCount, Rank: r.Rank,
		})
	}
	return out
}

func isPortraitDims(w, h int) bool { return int64(h)*20 > int64(w)*21 }

func isBannerDims(w, h int) bool { return int64(w)*3 >= int64(h)*4 }

const bannerMinWidth = 800

func isCoverArt(kind string) bool {
	switch kind {
	case "pkgback", "pkgmed", "pkgcontent", "pkgside":
		return false
	default:
		return true
	}
}

func (s *PublicService) pickCoverSlots(rows []WorkCoverRow, meta map[string]ImageMeta, allowSexual bool) *dto.PublicWorkCoverSlots {
	cand := s.scanCovers(rows, meta, false)
	if allowSexual && !cand.complete() {
		cand.fillFrom(s.scanCovers(rows, meta, true))
	}
	if cand.first == nil {
		return nil
	}
	return &dto.PublicWorkCoverSlots{
		Portrait: s.coverSlot(cand.portrait(), meta),
		Banner:   s.coverSlot(cand.banner(), meta),
	}
}

type coverCandidates struct {
	pinned, portraitArt, portraitAny *WorkCoverRow
	bannerWide, bannerAny            *WorkCoverRow
	first                            *WorkCoverRow
}

func (c coverCandidates) portrait() *WorkCoverRow {
	switch {
	case c.pinned != nil:
		return c.pinned
	case c.portraitArt != nil:
		return c.portraitArt
	case c.portraitAny != nil:
		return c.portraitAny
	default:
		return c.first
	}
}

func (c coverCandidates) banner() *WorkCoverRow {
	if c.bannerWide != nil {
		return c.bannerWide
	}
	return c.bannerAny
}

func (c coverCandidates) complete() bool {
	return c.pinned != nil && c.portraitArt != nil && c.portraitAny != nil &&
		c.bannerWide != nil && c.bannerAny != nil && c.first != nil
}

func (c *coverCandidates) fillFrom(other coverCandidates) {
	for _, pair := range []struct{ dst, src **WorkCoverRow }{
		{&c.pinned, &other.pinned}, {&c.portraitArt, &other.portraitArt},
		{&c.portraitAny, &other.portraitAny}, {&c.bannerWide, &other.bannerWide},
		{&c.bannerAny, &other.bannerAny}, {&c.first, &other.first},
	} {
		if *pair.dst == nil {
			*pair.dst = *pair.src
		}
	}
}

func (s *PublicService) scanCovers(rows []WorkCoverRow, meta map[string]ImageMeta, allowSexual bool) coverCandidates {
	var out coverCandidates
	for i := range rows {
		c := &rows[i]
		if !allowSexual && c.Sexual != 0 {
			continue
		}
		if s.imageURL(c.ImageHash) == "" {
			continue
		}
		if out.first == nil {
			out.first = c
		}
		if c.PortraitPinned && out.pinned == nil {
			out.pinned = c
		}
		m, ok := meta[c.ImageHash]
		if !ok || m.Width <= 0 || m.Height <= 0 {
			continue
		}
		switch {
		case isPortraitDims(m.Width, m.Height):
			if out.portraitAny == nil {
				out.portraitAny = c
			}
			if isCoverArt(c.Kind) && out.portraitArt == nil {
				out.portraitArt = c
			}
		case isBannerDims(m.Width, m.Height) && isCoverArt(c.Kind):
			if out.bannerAny == nil {
				out.bannerAny = c
			}
			if m.Width >= bannerMinWidth && out.bannerWide == nil {
				out.bannerWide = c
			}
		}
	}
	return out
}

func (s *PublicService) coverSlot(c *WorkCoverRow, meta map[string]ImageMeta) *dto.PublicCoverSlot {
	if c == nil {
		return nil
	}
	slot := &dto.PublicCoverSlot{
		URL: s.imageURL(c.ImageHash), Sexual: c.Sexual, Violence: c.Violence,
		Source: s.sourceKey(c.SourceID),
	}
	if m, ok := meta[c.ImageHash]; ok {
		slot.Width, slot.Height, slot.Thumbhash = m.Width, m.Height, m.Thumbhash
	}
	return slot
}
