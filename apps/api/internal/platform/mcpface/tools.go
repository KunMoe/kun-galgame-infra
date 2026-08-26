package mcpface

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true}

func registerTools(s *mcp.Server, up *Upstream) {
	t := &tools{up: up}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_search",
		Title:       "Search catalog entities",
		Description: descCatalogSearch,
		Annotations: readOnly,
	}, t.catalogSearch)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_get",
		Title:       "Get a catalog work by id",
		Description: descCatalogWorkGet,
		Annotations: readOnly,
	}, t.catalogWorkGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_lookup_external",
		Title:       "Look up a work by external id",
		Description: descCatalogLookup,
		Annotations: readOnly,
	}, t.catalogLookupExternal)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_name_get",
		Title:       "Get a catalog credited name by id",
		Description: descCatalogNameGet,
		Annotations: readOnly,
	}, t.catalogNameGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_label_get",
		Title:       "Get a catalog label by id",
		Description: descCatalogLabelGet,
		Annotations: readOnly,
	}, t.catalogLabelGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_character_get",
		Title:       "Get a catalog character by id",
		Description: descCatalogCharacterGet,
		Annotations: readOnly,
	}, t.catalogCharacterGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_works_list",
		Title:       "Browse / filter catalog works",
		Description: descCatalogWorksList,
		Annotations: readOnly,
	}, t.catalogWorksList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_changes",
		Title:       "Poll the catalog change feed",
		Description: descCatalogChanges,
		Annotations: readOnly,
	}, t.catalogChanges)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_tag_get",
		Title:       "Get a canonical tag by id",
		Description: descCatalogTagGet,
		Annotations: readOnly,
	}, t.catalogTagGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_works_search",
		Title:       "Search catalog works (product search)",
		Description: descCatalogWorksSearch,
		Annotations: readOnly,
	}, t.catalogWorksSearch)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_calendar",
		Title:       "Release calendar for one month",
		Description: descCatalogCalendar,
		Annotations: readOnly,
	}, t.catalogCalendar)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_calendar_pending",
		Title:       "Release calendar, month-unknown bucket",
		Description: descCatalogCalendarPending,
		Annotations: readOnly,
	}, t.catalogCalendarPending)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_calendar_tba",
		Title:       "Release calendar, undated bucket",
		Description: descCatalogCalendarTBA,
		Annotations: readOnly,
	}, t.catalogCalendarTBA)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_labels_list",
		Title:       "Browse catalog labels",
		Description: descCatalogLabelsList,
		Annotations: readOnly,
	}, t.catalogLabelsList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_tags_list",
		Title:       "Browse canonical tags",
		Description: descCatalogTagsList,
		Annotations: readOnly,
	}, t.catalogTagsList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_engines_list",
		Title:       "Browse catalog engines",
		Description: descCatalogEnginesList,
		Annotations: readOnly,
	}, t.catalogEnginesList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_engine_get",
		Title:       "Get a catalog engine by id",
		Description: descCatalogEngineGet,
		Annotations: readOnly,
	}, t.catalogEngineGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_series_list",
		Title:       "Browse catalog series",
		Description: descCatalogSeriesList,
		Annotations: readOnly,
	}, t.catalogSeriesList)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_series_get",
		Title:       "Get a catalog series by id",
		Description: descCatalogSeriesGet,
		Annotations: readOnly,
	}, t.catalogSeriesGet)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_stats",
		Title:       "Catalogue-wide counts",
		Description: descCatalogStats,
		Annotations: readOnly,
	}, t.catalogStats)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_label_relation_graph",
		Title:       "Corporate family around a label",
		Description: descCatalogLabelRelationGraph,
		Annotations: readOnly,
	}, t.catalogLabelRelationGraph)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_releases",
		Title:       "Release-grain timeline",
		Description: descCatalogReleases,
		Annotations: readOnly,
	}, t.catalogReleases)

	registerWorkSubresourceTools(s, t)
	registerNewsTools(s, t)
}

const descCatalogSearch = "Search the cross-media identity registry for entities by name. Choose the index with `type`: " +
	"names (creator credit-names / persons), characters, labels (brands / doujin circles), or works " +
	"(work titles across media; r18 hits excluded unless nsfw=true). Use this for " +
	"NATURAL-LANGUAGE lookup; when you already hold an external id use catalog_lookup_external, and to fetch a " +
	"work's full registry row use catalog_work_get."

