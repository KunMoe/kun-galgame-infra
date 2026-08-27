package handler

import (
	"encoding/json"
	stderrors "errors"
	"strconv"
	"strings"
	"time"

	"api/internal/platform/catalog/dto"
	"api/internal/platform/catalog/model"
	catsearch "api/internal/platform/catalog/search"
	"api/internal/platform/catalog/service"
	"api/pkg/errors"
	"api/pkg/response"

	"github.com/gofiber/fiber/v3"
)

type PublicHandler struct {
	svc     *service.PublicService
	resolve *service.ResolveService
	search  *catsearch.Indexer
	stats   *service.StatsService
	clients OAuthClientLookup
}

func (h *PublicHandler) WithModeration(clients OAuthClientLookup) *PublicHandler {
	h.clients = clients
	return h
}

func NewPublicHandler(svc *service.PublicService, resolve *service.ResolveService, searcher *catsearch.Indexer, stats *service.StatsService) *PublicHandler {
	return &PublicHandler{svc: svc, resolve: resolve, search: searcher, stats: stats}
}

const (
	cacheDetail    = "public, max-age=0, s-maxage=300, stale-while-revalidate=60"
	cacheSearch    = "public, max-age=0, s-maxage=60, stale-while-revalidate=60"
	cacheRedirects = "public, max-age=0, s-maxage=30, stale-while-revalidate=30"
	// The pending lane varies on the MODERATOR's own token, not just on the
	// URL a shared cache keys by, so one tenant's queue could be handed to the
	// next tenant that asked for the same URL.
	cacheModeration = "private, no-store"

	msgBadLimit = "limit must be a positive integer"

	msgBadIDsCursor = "ids does not paginate; do not also pass cursor"

	msgBadLookupType = "type must be one of work, name, character, label"
)

func (h *PublicHandler) WorkDetail(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	sel := fieldsQuery(c)
	rec, found, err := h.svc.WorkDetail(c.Context(), id,
		service.ParsePublicInclude(c.Query("include")), nsfwQuery(c), spoilersQuery(c), sel)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return response.NotFound(c, errors.ErrNotFound)
	}
	c.Set("Cache-Control", cacheDetail)
	return successProjected(c, rec, sel, sel.ProjectObject)
}

func (h *PublicHandler) Lookup(c fiber.Ctx) error {
	source := strings.TrimSpace(c.Query("source"))
	externalID := c.Query("external_id")
	if source == "" || strings.TrimSpace(externalID) == "" {
		return response.BadRequestMsg(c, errors.ErrBadRequest, "source and external_id are required")
	}
	entityType, _, ok := service.ParsePublicLookupType(c.Query("type"))
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLookupType)
	}
	data, found, err := h.svc.LookupTyped(c.Context(), source, externalID, entityType, nsfwQuery(c))
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return response.NotFound(c, errors.ErrNotFound)
	}
	c.Set("Cache-Control", cacheDetail)
	return response.Success(c, data)
}

func (h *PublicHandler) LookupBatch(c fiber.Ctx) error {
	var req dto.PublicLookupBatchRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequestMsg(c, errors.ErrBadRequest, "malformed body")
	}
	if len(req.Items) == 0 {
		return response.BadRequestMsg(c, errors.ErrBadRequest, "items is required (1-100 pairs)")
	}
	if len(req.Items) > 100 {
		return response.BadRequestMsg(c, errors.ErrValidationFailed, "at most 100 pairs per batch")
	}
	for _, p := range req.Items {
		if _, _, ok := service.ParsePublicLookupType(p.Type); !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLookupType)
		}
	}
	items, err := h.svc.LookupBatch(c.Context(), req.Items, nsfwQuery(c))
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	return response.Success(c, dto.PublicLookupBatchData{Items: items})
}

