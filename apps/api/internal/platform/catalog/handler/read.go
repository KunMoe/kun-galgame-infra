package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/model"
	catsearch "api/internal/platform/catalog/search"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"

	"github.com/danielgtaylor/huma/v2"
)

func (s *S2SServer) registerRead(api huma.API) {
	tags := []string{"catalog-s2s"}
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkByAnchor", Method: http.MethodGet, Path: "/api/v1/catalog/works/by-anchor",
		Summary: "Read a work through one of its external anchors (work- or release-level)", Tags: tags,
	}, s.workByAnchor)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkCredits", Method: http.MethodGet, Path: "/api/v1/catalog/works/{id}/credits",
		Summary: "List a work's credits grouped by role", Tags: tags,
	}, s.workCredits)
	huma.Register(api, huma.Operation{
		OperationID: "searchCatalogEntities", Method: http.MethodGet, Path: "/api/v1/catalog/search/entities",
		Summary: "Search catalog entities (credit names / characters / labels)", Tags: tags,
	}, s.searchEntities)
	huma.Register(api, huma.Operation{
		OperationID: "searchCatalogWorks", Method: http.MethodGet, Path: "/api/v1/catalog/works/search",
		Summary: "Search works by title (NFKC-folded substring) for the product-side create picker", Tags: tags,
	}, s.searchWorks)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogStats", Method: http.MethodGet, Path: "/api/v1/catalog/stats",
		Summary: "Dashboard counts for the internal data browser (one round-trip)", Tags: tags,
	}, s.getStats)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkByID", Method: http.MethodGet, Path: "/api/v1/catalog/works/{id}",
		Summary: "Read a work by catalog id (same bundle as by-anchor)", Tags: tags,
	}, s.workByID)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogLabelWorks", Method: http.MethodGet, Path: "/api/v1/catalog/labels/{id}/works",
		Summary: "Works attributed to a label (circle→works reverse, paginated)", Tags: tags,
	}, s.labelWorks)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogNameWorks", Method: http.MethodGet, Path: "/api/v1/catalog/names/{id}/works",
		Summary: "Works a credited name worked on (name→works reverse; sibling names + roles)", Tags: tags,
	}, s.nameWorks)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogCharacterWorks", Method: http.MethodGet, Path: "/api/v1/catalog/characters/{id}/works",
		Summary: "Works a character appears in (roster edges ∪ voice credits) with kind + voice names", Tags: tags,
	}, s.characterWorks)
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogCharacterByID", Method: http.MethodGet, Path: "/api/v1/catalog/characters/{id}",
		Summary: "Read a character's identity + aliases by catalog id", Tags: tags,
	}, s.characterByID)
}

type byAnchorInput struct {
	Source     string `query:"source" minLength:"1" doc:"Source key (dlsite/vndb/bangumi/erogamescape/…), validated against the source registry"`
	ExternalID string `query:"external_id" minLength:"1" doc:"The id within that source (e.g. a DLsite RJ number, a VNDB v-id)"`
}

type byAnchorOutput struct {
	Body Envelope[dto.WorkByAnchorResponse]
}

func (s *S2SServer) workByAnchor(ctx context.Context, in *byAnchorInput) (*byAnchorOutput, error) {
	detail, err := s.read.WorkByAnchor(ctx, in.Source, in.ExternalID)
	if err != nil {
		return nil, workDetailErr(err)
	}
	votes, err := s.coverVotes(ctx, detail)
	if err != nil {
		return nil, err
	}
	return &byAnchorOutput{Body: okEnvelope(buildWorkResponse(detail, votes))}, nil
}

type workByIDInput struct {
	ID int64 `path:"id" doc:"Catalog work id"`
}