type catalogSearchInput struct {
	Type   string `json:"type" jsonschema:"Which entity index to search (required): one of names, characters, labels, or works."`
	Q      string `json:"q,omitempty" jsonschema:"Relevance query over the entity's names."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Max hits (default 20, hard cap 20)."`
	Locale string `json:"locale,omitempty" jsonschema:"UI locale pinning the query language: zh, ja, or en."`
	Nsfw   bool   `json:"nsfw,omitempty" jsonschema:"works only: true = include r18 hits (default false = excluded server-side)."`
}

func (t *tools) catalogSearch(ctx context.Context, req *mcp.CallToolRequest, in catalogSearchInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "type", in.Type)
	setStr(q, "q", in.Q)
	setInt(q, "limit", in.Limit)
	setStr(q, "locale", in.Locale)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_search", "/v1/catalog/search", q)
}

const descCatalogWorkGet = "Fetch one catalog work's registry row by its numeric catalog work id: the canonical cross-media " +
	"identity plus external-source anchors. Pass include=credits and/or include=relations to embed the credited " +
	"staff/cast and the linked works in the same response."

type catalogWorkGetInput struct {
	ID      int    `json:"id" jsonschema:"The catalog work id (required)."`
	Include string `json:"include,omitempty" jsonschema:"Comma-separated extra blocks: credits and/or relations. Omit for the bare registry row."`
	Nsfw    bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works and r18 relation ends (default false = hidden)."`
}

func (t *tools) catalogWorkGet(ctx context.Context, req *mcp.CallToolRequest, in catalogWorkGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "include", in.Include)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_work_get", pathID("/v1/catalog/works", in.ID), q)
}

const descCatalogLookup = "Reverse-look up a catalog work by an EXTERNAL id — the go-to tool when you already have an id " +
	"from another database. Give the source and its id (e.g. source=vndb, external_id=v19658); returns the matched " +
	"work plus claim pointers. Prefer this over any search when an external id is available."

type catalogLookupInput struct {
	Source     string `json:"source" jsonschema:"External source key (required): vndb, bangumi, dlsite, or erogamescape."`
	ExternalID string `json:"external_id" jsonschema:"The id within that source (required), e.g. v19658 for VNDB."`
	Nsfw       bool   `json:"nsfw,omitempty" jsonschema:"true = resolve r18 works too (default false = 404 on an r18 hit)."`
}

func (t *tools) catalogLookupExternal(ctx context.Context, req *mcp.CallToolRequest, in catalogLookupInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "source", in.Source)
	setStr(q, "external_id", in.ExternalID)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_lookup_external", "/v1/catalog/lookup", q)
}

const descCatalogNameGet = "Fetch one credited name (creator identity) by its numeric id: localized intros[] (creator " +
	"bio; each entry carries source and machine=true when it is a machine translation), siblings[] (same-person " +
	"alternate names) and external refs. Pass include=credits to attach the works this name is credited on, with roles."

type catalogNameGetInput struct {
	ID      int    `json:"id" jsonschema:"The name id (required). Find one with catalog_search type=names."`
	Include string `json:"include,omitempty" jsonschema:"Set to credits to attach the works this name is credited on."`
	Nsfw    bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works among the credits (default false = hidden)."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max attached credits (default 50, cap 50)."`
	Offset  int    `json:"offset,omitempty" jsonschema:"Offset into the attached credits list."`
}

func (t *tools) catalogNameGet(ctx context.Context, req *mcp.CallToolRequest, in catalogNameGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "include", in.Include)
	setBool(q, "nsfw", in.Nsfw)
	setInt(q, "limit", in.Limit)
	setInt(q, "offset", in.Offset)
	return t.run(ctx, req, "catalog_name_get", pathID("/v1/catalog/names", in.ID), q)
}

const descCatalogLabelGet = "Fetch one label (brand / doujin circle) by its numeric id: display names, intros[] and links[]. " +
	"Pass include=works to attach the works attributed to the label."

type catalogLabelGetInput struct {
	ID      int    `json:"id" jsonschema:"The label id (required)."`
	Include string `json:"include,omitempty" jsonschema:"Set to works to attach the works attributed to this label."`
	Nsfw    bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works among the attributions (default false = hidden)."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Max attached works (default 50, cap 50)."`
	Offset  int    `json:"offset,omitempty" jsonschema:"Offset into the attached works list."`
}

func (t *tools) catalogLabelGet(ctx context.Context, req *mcp.CallToolRequest, in catalogLabelGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "include", in.Include)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setInt(q, "offset", in.Offset)
	return t.run(ctx, req, "catalog_label_get", pathID("/v1/catalog/labels", in.ID), q)
}