func (h *PublicHandler) Resolve(c fiber.Ctx) error {
	var req dto.PublicResolveRequest
	if err := c.Bind().JSON(&req); err != nil {
		return response.BadRequestMsg(c, errors.ErrBadRequest, "malformed body")
	}
	et, ok := entityTypeFromKey(req.EntityType)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "unknown entity_type")
	}
	if len(req.IDs) > 1000 {
		return response.BadRequestMsg(c, errors.ErrValidationFailed, "at most 1000 ids per call")
	}
	mappings, err := h.resolve.ResolveBatch(c.Context(), et, req.IDs)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	out := dto.PublicResolveData{Mappings: make(map[string]int64, len(mappings))}
	for old, canonical := range mappings {
		out.Mappings[strconv.FormatInt(old, 10)] = canonical
		if old != canonical {
			out.Redirected = append(out.Redirected, old)
		}
	}
	return response.Success(c, out)
}

func (h *PublicHandler) Redirects(c fiber.Ctx) error {
	cursor, err := decodeRedirectCursor(c.Query("cursor"))
	if err != nil {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "malformed cursor")
	}
	var filter *int16
	if k := c.Query("entity_type"); k != "" {
		et, ok := entityTypeFromKey(k)
		if !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "unknown entity_type")
		}
		filter = &et
	}
	limit := atoiOrPub(c.Query("limit"), 0)
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	items, next, err := h.resolve.RedirectsSince(c.Context(), filter, cursor, limit)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	out := dto.PublicRedirectsData{Items: make([]dto.PublicRedirectItem, len(items))}
	for i, it := range items {
		out.Items[i] = dto.PublicRedirectItem{
			EntityType: entityTypeKey(it.EntityType), OldID: it.OldID,
			CurrentID: it.CurrentID, MergedAt: it.MergedAt,
		}
	}
	if len(items) == limit {
		nc := encodeRedirectCursor(next)
		out.NextCursor = &nc
	}
	c.Set("Cache-Control", cacheRedirects)
	return response.Success(c, out)
}

func (h *PublicHandler) Name(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	limit, offset, ok := pagePub(c)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	rec, found, err := h.svc.Name(c.Context(), id, service.ParsePublicInclude(c.Query("include")).Credits, nsfwQuery(c), limit, offset)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return h.missOrMoved(c, model.EntityTypeCreditName, "names", id)
	}
	c.Set("Cache-Control", cacheDetail)
	return response.Success(c, rec)
}

func (h *PublicHandler) Character(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	limit, offset, ok := pagePub(c)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	rec, found, err := h.svc.Character(c.Context(), id, service.ParsePublicInclude(c.Query("include")).Works, nsfwQuery(c), spoilersQuery(c), limit, offset)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return h.missOrMoved(c, model.EntityTypeCharacter, "characters", id)
	}
	c.Set("Cache-Control", cacheDetail)
	return response.Success(c, rec)
}

func (h *PublicHandler) Label(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	limit, offset, ok := pagePub(c)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	rec, found, err := h.svc.Label(c.Context(), id, service.ParsePublicInclude(c.Query("include")).Works, nsfwQuery(c), limit, offset)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return h.missOrMoved(c, model.EntityTypeLabel, "labels", id)
	}
	c.Set("Cache-Control", cacheDetail)
	return response.Success(c, rec)
}

func (h *PublicHandler) missOrMoved(c fiber.Ctx, entityType int16, lane string, id int64) error {
	current, moved, err := h.resolve.Resolve(c.Context(), entityType, id)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !moved {
		return response.NotFound(c, errors.ErrNotFound)
	}
	c.Set("Location", "/v1/catalog/"+lane+"/"+strconv.FormatInt(current, 10))
	c.Set("Cache-Control", cacheDetail)
	return c.Status(fiber.StatusMovedPermanently).JSON(response.Response{
		Code:    errors.ErrMoved,
		Message: "this id was merged away; use current_id",
		Data: dto.PublicMovedData{
			EntityType: entityTypeKey(entityType), ID: id, CurrentID: current,
		},
	})
}