func (s *S2SServer) workByID(ctx context.Context, in *workByIDInput) (*byAnchorOutput, error) {
	detail, err := s.read.WorkByID(ctx, in.ID, 0)
	if err != nil {
		return nil, workDetailErr(err)
	}
	votes, err := s.coverVotes(ctx, detail)
	if err != nil {
		return nil, err
	}
	return &byAnchorOutput{Body: okEnvelope(buildWorkResponse(detail, votes))}, nil
}

func (s *S2SServer) coverVotes(ctx context.Context, detail *service.WorkDetail) (map[int64]service.CoverVoteTally, error) {
	if len(detail.Covers) == 0 {
		return nil, nil
	}
	coverIDs := make([]int64, 0, len(detail.Covers))
	for _, cv := range detail.Covers {
		coverIDs = append(coverIDs, cv.ID)
	}
	votes, err := s.read.CoverVotes(ctx, coverIDs, 0)
	if err != nil {
		slog.Error("catalog cover votes", "err", err)
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	return votes, nil
}

func workDetailErr(err error) error {
	if stderrors.Is(err, service.ErrWorkNotFound) {
		return apiErr(http.StatusNotFound, errors.ErrNotFound)
	}
	return apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
}

func buildWorkResponse(detail *service.WorkDetail, votes map[int64]service.CoverVoteTally) dto.WorkByAnchorResponse {
	resp := dto.WorkByAnchorResponse{
		Work: dto.WorkCore{
			ID: detail.Work.ID, MediumID: detail.Work.MediumID, DisplayName: detail.Work.DisplayName,
			OLang: detail.Work.OLang, ContentRating: detail.Work.ContentRating, Status: detail.Work.Status,
		},
		Titles:         make([]dto.WorkTitle, 0, len(detail.Titles)),
		Releases:       make([]dto.ReleaseBrief, 0, len(detail.Releases)),
		Labels:         make([]dto.WorkLabel, 0, len(detail.Labels)),
		Refs:           make([]dto.WorkRef, 0, len(detail.Refs)),
		Characters:     make([]dto.WorkCharacter, 0, len(detail.Characters)),
		Intro:          make([]dto.WorkIntro, 0, len(detail.Intros)),
		Covers:         make([]dto.WorkCover, 0, len(detail.Covers)),
		Screenshots:    make([]dto.WorkScreenshot, 0, len(detail.Screenshots)),
		Ratings:        make([]dto.WorkRating, 0, len(detail.Ratings)),
		Tags:           make([]dto.WorkTag, 0, len(detail.Tags)),
		Popularity:     make([]dto.WorkPopularity, 0, len(detail.Popularity)),
		Playtimes:      make([]dto.WorkPlaytime, 0, len(detail.Playtimes)),
		Series:         make([]dto.WorkSeries, 0, len(detail.Series)),
		Platforms:      make([]dto.WorkPlatform, 0, len(detail.Platforms)),
		Relations:      make([]dto.WorkRelation, 0, len(detail.Relations)),
		SeriesSiblings: make([]dto.WorkSeriesSibling, 0, len(detail.SeriesSiblings)),
	}
	if detail.Work.Site != nil {
		resp.Work.Site = *detail.Work.Site
	}
	if detail.Work.ProductWorkID != nil {
		resp.Work.ProductWorkID = *detail.Work.ProductWorkID
	}
	for _, t := range detail.Titles {
		resp.Titles = append(resp.Titles, dto.WorkTitle{
			Lang: t.Lang, Title: t.Title, Latin: t.Latin, Kind: t.Kind,
			Machine: t.Provenance == model.WorkTitleProvenanceMachine,
		})
	}
	for _, rd := range detail.Releases {
		rb := dto.ReleaseBrief{ID: rd.Release.ID, Kind: rd.Release.Kind, Hidden: rd.Release.DeletedAt.Valid}
		if rd.Release.Platform != nil {
			rb.Platform = *rd.Release.Platform
		}
		rb.Platforms = platformsFromExtra(rd.Release.Extra)
		rb.ReleasedY, rb.ReleasedM, rb.ReleasedD = derefI16(rd.Release.ReleasedY), derefI16(rd.Release.ReleasedM), derefI16(rd.Release.ReleasedD)
		for _, a := range rd.Anchors {
			rb.Anchors = append(rb.Anchors, dto.AnchorRef{Source: a.Source, ExternalID: a.ExternalID, LinkKind: a.LinkKind, MatchedBy: a.MatchedBy})
		}
		resp.Releases = append(resp.Releases, rb)
	}
	for _, l := range detail.Labels {
		resp.Labels = append(resp.Labels, dto.WorkLabel{
			LabelID: l.LabelID, DisplayName: l.DisplayName, LabelKind: l.LabelKind, Kind: l.Kind,
		})
	}
	for _, rf := range detail.Refs {
		wr := dto.WorkRef{Source: rf.Source, ExternalID: rf.ExternalID, Level: "work"}
		if rf.EntityType == model.EntityTypeRelease {
			wr.Level, wr.ReleaseID = "release", rf.ReleaseID
		}
		resp.Refs = append(resp.Refs, wr)
	}
	for _, c := range detail.Characters {
		wc := dto.WorkCharacter{
			CharacterID: c.CharacterID, DisplayName: c.DisplayName, Latin: derefStr(c.Latin),
			Gender: derefI16(c.Gender), Kind: c.Kind, Spoiler: c.Spoiler, ImageHash: derefStr(c.ImageHash),
			FigureHash: derefStr(c.FigureHash), Identity: c.Identity,
			Va: make([]dto.WorkCharacterVA, 0, len(c.Va)),
		}
		for _, v := range c.Va {
			wc.Va = append(wc.Va, dto.WorkCharacterVA{CreditNameID: v.CreditNameID, Name: v.Name})
		}
		resp.Characters = append(resp.Characters, wc)
	}
	for _, in := range detail.Intros {
		resp.Intro = append(resp.Intro, dto.WorkIntro{Lang: in.Lang, Intro: in.Intro, SourceID: in.SourceID, Machine: in.Machine})
	}
	for _, cv := range detail.Covers {
		tally := votes[cv.ID]
		resp.Covers = append(resp.Covers, dto.WorkCover{
			ID: cv.ID, ImageHash: cv.ImageHash, Kind: cv.Kind, PortraitPinned: cv.PortraitPinned,
			SortOrder: cv.SortOrder, Sexual: cv.Sexual, Violence: cv.Violence, SourceID: cv.SourceID,
			VoteCount: tally.Count,
		})
	}
	for _, sh := range detail.Screenshots {
		resp.Screenshots = append(resp.Screenshots, dto.WorkScreenshot{
			ImageHash: sh.ImageHash, Caption: sh.Caption,
			SortOrder: sh.SortOrder, Sexual: sh.Sexual, Violence: sh.Violence, SourceID: sh.SourceID,
		})
	}
	for _, rt := range detail.Ratings {
		resp.Ratings = append(resp.Ratings, dto.WorkRating{
			SourceID: rt.SourceID, Score: rt.Score, VoteCount: rt.VoteCount, Rank: rt.Rank,
			Distribution: rt.Distribution, Stats: rt.Stats,
		})
	}
	for _, tg := range detail.Tags {
		resp.Tags = append(resp.Tags, dto.WorkTag{
			Name: tg.Name, Count: tg.Count, SourceID: tg.SourceID,
			CanonicalID: tg.CanonicalID, Tier: tg.Tier, Kind: tg.Kind,
		})
	}
	for _, pp := range detail.Popularity {
		resp.Popularity = append(resp.Popularity, dto.WorkPopularity{
			SourceID: pp.SourceID, Metric: pp.Metric, Value: pp.Value,
		})
	}
	for _, pt := range detail.Playtimes {
		resp.Playtimes = append(resp.Playtimes, dto.WorkPlaytime{
			SourceID: pt.SourceID, Minutes: pt.Minutes, VoteCount: pt.VoteCount,
		})
	}
	for _, se := range detail.Series {
		resp.Series = append(resp.Series, dto.WorkSeries{
			ID: se.ID, Name: se.Name, SourceID: se.SourceID, MemberCount: se.MemberCount,
		})
	}
	for _, p := range detail.Platforms {
		resp.Platforms = append(resp.Platforms, dto.WorkPlatform{Platform: p.Platform, SourceID: p.SourceID})
	}
	for _, rl := range detail.Relations {
		wr := dto.WorkRelation{
			RelationType: rl.Key, Phrase: rl.Phrase, WorkID: rl.OtherID,
			DisplayName: rl.DisplayName, MediumID: rl.MediumID,
			ContentRating: rl.ContentRating, Status: rl.Status,
		}
		if rl.Site != nil {
			wr.Site = *rl.Site
		}
		if rl.ProductWorkID != nil {
			wr.ProductWorkID = *rl.ProductWorkID
		}
		resp.Relations = append(resp.Relations, wr)
	}
	for _, sb := range detail.SeriesSiblings {
		ws := dto.WorkSeriesSibling{
			WorkID: sb.WorkID, DisplayName: sb.DisplayName, MediumID: sb.MediumID,
			ContentRating: sb.ContentRating, Status: sb.Status,
		}
		if sb.Site != nil {
			ws.Site = *sb.Site
		}
		if sb.ProductWorkID != nil {
			ws.ProductWorkID = *sb.ProductWorkID
		}
		resp.SeriesSiblings = append(resp.SeriesSiblings, ws)
	}
	return resp
}

func platformsFromExtra(extra []byte) []string {
	if len(extra) == 0 {
		return nil
	}
	var e struct {
		Platforms []string `json:"platforms"`
	}
	if err := json.Unmarshal(extra, &e); err != nil {
		return nil
	}
	return e.Platforms
}

type creditsInput struct {
	ID int64 `path:"id" doc:"Catalog work id"`
}

type creditsOutput struct {
	Body Envelope[dto.WorkCreditsResponse]
}

func (s *S2SServer) workCredits(ctx context.Context, in *creditsInput) (*creditsOutput, error) {
	rows, err := s.read.WorkCredits(ctx, in.ID)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	resp := dto.WorkCreditsResponse{WorkID: in.ID, Groups: make([]dto.CreditGroup, 0)}
	var cur *dto.CreditGroup
	for _, r := range rows {
		if cur == nil || cur.RoleID != r.RoleID {
			resp.Groups = append(resp.Groups, dto.CreditGroup{
				RoleID: r.RoleID, RoleKey: r.RoleKey, RoleName: firstNonEmpty(r.RoleNameCN, r.RoleNameJA, r.RoleKey),
			})
			cur = &resp.Groups[len(resp.Groups)-1]
		}
		item := dto.CreditItem{CreditNameID: r.CreditNameID, Name: r.Name, Lang: r.Lang, Note: r.Note, Identity: r.Identity}
		if r.Latin != nil {
			item.Latin = *r.Latin
		}
		if r.CharacterID != nil {
			item.CharacterID = *r.CharacterID
		}
		if r.CharacterNM != nil {
			item.Character = *r.CharacterNM
		}
		if r.SourceKey != nil {
			item.Source = *r.SourceKey
		}
		cur.Credits = append(cur.Credits, item)
	}
	return &creditsOutput{Body: okEnvelope(resp)}, nil
}

type searchWorksInput struct {
	Q          string `query:"q" minLength:"1" doc:"Title search text (NFKC-folded substring match)"`
	MediumID   int16  `query:"medium_id" default:"-1" doc:"Filter to one medium; -1 = all"`
	Limit      int    `query:"limit" default:"20" doc:"Max hits (capped at 50)"`
	ClaimState string `query:"claim_state" doc:"Comma-separated subset of none, live, draft, pending, declined, hidden; absent = every state"`
	Site       string `query:"site" doc:"Restrict to works claimed by ONE site; absent = every tenant and every unclaimed work. Live SQL predicate — no reindex delay"`
}

type searchWorksOutput struct {
	Body Envelope[dto.WorkSearchResponse]
}

func (s *S2SServer) searchWorks(ctx context.Context, in *searchWorksInput) (*searchWorksOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	claimStates, ok := claimStatesPub(in.ClaimState)
	if !ok {
		return nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam, msgBadClaimState)
	}
	hits, err := s.read.SearchWorks(ctx, in.Q, in.MediumID, limit, claimStates, in.Site)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	resp := dto.WorkSearchResponse{Items: make([]dto.WorkSearchHit, 0, len(hits))}
	for _, h := range hits {
		resp.Items = append(resp.Items, dto.WorkSearchHit{
			WorkID: h.WorkID, DisplayName: h.DisplayName, MediumID: h.MediumID,
			ContentRating: h.ContentRating, Status: h.Status, Site: h.Site, DlsiteID: h.DlsiteID,
		})
	}
	return &searchWorksOutput{Body: okEnvelope(resp)}, nil
}