const descCatalogCharacterGet = "Fetch one character by its numeric id: localized intros[] (bio), a portrait image URL, " +
	"and external refs. Traits are spoiler-gated: pass spoilers=1|2 to raise the max spoiler level (default " +
	"0 = safe). Pass include=works to attach the works the character appears in (with voice-actor names)."

type catalogCharacterGetInput struct {
	ID       int    `json:"id" jsonschema:"The character id (required)."`
	Include  string `json:"include,omitempty" jsonschema:"Set to works to attach the works the character appears in."`
	Nsfw     bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works and sexual-family traits (default false = both hidden)."`
	Spoilers int    `json:"spoilers,omitempty" jsonschema:"Max trait spoiler level 0-2 (default 0 = safe)."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max attached works (default 50, cap 50)."`
	Offset   int    `json:"offset,omitempty" jsonschema:"Offset into the attached works list."`
}

func (t *tools) catalogCharacterGet(ctx context.Context, req *mcp.CallToolRequest, in catalogCharacterGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "include", in.Include)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setInt(q, "spoilers", in.Spoilers)
	setInt(q, "offset", in.Offset)
	return t.run(ctx, req, "catalog_character_get", pathID("/v1/catalog/characters", in.ID), q)
}

const descCatalogWorksList = "Browse / filter the catalog works registry — the bulk lane. Filter by content_rating / " +
	"claimed / label_id / tag_id / series_id / platform / release window; ids=comma-list (max 100) batch-hydrates " +
	"known ids in one call. sort=id (stable browse, default) or updated (newest-updated first). Keyset-paginated: " +
	"pass the returned next_cursor to continue. For NATURAL-LANGUAGE title search use catalog_search type=works. " +
	"status=pending asks for the MODERATOR REVIEW QUEUE instead of the live set; it requires a SECOND credential " +
	"(the moderator's own Bearer access token alongside the API key) and is refused with 403 otherwise. THIS MCP " +
	"SERVER CARRIES ONE CREDENTIAL ONLY — the API key in the Bearer slot — so status=pending always refuses here; " +
	"the parameter exists so the refusal is the endpoint's own, not a silently dropped filter."

type catalogWorksListInput struct {
	ContentRating  string `json:"content_rating,omitempty" jsonschema:"Filter by rating: all_ages, sensitive, or r18 (r18 additionally requires nsfw=true)."`
	Claimed        string `json:"claimed,omitempty" jsonschema:"true = claimed works only; false = bodyless only; omit = both."`
	LabelID        int    `json:"label_id,omitempty" jsonschema:"Only works attributed to this label id."`
	TagID          int    `json:"tag_id,omitempty" jsonschema:"Only works carrying a source tag mapped to this canonical tag id."`
	SeriesID       int    `json:"series_id,omitempty" jsonschema:"Only member works of this series id."`
	Platform       string `json:"platform,omitempty" jsonschema:"vndb platform code, e.g. win, and, ios."`
	ReleasedAfter  string `json:"released_after,omitempty" jsonschema:"YYYY-MM-DD inclusive lower bound on the earliest release date per work."`
	ReleasedBefore string `json:"released_before,omitempty" jsonschema:"YYYY-MM-DD inclusive upper bound."`
	IDs            string `json:"ids,omitempty" jsonschema:"Comma-separated work ids (max 100) — batch-hydrate known ids in one call."`
	Sort           string `json:"sort,omitempty" jsonschema:"id = ascending browse order (default); updated = newest-updated first."`
	Cursor         string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit          int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw           bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works (default false = hidden)."`
	Status         string `json:"status,omitempty" jsonschema:"live (default) = the public live registry set; pending = the moderator review queue, which needs a second end-user credential this MCP transport cannot carry and therefore always 403s here."`
}

func (t *tools) catalogWorksList(ctx context.Context, req *mcp.CallToolRequest, in catalogWorksListInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "content_rating", in.ContentRating)
	setStr(q, "claimed", in.Claimed)
	setInt(q, "label_id", in.LabelID)
	setInt(q, "tag_id", in.TagID)
	setInt(q, "series_id", in.SeriesID)
	setStr(q, "platform", in.Platform)
	setStr(q, "released_after", in.ReleasedAfter)
	setStr(q, "released_before", in.ReleasedBefore)
	setStr(q, "ids", in.IDs)
	setStr(q, "sort", in.Sort)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "status", in.Status)
	return t.run(ctx, req, "catalog_works_list", "/v1/catalog/works", q)
}