func (h *PublicHandler) Search(c fiber.Ctx) error {
	uid, entityType, ok := publicSearchIndex(c.Query("type"))
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "type must be one of names|characters|labels|works|tags")
	}
	limit := atoiOrPub(c.Query("limit"), 20)
	if limit <= 0 || limit > 20 {
		limit = 20
	}
	filter := ""
	if entityType == "work" && !nsfwQuery(c) {
		filter = "content_rating != " + strconv.Itoa(int(model.ContentRatingR18))
	}
	res, err := h.search.SearchEntities(c.Context(), uid, c.Query("q"), catsearch.LocalesForUI(uid, c.Query("locale")), limit, filter)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	out := dto.PublicEntitySearchData{Total: res.Total, Items: make([]dto.PublicEntityHit, 0, len(res.Hits))}
	for _, d := range res.Hits {
		id, ok := stripEntityPrefix(d.ID)
		if !ok {
			continue
		}
		hit := dto.PublicEntityHit{
			ID: id, EntityType: entityType, DisplayName: d.Name(), Latin: d.Latin, Sources: d.Sources,
		}
		if d.ContentRating != nil {
			hit.ContentRating = publicContentRatingKey(*d.ContentRating)
		}
		if entityType == "tag" {
			hit.Tier = publicTagTierKey(derefI16(d.Tier))
			hit.Kind = publicTagKindKey(derefI16(d.Kind))
		}
		out.Items = append(out.Items, hit)
	}
	ids := make([]int64, len(out.Items))
	for i := range out.Items {
		ids[i] = out.Items[i].ID
	}
	switch entityType {
	case "work":
		blocks, err := h.svc.WorkNamesByID(c.Context(), ids)
		if err != nil {
			return response.InternalError(c, errors.ErrInternalServer)
		}
		for i := range out.Items {
			out.Items[i].Localized = blocks[out.Items[i].ID].Localized
		}
	case "name", "character", "label":
		loc, err := h.svc.LocalizedForEntities(c.Context(), entityType, ids)
		if err != nil {
			return response.InternalError(c, errors.ErrInternalServer)
		}
		for i := range out.Items {
			out.Items[i].Localized = loc[out.Items[i].ID]
		}
	}
	c.Set("Cache-Control", cacheSearch)
	return response.Success(c, out)
}

func nsfwQuery(c fiber.Ctx) bool {
	return boolQueryPub(c.Query("nsfw"))
}

func fieldsQuery(c fiber.Ctx) service.PublicFields {
	return service.ParsePublicFields(c.Query("fields"))
}