const (
	entityTypeLabel  = "label"
	labelDocIDPrefix = "b"
)

type searchInput struct {
	Q      string `query:"q" doc:"Search text; empty returns the most-credited entities"`
	Type   string `query:"type" enum:"names,characters,labels,works" doc:"Which entity index to search (works: wave 105 — LIVE galgame registry works, r18 verbatim on this face)"`
	Locale string `query:"locale" enum:"zh,ja,en" doc:"UI locale; the server pins the query language (client-supplied Meili locales are never accepted)"`
	Limit  int    `query:"limit" default:"20" doc:"Max hits (capped at 20)"`
}

type searchOutput struct {
	Body Envelope[dto.EntitySearchResponse]
}

func (s *S2SServer) searchEntities(ctx context.Context, in *searchInput) (*searchOutput, error) {
	uid, ok := catsearch.IndexForType(in.Type)
	if !ok {
		return nil, apiErrMsg(http.StatusBadRequest, errors.ErrInvalidParam, "type must be one of names|characters|labels|works")
	}
	limit := in.Limit
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	res, err := s.search.SearchEntities(ctx, uid, in.Q, catsearch.LocalesForUI(uid, in.Locale), limit, "")
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	logos, err := s.labelLogos(ctx, res.Hits)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	resp := dto.EntitySearchResponse{Total: res.Total}
	for _, d := range res.Hits {
		resp.Items = append(resp.Items, dto.EntitySearchHit{
			ID: d.ID, EntityType: d.EntityType, Name: d.Name(), Latin: d.Latin,
			Sources: d.Sources, Popularity: d.Popularity, Kind: d.Kind, PersonID: d.PersonID,
			ContentRating: d.ContentRating, LogoHash: logos[d.ID],
		})
	}
	return &searchOutput{Body: okEnvelope(resp)}, nil
}