const descCatalogChanges = "Poll the catalog change feed for INCREMENTAL SYNC: keyset-paginated entries for works " +
	"whose records changed, oldest first. Store the returned next_cursor and pass it on the next poll to receive " +
	"only what changed since. entity_type currently supports work (default)."

type catalogChangesInput struct {
	EntityType string `json:"entity_type,omitempty" jsonschema:"Feed scope: work (default)."`
	Cursor     string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor; omit to start from the beginning."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Items per page 1-500 (default 100)."`
}

func (t *tools) catalogChanges(ctx context.Context, req *mcp.CallToolRequest, in catalogChangesInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "entity_type", in.EntityType)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	return t.run(ctx, req, "catalog_changes", "/v1/catalog/changes", q)
}

const descCatalogTagGet = "Fetch one canonical tag (the cross-source tag vocabulary) by its numeric id. Pass " +
	"include=works to attach the works carrying any source tag mapped to it (limit/offset paginated)."

type catalogTagGetInput struct {
	ID      int    `json:"id" jsonschema:"The canonical tag id (required)."`
	Include string `json:"include,omitempty" jsonschema:"Set to works to attach works carrying any mapped source tag."`
	Nsfw    bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works among the attachments (default false = hidden)."`
	Limit   int    `json:"limit,omitempty" jsonschema:"Works per page 1-50 (default 50)."`
	Offset  int    `json:"offset,omitempty" jsonschema:"Rows to skip."`
}

func (t *tools) catalogTagGet(ctx context.Context, req *mcp.CallToolRequest, in catalogTagGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "include", in.Include)
	setBool(q, "nsfw", in.Nsfw)
	setInt(q, "limit", in.Limit)
	setInt(q, "offset", in.Offset)
	return t.run(ctx, req, "catalog_tag_get", pathID("/v1/catalog/tags", in.ID), q)
}

const descCatalogWorksSearch = "PRODUCT SEARCH over catalog works: free text plus the whole works-list filter set, " +
	"page-paginated, with five sort lanes and opt-in facet counts. Prefer this over catalog_search type=works " +
	"whenever the user combines a query with filters (rating / label / tag / engine / series / release window / " +
	"original language) or wants a ranked, faceted result set; catalog_search is the plain name index. A q that is " +
	"exactly a VNDB id (v19658) short-circuits to that one work. Empty q = a filter-only browse ordered by popularity."

type catalogWorksSearchInput struct {
	Q              string `json:"q,omitempty" jsonschema:"Free text over every indexed title / alias; empty = filter-only browse by popularity."`
	ContentRating  string `json:"content_rating,omitempty" jsonschema:"Filter by rating: all_ages, sensitive, or r18 (r18 additionally requires nsfw=true)."`
	Claimed        string `json:"claimed,omitempty" jsonschema:"true = claimed works only; false = bodyless only; omit = both."`
	ClaimState     string `json:"claim_state,omitempty" jsonschema:"Comma-separated CLOSED vocabulary none,live,draft,hidden matched as ANY-of; a product site rendering its own catalogue wants live. Unknown token = 400."`
	ContentLimit   string `json:"content_limit,omitempty" jsonschema:"Comma-separated CLOSED vocabulary sfw,nsfw — the EDITORIAL DISPLAY axis (is the material you would render safe to publish), NOT the age rating. Unknown token = 400."`
	LabelID        int    `json:"label_id,omitempty" jsonschema:"Only works attributed to this label id."`
	TagID          string `json:"tag_id,omitempty" jsonschema:"Up to 10 comma-separated canonical tag ids, ANDed (a work must carry all of them)."`
	SeriesID       int    `json:"series_id,omitempty" jsonschema:"Only member works of this series id."`
	EngineID       int    `json:"engine_id,omitempty" jsonschema:"Only works built with this engine id."`
	ReleasedAfter  string `json:"released_after,omitempty" jsonschema:"YYYY-MM-DD inclusive lower bound on the earliest release date per work."`
	ReleasedBefore string `json:"released_before,omitempty" jsonschema:"YYYY-MM-DD inclusive upper bound."`
	Olang          string `json:"olang,omitempty" jsonschema:"Original-language gate: comma-separated BCP-47 values (ja, zh-Hans, en, …) or all. Default = NO gate, unlike the calendar."`
	Sort           string `json:"sort,omitempty" jsonschema:"relevance (default), released_desc, released_asc, updated, or popularity."`
	Facets         string `json:"facets,omitempty" jsonschema:"Comma-separated CLOSED vocabulary: content_rating,olang,claimed,tag_id,label_id,engine_id,series_id,source. Counted over the same filtered set as total."`
	Page           int    `json:"page,omitempty" jsonschema:"1-based page number (default 1); a page past the end is empty."`
	Limit          int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw           bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works in items, total AND facets (default false = hidden)."`
	Include        string `json:"include,omitempty" jsonschema:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs."`
	SearchIntro    bool   `json:"search_intro,omitempty" jsonschema:"true = also match q against the work synopsis, not just titles/aliases (default false)."`
}