// successProjected keeps the unprojected path on the ORIGINAL typed value: a
// map/bytes round trip on every response is how key order and number
// formatting drift away from a published contract nobody meant to change.
func successProjected(c fiber.Ctx, data any, sel service.PublicFields, project func([]byte) ([]byte, error)) error {
	if !sel.Active() {
		return response.Success(c, data)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	trimmed, err := project(raw)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	return response.Success(c, json.RawMessage(trimmed))
}

func boolQueryPub(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

func spoilersQuery(c fiber.Ctx) int16 {
	switch c.Query("spoilers") {
	case "1":
		return 1
	case "2":
		return 2
	}
	return 0
}

func entityTypeKey(t int16) string {
	switch t {
	case model.EntityTypePerson:
		return "person"
	case model.EntityTypeCreditName:
		return "name"
	case model.EntityTypeLabel:
		return "label"
	case model.EntityTypeCharacter:
		return "character"
	case model.EntityTypeWork:
		return "work"
	case model.EntityTypeRelease:
		return "release"
	default:
		return ""
	}
}

func entityTypeFromKey(k string) (int16, bool) {
	switch k {
	case "person":
		return model.EntityTypePerson, true
	case "name":
		return model.EntityTypeCreditName, true
	case "label":
		return model.EntityTypeLabel, true
	case "character":
		return model.EntityTypeCharacter, true
	case "work":
		return model.EntityTypeWork, true
	case "release":
		return model.EntityTypeRelease, true
	default:
		return 0, false
	}
}

func publicSearchIndex(t string) (uid, entityType string, ok bool) {
	switch t {
	case "names":
		uid, ok = catsearch.IndexForType("names")
		return uid, "name", ok
	case "characters":
		uid, ok = catsearch.IndexForType("characters")
		return uid, "character", ok
	case "labels":
		uid, ok = catsearch.IndexForType("labels")
		return uid, "label", ok
	case "works":
		uid, ok = catsearch.IndexForType("works")
		return uid, "work", ok
	case "tags":
		uid, ok = catsearch.IndexForType("tags")
		return uid, "tag", ok
	default:
		return "", "", false
	}
}

func publicTagTierKey(t int16) string {
	switch t {
	case model.TagTierLongtail:
		return "longtail"
	case model.TagTierHidden:
		return "hidden"
	default:
		return "core"
	}
}

func publicTagKindKey(k int16) string {
	if k == model.TagKindMeta {
		return "meta"
	}
	return "content"
}

func stripEntityPrefix(id string) (int64, bool) {
	if len(id) < 2 {
		return 0, false
	}
	n, err := strconv.ParseInt(id[1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func pagePub(c fiber.Ctx) (limit, offset int, ok bool) {
	limit, ok = limitPub(c.Query("limit"), 50, 50)
	if !ok {
		return 0, 0, false
	}
	offset = atoiOrPub(c.Query("offset"), 0)
	if offset < 0 {
		offset = 0
	}
	return limit, offset, true
}

func limitPub(raw string, def, max int) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, false
	}
	if n > max {
		return max, true
	}
	return n, true
}

func posIntQueryPub(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

const maxTagIDFilters = 10

const msgBadTagIDs = "tag_id must be up to 10 comma-separated positive integers"

const msgBadClaimState = "claim_state must be a comma-separated subset of none, live, draft, pending, declined, hidden"

func claimStatesPub(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		tok := strings.TrimSpace(p)
		if !service.IsWorksSearchClaimState(tok) {
			return nil, false
		}
		out = append(out, tok)
	}
	return out, true
}

const msgBadDisplayLimit = "content_limit must be a comma-separated subset of sfw, nsfw"

func displayLimitsPub(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		tok := strings.TrimSpace(p)
		if !service.IsDisplayLimit(tok) {
			return nil, false
		}
		out = append(out, tok)
	}
	return out, true
}

func posIntListQueryPub(raw string, max int) ([]int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	if len(parts) > max {
		return nil, false
	}
	out := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || n <= 0 {
			return nil, false
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out, true
}

func atoiOrPub(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

func publicContentRatingKey(r int16) string {
	switch r {
	case model.ContentRatingAllAges:
		return "all_ages"
	case model.ContentRatingSensitive:
		return "sensitive"
	case model.ContentRatingR18:
		return "r18"
	default:
		return ""
	}
}

func contentRatingFromKey(k string) (int16, bool) {
	switch k {
	case "all_ages":
		return model.ContentRatingAllAges, true
	case "sensitive":
		return model.ContentRatingSensitive, true
	case "r18":
		return model.ContentRatingR18, true
	default:
		return 0, false
	}
}

func datePub(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return 0, false
	}
	return int64(t.Year())*10000 + int64(t.Month())*100 + int64(t.Day()), true
}

func (h *PublicHandler) WorksList(c fiber.Ctx) error {
	f := service.WorksListFilter{
		NSFW:     nsfwQuery(c),
		Platform: strings.TrimSpace(c.Query("platform")),
		Include:  service.ParseWorksListInclude(c.Query("include")),
		Fields:   fieldsQuery(c),
		Site:     strings.TrimSpace(c.Query("site")),
		// The zero PublicOLang curates to ja+zh (the calendar's home
		// population). 2f326114 added the field for v2 and left this
		// constructor unset, which silently dropped every non-ja/zh work
		// from the v1 browse and ids= lanes — the forum saw short pages,
		// and works?ids= contradicted works/search. v1 declares no olang=
		// parameter: this lane is always the whole population.
		OLang: service.PublicOLang{All: true},
	}
	switch sort := c.Query("sort"); sort {
	case "", "id":
		f.Sort = "id"
	case "updated":
		f.Sort = "updated"
	default:
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "sort must be id|updated")
	}
	if cr := c.Query("content_rating"); cr != "" {
		v, ok := contentRatingFromKey(cr)
		if !ok {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "content_rating must be all_ages|sensitive|r18")
		}
		if v == model.ContentRatingR18 && !f.NSFW {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "content_rating=r18 requires nsfw=1")
		}
		f.ContentRating = &v
	}
	if cl := c.Query("claimed"); cl != "" {
		switch strings.ToLower(strings.TrimSpace(cl)) {
		case "1", "true", "yes":
			t := true
			f.Claimed = &t
		case "0", "false", "no":
			v := false
			f.Claimed = &v
		default:
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "claimed must be true|false")
		}
	}
	var ok bool
	if f.ClaimStates, ok = claimStatesPub(c.Query("claim_state")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadClaimState)
	}
	if f.DisplayLimits, ok = displayLimitsPub(c.Query("content_limit")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadDisplayLimit)
	}
	if refusal := h.applyWorksStatus(c, &f); refusal != nil {
		if refusal.status == fiber.StatusBadRequest {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, refusal.msg)
		}
		return response.ForbiddenMsg(c, errors.ErrForbidden, refusal.msg)
	}
	if f.LabelID, ok = posIntQueryPub(c.Query("label_id")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "label_id must be a positive integer")
	}
	f.LabelRollup = boolQueryPub(c.Query("label_rollup"))
	if f.TagIDs, ok = posIntListQueryPub(c.Query("tag_id"), maxTagIDFilters); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadTagIDs)
	}
	if f.SeriesID, ok = posIntQueryPub(c.Query("series_id")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "series_id must be a positive integer")
	}
	if f.EngineID, ok = posIntQueryPub(c.Query("engine_id")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "engine_id must be a positive integer")
	}
	if f.ReleasedAfter, ok = datePub(c.Query("released_after")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "released_after must be YYYY-MM-DD")
	}
	if f.ReleasedBefore, ok = datePub(c.Query("released_before")); !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "released_before must be YYYY-MM-DD")
	}
	if raw := strings.TrimSpace(c.Query("ids")); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) > 100 {
			return response.BadRequestMsg(c, errors.ErrValidationFailed, "at most 100 ids")
		}
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil || id <= 0 {
				return response.BadRequestMsg(c, errors.ErrInvalidParam, "ids must be positive integers")
			}
			f.IDs = append(f.IDs, id)
		}
	}
	limit, ok := limitPub(c.Query("limit"), 20, 100)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	if len(f.IDs) > 0 && c.Query("cursor") != "" {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadIDsCursor)
	}
	data, err := h.svc.WorksList(c.Context(), f, c.Query("cursor"), limit)
	if err != nil {
		if stderrors.Is(err, service.ErrBadCursor) {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "malformed cursor")
		}
		return response.InternalError(c, errors.ErrInternalServer)
	}
	c.Set("Cache-Control", worksListCacheControl(c))
	return successProjected(c, data, f.Fields, f.Fields.ProjectItems)
}