func (s *S2SServer) labelLogos(ctx context.Context, hits []catsearch.EntityDoc) (map[string]string, error) {
	docIDs := make(map[int64]string, len(hits))
	ids := make([]int64, 0, len(hits))
	for _, d := range hits {
		if d.EntityType != entityTypeLabel {
			continue
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(d.ID, labelDocIDPrefix), 10, 64)
		if err != nil {
			continue
		}
		docIDs[id] = d.ID
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	byID, err := s.read.LabelLogoHashes(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(byID))
	for id, hash := range byID {
		out[docIDs[id]] = hash
	}
	return out, nil
}

type statsOutput struct {
	Body Envelope[dto.CatalogStats]
}

func (s *S2SServer) getStats(ctx context.Context, _ *struct{}) (*statsOutput, error) {
	o, err := s.stats.Overview(ctx)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	st := dto.CatalogStats{
		Works: dto.WorksMatrix{Total: o.WorksTotal},
		Entities: dto.EntityCounts{
			Persons: o.Entities.Persons, CreditNames: o.Entities.CreditNames,
			OrphanCreditNames: o.Entities.OrphanNames,
			Labels:            o.Entities.Labels, Characters: o.Entities.Characters,
		},
		Queues: dto.QueueLevels{ProbableRefs: o.ProbableRefs, Rejections: o.Rejections},
	}
	for _, c := range o.WorksCells {
		st.Works.Cells = append(st.Works.Cells, dto.WorksCell{MediumID: c.MediumID, Claimed: c.Claimed, Status: c.Status, Count: c.Count})
	}
	for _, c := range o.Credits {
		st.CreditsBySource = append(st.CreditsBySource, dto.KeyCount{Key: c.Key, Count: c.Count})
	}
	for _, c := range o.Attributions {
		st.AttributionsByKind = append(st.AttributionsByKind, dto.KindCount{Kind: c.Kind, Count: c.Count})
	}
	for _, c := range o.Anchors {
		st.AnchorsBySourceTier = append(st.AnchorsBySourceTier, dto.AnchorTierCell{Source: c.Source, LinkKind: c.LinkKind, Count: c.Count})
	}
	for _, c := range o.Candidates {
		st.Queues.CandidatesByStatus = append(st.Queues.CandidatesByStatus, dto.StatusCount{Status: c.Status, Count: c.Count})
	}
	for _, c := range o.Proposals {
		st.Queues.ProposalsByStatus = append(st.Queues.ProposalsByStatus, dto.StatusCount{Status: c.Status, Count: c.Count})
	}
	for _, c := range o.LLMBid {
		st.LLMBidVerdicts = append(st.LLMBidVerdicts, dto.KeyCount{Key: c.Key, Count: c.Count})
	}
	for _, f := range o.Freshness {
		st.SourceFreshness = append(st.SourceFreshness, dto.SourceFreshness{Source: f.Source, LatestRef: f.LatestRef})
	}
	return &statsOutput{Body: okEnvelope(st)}, nil
}

type labelWorksInput struct {
	ID     int64 `path:"id" doc:"Catalog label id"`
	Limit  int   `query:"limit" default:"50" doc:"Page size (capped at 50)"`
	Offset int   `query:"offset" doc:"Rows to skip"`
}

type labelWorksOutput struct {
	Body Envelope[dto.LabelWorksResponse]
}

func (s *S2SServer) labelWorks(ctx context.Context, in *labelWorksInput) (*labelWorksOutput, error) {
	limit := in.Limit
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	head, items, total, err := s.read.LabelWorks(ctx, in.ID, limit, offset)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	if head == nil {
		return nil, apiErr(http.StatusNotFound, errors.ErrNotFound)
	}
	resp := dto.LabelWorksResponse{Total: total}
	resp.Label = &dto.LabelHead{ID: head.ID, DisplayName: head.DisplayName, Kind: head.Kind, LogoHash: head.LogoHash}
	for _, w := range items {
		resp.Items = append(resp.Items, dto.LabelWorkRow{
			WorkID: w.WorkID, DisplayName: w.DisplayName, MediumID: w.MediumID,
			ContentRating: w.ContentRating, Status: w.Status, Kind: w.Kind,
		})
	}
	return &labelWorksOutput{Body: okEnvelope(resp)}, nil
}

type nameWorksInput struct {
	ID     int64 `path:"id" doc:"Catalog credit-name id"`
	Limit  int   `query:"limit" default:"50" doc:"Page size (capped at 50)"`
	Offset int   `query:"offset" doc:"Rows to skip"`
}

type nameWorksOutput struct {
	Body Envelope[dto.NameWorksResponse]
}

func (s *S2SServer) nameWorks(ctx context.Context, in *nameWorksInput) (*nameWorksOutput, error) {
	limit, offset := pageParams(in.Limit, in.Offset)
	res, err := s.read.NameWorks(ctx, in.ID, limit, offset)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	if res.Head == nil {
		return nil, apiErr(http.StatusNotFound, errors.ErrNotFound)
	}
	resp := dto.NameWorksResponse{
		Name: dto.NameHead{
			ID:          res.Head.ID,
			DisplayName: res.Head.Name,
			Lang:        res.Head.Lang,
			Latin:       derefStr(res.Head.Latin),
			Siblings:    make([]dto.SiblingName, 0, len(res.Siblings)),
			PhotoHash:   res.Head.PhotoHash,
			Gender:      res.Head.Gender,
			BirthY:      res.Head.BirthY,
			BirthM:      res.Head.BirthM,
			BirthD:      res.Head.BirthD,
		},
		Items: make([]dto.NameWorkRow, 0, len(res.Works)),
		Total: res.Total,
	}
	if res.Head.PersonID != nil {
		resp.Name.PersonID = *res.Head.PersonID
	}
	for _, sib := range res.Siblings {
		resp.Name.Siblings = append(resp.Name.Siblings, dto.SiblingName{
			ID: sib.ID, DisplayName: sib.Name, Lang: sib.Lang, Latin: derefStr(sib.Latin),
		})
	}
	for _, w := range res.Works {
		row := dto.NameWorkRow{Work: workBriefDTO(w.Brief), Roles: make([]dto.NameWorkRole, 0, len(w.Roles))}
		for _, r := range w.Roles {
			nr := dto.NameWorkRole{
				RoleID: r.RoleID, RoleKey: r.RoleKey,
				RoleName: firstNonEmpty(r.RoleNameCN, r.RoleNameJA, r.RoleKey),
				Identity: r.Identity,
			}
			if r.CharacterID != nil {
				nr.CharacterID = *r.CharacterID
			}
			if r.CharacterNM != nil {
				nr.Character = *r.CharacterNM
			}
			row.Roles = append(row.Roles, nr)
		}
		resp.Items = append(resp.Items, row)
	}
	return &nameWorksOutput{Body: okEnvelope(resp)}, nil
}

type characterWorksInput struct {
	ID     int64 `path:"id" doc:"Catalog character id"`
	Limit  int   `query:"limit" default:"50" doc:"Page size (capped at 50)"`
	Offset int   `query:"offset" doc:"Rows to skip"`
}

type characterWorksOutput struct {
	Body Envelope[dto.CharacterWorksResponse]
}

func (s *S2SServer) characterWorks(ctx context.Context, in *characterWorksInput) (*characterWorksOutput, error) {
	limit, offset := pageParams(in.Limit, in.Offset)
	res, err := s.read.CharacterWorks(ctx, in.ID, limit, offset)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	if res.Head == nil {
		return nil, apiErr(http.StatusNotFound, errors.ErrNotFound)
	}
	resp := dto.CharacterWorksResponse{
		Character: dto.CharacterHead{
			ID: res.Head.ID, DisplayName: res.Head.DisplayName, Lang: res.Head.Lang, Latin: derefStr(res.Head.Latin),
		},
		Items: make([]dto.CharacterWorkRow, 0, len(res.Works)),
		Total: res.Total,
	}
	for _, w := range res.Works {
		row := dto.CharacterWorkRow{
			Work: workBriefDTO(w.Brief), Kind: w.Kind, Spoiler: w.Spoiler,
			Identity: w.Identity, Voiced: w.Voiced,
			Voices: make([]dto.VoiceName, 0, len(w.Voices)),
		}
		for _, v := range w.Voices {
			row.Voices = append(row.Voices, dto.VoiceName{
				CreditNameID: v.CreditNameID, Name: v.Name, Lang: v.Lang, Latin: derefStr(v.Latin),
			})
		}
		resp.Items = append(resp.Items, row)
	}
	return &characterWorksOutput{Body: okEnvelope(resp)}, nil
}

type characterByIDInput struct {
	ID       int64 `path:"id" doc:"Catalog character id"`
	Spoilers int16 `query:"spoilers" doc:"max trait spoiler level to include (0-2, default 0)"`
}

type characterByIDOutput struct {
	Body Envelope[dto.CharacterDetailResponse]
}

func (s *S2SServer) characterByID(ctx context.Context, in *characterByIDInput) (*characterByIDOutput, error) {
	maxSpoiler := in.Spoilers
	if maxSpoiler < 0 {
		maxSpoiler = 0
	}
	if maxSpoiler > 2 {
		maxSpoiler = 2
	}
	detail, err := s.read.CharacterByID(ctx, in.ID, maxSpoiler)
	if err != nil {
		return nil, apiErr(http.StatusInternalServerError, errors.ErrInternalServer)
	}
	if detail == nil {
		return nil, apiErr(http.StatusNotFound, errors.ErrNotFound)
	}
	resp := dto.CharacterDetailResponse{
		ID: detail.ID, DisplayName: detail.DisplayName, Latin: derefStr(detail.Latin),
		Lang: detail.Lang, Gender: derefI16(detail.Gender), Description: detail.Description,
		InstanceOf: derefI64(detail.InstanceOf), ImageHash: derefStr(detail.ImageHash),
		FigureHash:    derefStr(detail.FigureHash),
		BirthdayMonth: derefI16(detail.BirthdayMonth), BirthdayDay: derefI16(detail.BirthdayDay),
		BloodType: derefI16(detail.BloodType), HeightCm: derefI16(detail.HeightCm),
		WeightKg: derefI16(detail.WeightKg), BustCm: derefI16(detail.BustCm),
		WaistCm: derefI16(detail.WaistCm), HipCm: derefI16(detail.HipCm), Cup: derefStr(detail.Cup),
		Extra: detail.Extra, AttrSources: detail.AttrSources,
		Aliases: make([]dto.CharacterAlias, 0, len(detail.Aliases)),
		Intros:  make([]dto.WorkIntro, 0, len(detail.Intros)),
		Traits:  make([]dto.CharacterTrait, 0, len(detail.Traits)),
	}
	for _, a := range detail.Aliases {
		resp.Aliases = append(resp.Aliases, dto.CharacterAlias{
			ID: a.ID, Name: a.Name, Latin: derefStr(a.Latin), Lang: a.Lang,
			Kind: a.Kind, IsPrimaryForLocale: a.IsPrimaryForLocale,
		})
	}
	for _, in := range detail.Intros {
		resp.Intros = append(resp.Intros, dto.WorkIntro{Lang: in.Lang, Intro: in.Intro, SourceID: in.SourceID, Machine: in.Machine})
	}
	for _, tr := range detail.Traits {
		resp.Traits = append(resp.Traits, dto.CharacterTrait{
			ID: tr.ID, Name: tr.Name, NameZh: tr.NameZh,
			GroupTID: tr.GroupTID, GroupName: derefStr(tr.GroupName), GroupNameZh: derefStr(tr.GroupNameZh),
			Sexual: tr.Sexual, SpoilerLevel: tr.SpoilerLevel, Lie: tr.Lie,
		})
	}
	return &characterByIDOutput{Body: okEnvelope(resp)}, nil
}

func pageParams(limit, offset int) (int, int) {
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func workBriefDTO(b service.WorkBriefRow) dto.WorkBrief {
	return dto.WorkBrief{
		WorkID: b.WorkID, DisplayName: b.DisplayName, MediumID: b.MediumID,
		ContentRating: b.ContentRating, Status: b.Status, Site: b.Site,
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefI16(p *int16) int16 {
	if p == nil {
		return 0
	}
	return *p
}

func derefI64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