func (t *tools) catalogWorksSearch(ctx context.Context, req *mcp.CallToolRequest, in catalogWorksSearchInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "q", in.Q)
	setStr(q, "content_rating", in.ContentRating)
	setStr(q, "claimed", in.Claimed)
	setStr(q, "claim_state", in.ClaimState)
	setStr(q, "content_limit", in.ContentLimit)
	setInt(q, "label_id", in.LabelID)
	setStr(q, "tag_id", in.TagID)
	setInt(q, "series_id", in.SeriesID)
	setInt(q, "engine_id", in.EngineID)
	setStr(q, "released_after", in.ReleasedAfter)
	setStr(q, "released_before", in.ReleasedBefore)
	setStr(q, "olang", in.Olang)
	setStr(q, "sort", in.Sort)
	setStr(q, "facets", in.Facets)
	setInt(q, "page", in.Page)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "include", in.Include)
	setBool(q, "search_intro", in.SearchIntro)
	return t.run(ctx, req, "catalog_works_search", "/v1/catalog/works/search", q)
}

const descCatalogCalendar = "Release calendar for ONE ISO month, date ascending. Omit month to get the current " +
	"Asia/Tokyo month (the resolved month is echoed back). The olang gate defaults to the ja + zh* family — pass " +
	"olang=all for every language. Works whose month is not yet known live in catalog_calendar_pending, and " +
	"announced-but-undated ones in catalog_calendar_tba."

type catalogCalendarInput struct {
	Month        string `json:"month,omitempty" jsonschema:"ISO month YYYY-MM; omit for the current Asia/Tokyo month. A malformed value is a 400."`
	Olang        string `json:"olang,omitempty" jsonschema:"Comma-separated original languages in BCP-47 spelling (ja, zh-Hans, en, …) or all. Default = the ja + zh* family; an unknown value yields an empty bucket, never a 400."`
	ContentLimit string `json:"content_limit,omitempty" jsonschema:"Comma-separated CLOSED vocabulary sfw,nsfw — the editorial display axis, gating bucket membership and the count alike. Unknown token = 400."`
	Cursor       string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit        int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw         bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works (default false = hidden)."`
	Include      string `json:"include,omitempty" jsonschema:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs."`
}

func (t *tools) catalogCalendar(ctx context.Context, req *mcp.CallToolRequest, in catalogCalendarInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "month", in.Month)
	setStr(q, "olang", in.Olang)
	setStr(q, "content_limit", in.ContentLimit)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "include", in.Include)
	return t.run(ctx, req, "catalog_calendar", "/v1/catalog/calendar", q)
}

const descCatalogCalendarPending = "Release calendar bucket for works dated to a YEAR but whose MONTH is still " +
	"unknown, id ascending. Omit year for the current Asia/Tokyo year (echoed back). Same gates as catalog_calendar."

type catalogCalendarPendingInput struct {
	Year         string `json:"year,omitempty" jsonschema:"YYYY; omit for the current Asia/Tokyo year. A malformed value is a 400."`
	Olang        string `json:"olang,omitempty" jsonschema:"Comma-separated original languages or all. Default = the ja + zh* family."`
	ContentLimit string `json:"content_limit,omitempty" jsonschema:"Comma-separated CLOSED vocabulary sfw,nsfw (editorial display axis). Unknown token = 400."`
	Cursor       string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit        int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw         bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works (default false = hidden)."`
	Include      string `json:"include,omitempty" jsonschema:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs."`
}

func (t *tools) catalogCalendarPending(ctx context.Context, req *mcp.CallToolRequest, in catalogCalendarPendingInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "year", in.Year)
	setStr(q, "olang", in.Olang)
	setStr(q, "content_limit", in.ContentLimit)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "include", in.Include)
	return t.run(ctx, req, "catalog_calendar_pending", "/v1/catalog/calendar/pending", q)
}

const descCatalogCalendarTBA = "Release calendar bucket for works ANNOUNCED BUT UNDATED (no release date at all), " +
	"global rather than per-period, id ascending. Same gates as catalog_calendar."