func worksListCacheControl(c fiber.Ctx) string {
	if strings.TrimSpace(c.Query("status")) == worksStatusPending {
		return cacheModeration
	}
	return cacheSearch
}

func (h *PublicHandler) Changes(c fiber.Ctx) error {
	if et := c.Query("entity_type"); et != "" && et != "work" {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, "entity_type must be work (the v1 feed scope)")
	}
	limit, ok := limitPub(c.Query("limit"), 100, 500)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	data, err := h.svc.Changes(c.Context(), c.Query("cursor"), limit)
	if err != nil {
		if stderrors.Is(err, service.ErrBadCursor) {
			return response.BadRequestMsg(c, errors.ErrInvalidParam, "malformed cursor")
		}
		return response.InternalError(c, errors.ErrInternalServer)
	}
	c.Set("Cache-Control", cacheRedirects)
	return response.Success(c, data)
}

func (h *PublicHandler) Tag(c fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return response.BadRequest(c, errors.ErrInvalidID)
	}
	limit, offset, ok := pagePub(c)
	if !ok {
		return response.BadRequestMsg(c, errors.ErrInvalidParam, msgBadLimit)
	}
	rec, found, err := h.svc.TagDetail(c.Context(), id, service.ParsePublicInclude(c.Query("include")).Works, nsfwQuery(c), limit, offset)
	if err != nil {
		return response.InternalError(c, errors.ErrInternalServer)
	}
	if !found {
		return response.NotFound(c, errors.ErrNotFound)
	}
	c.Set("Cache-Control", cacheDetail)
	return response.Success(c, rec)
}