type catalogCalendarTBAInput struct {
	Olang        string `json:"olang,omitempty" jsonschema:"Comma-separated original languages or all. Default = the ja + zh* family."`
	ContentLimit string `json:"content_limit,omitempty" jsonschema:"Comma-separated CLOSED vocabulary sfw,nsfw (editorial display axis). Unknown token = 400."`
	Cursor       string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit        int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw         bool   `json:"nsfw,omitempty" jsonschema:"true = include r18 works (default false = hidden)."`
	Include      string `json:"include,omitempty" jsonschema:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs."`
}

func (t *tools) catalogCalendarTBA(ctx context.Context, req *mcp.CallToolRequest, in catalogCalendarTBAInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "olang", in.Olang)
	setStr(q, "content_limit", in.ContentLimit)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "include", in.Include)
	return t.run(ctx, req, "catalog_calendar_tba", "/v1/catalog/calendar/tba", q)
}

const descCatalogLabelsList = "Browse the label (brand / doujin circle) vocabulary itself, id ascending and keyset " +
	"paginated, optionally filtered by kind; each row carries an nsfw-aware work_count. Use this to DISCOVER label " +
	"ids; to fetch one label with its works use catalog_label_get, and to find a label by name use catalog_search " +
	"type=labels."

type catalogLabelsListInput struct {
	Kind   string `json:"kind,omitempty" jsonschema:"Filter by kind: game_brand, bunko, publisher, anime_studio, doujin_circle, or group. A token outside this closed set is a 400."`
	Cursor string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw   bool   `json:"nsfw,omitempty" jsonschema:"true = count r18 works in work_count (default false = excluded)."`
}

func (t *tools) catalogLabelsList(ctx context.Context, req *mcp.CallToolRequest, in catalogLabelsListInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "kind", in.Kind)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_labels_list", "/v1/catalog/labels", q)
}

const descCatalogTagsList = "Browse the canonical tag vocabulary itself, id ascending and keyset paginated, " +
	"optionally filtered by tier / kind; each row carries an nsfw-aware work_count. Use this to DISCOVER tag ids " +
	"to feed catalog_works_list tag_id= or catalog_works_search; to fetch one tag with its works use catalog_tag_get. " +
	"Pass ids= to BATCH-HYDRATE tag ids you already hold — e.g. naming the bare tag_id rows of a " +
	"catalog_works_search facet block in one call rather than one catalog_tag_get per row."

type catalogTagsListInput struct {
	Tier   string `json:"tier,omitempty" jsonschema:"Filter by display tier: core, longtail, or hidden. A token outside this closed set is a 400."`
	Kind   string `json:"kind,omitempty" jsonschema:"Filter by kind: content or meta. A token outside this closed set is a 400."`
	IDs    string `json:"ids,omitempty" jsonschema:"Comma-separated tag ids (max 100) — batch-hydrate known ids in one call."`
	Cursor string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw   bool   `json:"nsfw,omitempty" jsonschema:"true = count r18 works in work_count (default false = excluded)."`
}

func (t *tools) catalogTagsList(ctx context.Context, req *mcp.CallToolRequest, in catalogTagsListInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "tier", in.Tier)
	setStr(q, "kind", in.Kind)
	setStr(q, "ids", in.IDs)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_tags_list", "/v1/catalog/tags", q)
}

const descCatalogEnginesList = "Browse the engine vocabulary itself (the visual-novel engines works are built with), " +
	"id ascending and keyset paginated; each row carries an nsfw-aware work_count. Use this to DISCOVER engine ids " +
	"to feed catalog_works_search engine_id=."

type catalogEnginesListInput struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw   bool   `json:"nsfw,omitempty" jsonschema:"true = count r18 works in work_count (default false = excluded)."`
}

func (t *tools) catalogEnginesList(ctx context.Context, req *mcp.CallToolRequest, in catalogEnginesListInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_engines_list", "/v1/catalog/engines", q)
}

const descCatalogEngineGet = "Fetch one engine by its numeric id: name, an nsfw-aware work_count and the exact " +
	"cross-source refs. Find an id with catalog_engines_list."

type catalogEngineGetInput struct {
	ID   int  `json:"id" jsonschema:"The catalog engine id (required)."`
	Nsfw bool `json:"nsfw,omitempty" jsonschema:"true = count r18 works in work_count (default false = excluded)."`
}

func (t *tools) catalogEngineGet(ctx context.Context, req *mcp.CallToolRequest, in catalogEngineGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_engine_get", pathID("/v1/catalog/engines", in.ID), q)
}

const descCatalogSeriesList = "Browse the series vocabulary itself (a series groups a work's sequels / fandiscs / " +
	"remakes under one banner), id ascending and keyset paginated; each row carries an nsfw-aware work_count and the " +
	"source that filed it. Series are NOT reachable from catalog_search — this lane is the only way to discover a " +
	"series id; to fetch one with its member works use catalog_series_get."

type catalogSeriesListInput struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit  int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20)."`
	Nsfw   bool   `json:"nsfw,omitempty" jsonschema:"true = count r18 works in work_count (default false = excluded)."`
	Source string `json:"source,omitempty" jsonschema:"Comma-separated filter on the same key each row prints in source: curated (hand-filed), derived (built by the automatic series lane), dlsite (filed by that importer). An OPEN vocabulary — an unrecognized token yields an empty page rather than an error."`
}

func (t *tools) catalogSeriesList(ctx context.Context, req *mcp.CallToolRequest, in catalogSeriesListInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "source", in.Source)
	return t.run(ctx, req, "catalog_series_list", "/v1/catalog/series", q)
}

const descCatalogSeriesGet = "Fetch one series by its numeric id: identity, the source anchor and intros, plus its " +
	"member works IN READING ORDER when include_works is set — which is what makes this the tool to answer \"what " +
	"order do I play this in\". Find an id with catalog_series_list."

type catalogSeriesGetInput struct {
	ID           int  `json:"id" jsonschema:"The catalog series id (required)."`
	IncludeWorks bool `json:"include_works,omitempty" jsonschema:"true = attach the series' member works in reading order (default false = identity only)."`
	Nsfw         bool `json:"nsfw,omitempty" jsonschema:"true = keep r18 works among the members (default false = hidden)."`
	Limit        int  `json:"limit,omitempty" jsonschema:"Member works per page 1-50 (default 50)."`
	Offset       int  `json:"offset,omitempty" jsonschema:"Member works to skip, for paging past the first page."`
}

func (t *tools) catalogSeriesGet(ctx context.Context, req *mcp.CallToolRequest, in catalogSeriesGetInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	if in.IncludeWorks {
		setStr(q, "include", "works")
	}
	setBool(q, "nsfw", in.Nsfw)
	setInt(q, "limit", in.Limit)
	setInt(q, "offset", in.Offset)
	return t.run(ctx, req, "catalog_series_get", pathID("/v1/catalog/series", in.ID), q)
}

const descCatalogStats = "Catalogue-wide totals: LIVE work counts per medium plus the identity-family totals " +
	"(names / characters / labels and friends). Takes no arguments. Use this to answer \"how big is the catalogue\" " +
	"or to sanity-check coverage before claiming something is absent — it is a summary, never a way to find a " +
	"specific entity."

type catalogStatsInput struct{}

func (t *tools) catalogStats(ctx context.Context, req *mcp.CallToolRequest, _ catalogStatsInput) (*mcp.CallToolResult, any, error) {
	return t.run(ctx, req, "catalog_stats", "/v1/catalog/stats", newQuery())
}

const descCatalogLabelRelationGraph = "Fetch the whole corporate family around one brand/label in a single call: parents, " +
	"subsidiaries, imprints, spin-offs and succession, as nodes[] + edges[]. Use this — not repeated catalog_label_get " +
	"calls — whenever the question is structural (\"which brands belong to X\", \"who owns Y\", \"is A related to B\"): " +
	"catalog_label_get.relations[] is only ONE HOP from the label you asked about, so reconstructing a family from it " +
	"means guessing which neighbour to walk next. The walk is bounded server-side at depth 4 and 60 nodes, breadth-first " +
	"from the seed, so a truncated answer keeps the neighbourhood NEAREST the label you asked for; there is no paging. " +
	"nodes[0] is always the seed. A label with no relations is a one-node, zero-edge graph — that is the answer \"it " +
	"stands alone\", not an error. EDGE READING: {from, to, relation} means \"`to` is the `relation` of `from`\"."

type catalogLabelRelationGraphInput struct {
	ID   int  `json:"id" jsonschema:"The catalog label id to grow the graph from (required)."`
	Nsfw bool `json:"nsfw,omitempty" jsonschema:"true = count r18 works in every node's work_count (default false = excluded, matching an sfw catalog_label_get)."`
}

func (t *tools) catalogLabelRelationGraph(ctx context.Context, req *mcp.CallToolRequest, in catalogLabelRelationGraphInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, "catalog_label_relation_graph",
		pathID("/v1/catalog/labels", in.ID)+"/relation-graph", q)
}

const descCatalogReleases = "Query individual RELEASES — the calendar one grain down, and the tool for any question that " +
	"names a platform, a language, an edition or a date RANGE. catalog_calendar places a work in the month of its " +
	"EARLIEST release and shows it once, so a Switch port, a Steam re-issue or a Chinese localisation of an older title " +
	"is invisible there by construction; here every release row is its own dated item. is_first tells the two apart: " +
	"true = the work genuinely coming out, false = a later edition. Filter by date_from/date_to, platform, lang " +
	"(the RELEASE's language) vs olang (the parent WORK's original language), kind and official. Note two defaults that " +
	"silently narrow the feed: kind defaults to default,digital,physical (demos and translation patches are EXCLUDED — " +
	"pass kind=default,digital,physical,trial,patch for everything, or kind=patch to watch localisations land), and olang " +
	"defaults to the ja + zh* family (pass olang=all to switch it off). Only releases dated to at least the MONTH appear; " +
	"year-only and undated ones live in catalog_calendar_pending and catalog_calendar_tba."

type catalogReleasesInput struct {
	Sort         string `json:"sort,omitempty" jsonschema:"date_desc (default, newest first) or date_asc. A cursor minted in one direction is refused in the other."`
	DateFrom     string `json:"date_from,omitempty" jsonschema:"YYYY-MM-DD inclusive lower bound on the release's own date. A month-precision release sits at its month's START, so date_from=2024-06-01 excludes a 2024-06 release while 2024-05-31 includes it. Malformed = 400."`
	DateTo       string `json:"date_to,omitempty" jsonschema:"YYYY-MM-DD inclusive upper bound, same precision rule as date_from. Malformed = 400."`
	Olang        string `json:"olang,omitempty" jsonschema:"Original language of the PARENT WORK: comma-separated BCP-47 (ja, zh-Hans, en, …) or all. Default = the ja + zh* family. Open vocabulary — an unknown value yields an empty feed, never a 400."`
	Lang         string `json:"lang,omitempty" jsonschema:"The RELEASE's own language: comma-separated BCP-47, or all. Default = no gate. Store-SKU lanes record no language of their own and match their work's original language. Open vocabulary."`
	Kind         string `json:"kind,omitempty" jsonschema:"Comma-separated CLOSED vocabulary: default,digital,physical,trial,patch. DEFAULT (absent) = default,digital,physical only — demos and patches are excluded. Unknown token = 400."`
	Official     string `json:"official,omitempty" jsonschema:"true drops explicitly-unofficial rows (fan translations, unofficial editions); false selects exactly those. Absent = no gate; a row without the flag counts as official."`
	Platform     string `json:"platform,omitempty" jsonschema:"vndb platform code (win/and/ios/…) matched against the release's PRIMARY platform. Open vocabulary — an unknown code is an empty feed."`
	ContentLimit string `json:"content_limit,omitempty" jsonschema:"Comma-separated CLOSED vocabulary sfw,nsfw — the editorial DISPLAY axis on the parent work (not the age rating). Unknown token = 400."`
	Cursor       string `json:"cursor,omitempty" jsonschema:"Opaque keyset cursor from a prior next_cursor; omit for the first page."`
	Limit        int    `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 20); above 100 is clamped."`
	Nsfw         bool   `json:"nsfw,omitempty" jsonschema:"true = include releases of r18 works (default false = hidden)."`
	Include      string `json:"include,omitempty" jsonschema:"Comma-separated rich-brief blocks applied to each item's attached WORK: names,intros,labels,ratings,covers,refs. The release's own refs[] are always present."`
}

func (t *tools) catalogReleases(ctx context.Context, req *mcp.CallToolRequest, in catalogReleasesInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setStr(q, "sort", in.Sort)
	setStr(q, "date_from", in.DateFrom)
	setStr(q, "date_to", in.DateTo)
	setStr(q, "olang", in.Olang)
	setStr(q, "lang", in.Lang)
	setStr(q, "kind", in.Kind)
	setStr(q, "official", in.Official)
	setStr(q, "platform", in.Platform)
	setStr(q, "content_limit", in.ContentLimit)
	setStr(q, "cursor", in.Cursor)
	setInt(q, "limit", in.Limit)
	setBool(q, "nsfw", in.Nsfw)
	setStr(q, "include", in.Include)
	return t.run(ctx, req, "catalog_releases", "/v1/catalog/releases", q)
}
