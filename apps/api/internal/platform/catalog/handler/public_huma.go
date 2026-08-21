package handler

import (
	"context"
	"net/http"

	"api/internal/platform/catalog/dto"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
)


type publicWorkInput struct {
	ID       int64  `path:"id" doc:"Catalog work id"`
	Include  string `query:"include" doc:"Comma-separated heavy blocks: relations,credits (default: none)"`
	NSFW     bool   `query:"nsfw" doc:"true/1 = serve r18 works and r18 relation ends (caller-controlled; default false = hidden)"`
	Spoilers int16  `query:"spoilers" doc:"Max tag spoiler level 0-2 (default 0 = safe): tags[] carries per-edge spoiler + per-tag sexual flags, and rows above this ceiling are omitted entirely. The axis is populated for the VNDB-derived vocabulary only — Bangumi/DLsite folksonomy publishes no spoiler or category concept, so those rows read 0/false"`
}
type publicWorkOutput struct {
	Body Envelope[dto.PublicCatalogWork]
}

type publicLookupInput struct {
	Source     string `query:"source" doc:"Upstream source key: vndb | bangumi | dlsite | erogamescape (the legacy misspelling erogamespace is still accepted)"`
	ExternalID string `query:"external_id" doc:"The id within that source. type=work accepts vndb v19658 or 19658 (and dlsite RJ/VJ numbers); the non-work types match VERBATIM as the registry stores them — vndb character c1234, label p129, staff a bare number"`
	Type       string `query:"type" enum:"work,name,character,label" default:"work" doc:"Which entity family the external id is resolved against (default work); an unknown token is a 400, whereas an unknown SOURCE is a miss"`
	NSFW       bool   `query:"nsfw" doc:"true/1 = resolve r18 works too (default false = 404 on an r18 hit); on type=character it also keeps sexual traits"`
}
type publicLookupOutput struct {
	Body Envelope[dto.PublicLookupData]
}

type publicLookupBatchInput struct {
	Body dto.PublicLookupBatchRequest
}
type publicLookupBatchOutput struct {
	Body Envelope[dto.PublicLookupBatchData]
}

type publicResolveInput struct {
	Body dto.PublicResolveRequest
}
type publicResolveOutput struct {
	Body Envelope[dto.PublicResolveData]
}

type publicRedirectsInput struct {
	EntityType string `query:"entity_type" doc:"Filter to one entity type: person|name|label|character|work|release"`
	Cursor     string `query:"cursor" doc:"Opaque keyset cursor from a prior response's next_cursor; omit for the first page"`
	Limit      int    `query:"limit" doc:"Items per page 1-500 (default 100)"`
}
type publicRedirectsOutput struct {
	Body Envelope[dto.PublicRedirectsData]
}

type publicPersonInput struct {
	ID      int64  `path:"id" doc:"Credit-name id (the addressable credited identity)"`
	Include string `query:"include" doc:"credits = attach the works this name is credited on"`
	NSFW    bool   `query:"nsfw" doc:"true/1 = include r18 works among the credits (default false = dropped)"`
	Limit   int    `query:"limit" doc:"Credits per page 1-50 (default 50); above 50 is clamped to 50, a non-positive or non-numeric value is a 400"`
	Offset  int    `query:"offset" doc:"Rows to skip"`
}
type publicPersonOutput struct {
	Body Envelope[dto.PublicName]
}

type publicCharacterInput struct {
	ID       int64  `path:"id" doc:"Catalog character id"`
	Include  string `query:"include" doc:"works = attach the works this character appears in"`
	NSFW     bool   `query:"nsfw" doc:"true/1 = include r18 works and sexual-family traits (default false = both dropped)"`
	Spoilers int16  `query:"spoilers" doc:"max trait spoiler level 0-2 (default 0 = safe)"`
	Limit    int    `query:"limit" doc:"Works per page 1-50 (default 50); above 50 is clamped to 50, a non-positive or non-numeric value is a 400"`
	Offset   int    `query:"offset" doc:"Rows to skip"`
}
type publicCharacterOutput struct {
	Body Envelope[dto.PublicCharacter]
}

type publicLabelInput struct {
	ID      int64  `path:"id" doc:"Catalog label id"`
	Include string `query:"include" doc:"works = attach the works attributed to this label"`
	NSFW    bool   `query:"nsfw" doc:"true/1 = include r18 works among the attributions (default false = dropped)"`
	Limit   int    `query:"limit" doc:"Works per page 1-50 (default 50); above 50 is clamped to 50, a non-positive or non-numeric value is a 400"`
	Offset  int    `query:"offset" doc:"Rows to skip"`
}
type publicLabelOutput struct {
	Body Envelope[dto.PublicLabel]
}

type publicLabelGraphInput struct {
	ID   int64 `path:"id" doc:"Catalog label id — the seed the graph is grown from"`
	NSFW bool  `query:"nsfw" doc:"true/1 = count r18 works in every node's work_count (default false = excluded, matching an sfw labels/{id} call)"`
}
type publicLabelGraphOutput struct {
	Body Envelope[dto.PublicLabelGraph]
}

type publicEntitySearchInput struct {
	Type   string `query:"type" enum:"names,characters,labels,works,tags" doc:"Which index to search; works (wave 105) searches every LIVE galgame registry work by any title, tags (A2-1d) the canonical cross-source tag vocabulary"`
	Q      string `query:"q" doc:"Search text; empty returns the most-credited entities"`
	Locale string `query:"locale" enum:"zh,ja,en" doc:"UI locale; the server pins the query language"`
	Limit  int    `query:"limit" doc:"Max hits (capped at 20)"`
	NSFW   bool   `query:"nsfw" doc:"works only: true/1 = include r18 hits (default false = excluded server-side)"`
}
type publicEntitySearchOutput struct {
	Body Envelope[dto.PublicEntitySearchData]
}


type publicWorksSearchInput struct {
	Q              string `query:"q" doc:"Free text over every indexed title / alias of a work (search hints included — findability only). A query that is EXACTLY a VNDB work id (v19658) short-circuits to that one work via its exact anchor instead of full-text, which would prefix-bleed (v1965 also matches v19650). Empty = a filter-only browse ordered by popularity"`
	ContentRating  string `query:"content_rating" enum:"all_ages,sensitive,r18" doc:"Filter by rating (r18 additionally requires nsfw=1)"`
	Claimed        string `query:"claimed" enum:"true,false" doc:"true = claimed works only; false = bodyless only; absent = both"`
	ClaimState     string `query:"claim_state" doc:"Comma-separated CLOSED vocabulary: none,live,draft,pending,declined,hidden — the values claimed_by.state renders (none = unclaimed registry row). Matching works must be in ANY of the listed states. An unknown token is a 400. Absent = no gate (every state), which keeps pre-existing callers byte-identical. A product site rendering its own catalogue should pass claim_state=live: the claimed parameter alone cannot tell a LIVE claim from a DRAFT (unpublished) or WITHDRAWN one. Freshness follows the index: a claim-state change is reflected by the next reindex-catalog run (daily cron), like every other indexed facet"`
	ContentLimit   string `query:"content_limit" doc:"Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis, i.e. the values claimed_by.content_limit renders. An unknown token is a 400. Absent = no gate (both values), which keeps pre-existing callers byte-identical. This is NOT content_rating: content_rating is the AGE axis (what the GAME is rated), this is whether the material you would RENDER (cover, screenshots, synopsis) is safe to publish. Most r18 games carry editorially sfw display material, so filtering by content_rating instead hides the majority of a healthy catalogue. Orthogonal to nsfw= and claim_state=: all of them AND together in one filter, so total, facets and items stay behind the same gate"`
	LabelID        int64  `query:"label_id" doc:"Only works attributed to this label"`
	TagID          string `query:"tag_id" doc:"Only works carrying a source tag mapped to this canonical tag; up to 10 comma-separated ids are ANDed (a work must carry all of them), more than 10 or a non-positive/non-numeric entry is a 400"`
	SeriesID       int64  `query:"series_id" doc:"Only member works of this series"`
	EngineID       int64  `query:"engine_id" doc:"Only works built with this engine"`
	ReleasedAfter  string `query:"released_after" doc:"YYYY-MM-DD, inclusive, over the EARLIEST release date per work — the same anchor the works list filters and the calendar buckets on"`
	ReleasedBefore string `query:"released_before" doc:"YYYY-MM-DD, inclusive"`
	OLang          string `query:"olang" doc:"Original-language gate: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …), or 'all'. Default (absent) = NO gate, i.e. the whole population — search is the discovery surface, so a caller who did not name a language gets every language. This is deliberately NOT the calendar's default, which curates down to the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty result, never a 400"`
	Sort           string `query:"sort" enum:"relevance,released_desc,released_asc,updated,popularity" doc:"relevance (default; an empty q degenerates to popularity), released_desc/asc over the earliest release date (works with no dated release sort last in BOTH directions), updated = newest-updated first, popularity = the cross-source signal log1p(max(bangumi collect shelf, DLsite downloads))"`
	Facets         string `query:"facets" doc:"Comma-separated CLOSED vocabulary: content_rating,olang,claimed,tag_id,label_id,engine_id,series_id,source. An unknown token is a 400. Each distribution is counted over the SAME filtered set as total and is keyed by the values you would pass back to that very filter (content_rating counts use the public strings, not enum ints). At most 100 values per facet"`
	Page           int    `query:"page" doc:"1-based page number (default 1); a non-positive or non-numeric value is a 400. A page past the end is an empty page"`
	Limit          int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW           bool   `query:"nsfw" doc:"true/1 = include r18 works (default false = dropped from items, total AND facets alike)"`
	Include        string `query:"include" doc:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim (unknown tokens ignored)"`
	SearchIntro    bool   `query:"search_intro" doc:"true/1 = also match q against the work SYNOPSIS, not just its titles and aliases (A2-1f). Default false = titles only, byte-identical to the pre-A2-1f result set. Indexed synopses are capped at 2000 characters per language, and a synopsis match can never outrank a title match (the title attributes are ranked first)"`
}
type publicWorksSearchOutput struct {
	Body Envelope[dto.PublicWorksSearchData]
}

type publicWorksListInput struct {
	ContentRating  string `query:"content_rating" enum:"all_ages,sensitive,r18" doc:"Filter by rating (r18 additionally requires nsfw=1)"`
	Claimed        string `query:"claimed" enum:"true,false" doc:"true = claimed works only; false = bodyless only; absent = both"`
	ClaimState     string `query:"claim_state" doc:"Comma-separated CLOSED vocabulary: none,live,draft,pending,declined,hidden — the values claimed_by.state renders on these very items (none = unclaimed registry row). Matching works must be in ANY of the listed states. An unknown token is a 400. Absent = no gate (every state), which keeps pre-existing callers byte-identical. Word-for-word the works/search parameter of the same name; a product site listing an entity's member works should pass claim_state=live, since the claimed parameter alone cannot tell a LIVE claim from a DRAFT (unpublished) or WITHDRAWN one. Unlike the search face this is a live registry predicate, so a claim-state change takes effect immediately — there is no index to wait for"`
	ContentLimit   string `query:"content_limit" doc:"Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis, i.e. the values claimed_by.content_limit renders on these very items. An unknown token is a 400. Absent = no gate (both values), which keeps pre-existing callers byte-identical. Word-for-word the works/search parameter of the same name. This is NOT content_rating: content_rating is the AGE axis (what the GAME is rated), this is whether the material you would RENDER (cover, screenshots, synopsis) is safe to publish — a claimed work reads its wiki body's editorial flag, a bodyless one falls back to r18=nsfw. Unlike the search face this is a live registry predicate, so an editorial change takes effect immediately"`
	Status         string `query:"status" enum:"live,pending" doc:"Which population to browse. Absent or live = the public LIVE registry set, byte-identical to every pre-existing call. pending = the MODERATOR REVIEW QUEUE: the works whose claim is submitted and awaiting a curator's decision (claimed_by.state=pending), tenant-pinned. The queue view needs a SECOND credential — the moderator's own OAuth access token in Authorization: Bearer alongside the API key in X-API-Key (the dual-credential transport) — and refuses with 403 unless that token carries a user identity, was issued to a FIRST-PARTY site client bound to a catalog site (a third-party application is never a moderation surface), and holds the catalog.claim.review permission. The page is forcibly scoped to that client's own catalog site: site= may repeat it but naming any other site is a 403, never a silent re-point. Refusals are always explicit — this lane never degrades to the live set, because a moderator handed an empty page would read it as an empty queue. Platform-wide queues stay on the staff face. Passing claim_state= together with status=pending is a 400 (the parameter already IS that gate)"`
	Site           string `query:"site" doc:"Restrict to the works claimed by ONE site (the value claimed_by.site renders on these very items); absent = every tenant and every unclaimed work, which keeps pre-existing callers byte-identical. An unknown site matches nothing rather than erroring. Live registry predicate applied inside the page, so both the page and its next_cursor describe that one tenant — and a claim that moves tenant is reflected immediately, with no index to wait for. The works/search face deliberately does NOT take this parameter: its index carries no site facet, and a tenant queue must read the live registry anyway"`
	LabelID        int64  `query:"label_id" doc:"Only works attributed to this label (the catalog_work_label edge)"`
	LabelRollup    bool   `query:"label_rollup" doc:"true/1 = widen label_id one hop DOWN the corporate graph, to the label's imprints and subsidiaries (wave 199) — the page a holding company that publishes nothing under its own name needs. Every row that came in through a child carries via_label naming that child; the company's own works carry none. The population is exactly work_count + imprint_work_count of labels/{id}. Spin-offs (spawned) and successions (succeeded_by) are NOT followed: those are other companies' catalogues. Ignored without label_id"`
	TagID          string `query:"tag_id" doc:"Only works carrying a source tag mapped to this canonical tag; up to 10 comma-separated ids are ANDed (a work must carry all of them), more than 10 or a non-positive/non-numeric entry is a 400"`
	SeriesID       int64  `query:"series_id" doc:"Only member works of this series"`
	EngineID       int64  `query:"engine_id" doc:"Only works built with this engine (the catalog_work_engine edge); browse the ids via GET /v1/catalog/engines"`
	Platform       string `query:"platform" doc:"vndb platform code (win/and/ios/...) — release-level and work-level rows unioned"`
	ReleasedAfter  string `query:"released_after" doc:"YYYY-MM-DD, inclusive, over the EARLIEST release date per work"`
	ReleasedBefore string `query:"released_before" doc:"YYYY-MM-DD, inclusive"`
	IDs            string `query:"ids" doc:"Comma-separated work ids (max 100) — the batch-hydrate lane"`
	Sort           string `query:"sort" enum:"id,updated" doc:"id = ascending browse order (default); updated = newest-updated first"`
	Cursor         string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit          int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW           bool   `query:"nsfw" doc:"true/1 = include r18 works (default false = dropped)"`
	Include        string `query:"include" doc:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs (default: none — the response is then byte-identical to the base contract). Unknown tokens are ignored. names carries latin + localized{}; intros carries one intro per language, detail-face shape; covers carries the portrait + banner slots with width/height/thumbhash; refs carries the work exact identity anchors, detail-face shape"`
}
type publicWorksListOutput struct {
	Body Envelope[dto.PublicWorksListData]
}

type publicChangesInput struct {
	EntityType string `query:"entity_type" enum:"work" doc:"v1 feed scope: work (default)"`
	Cursor     string `query:"cursor" doc:"Opaque keyset cursor; omit to start from the beginning"`
	Limit      int    `query:"limit" doc:"Items per page 1-500 (default 100); above 500 is clamped to 500, a non-positive or non-numeric value is a 400"`
}
type publicChangesOutput struct {
	Body Envelope[dto.PublicChangesData]
}

type publicSeriesInput struct {
	ID      int64  `path:"id" doc:"Catalog series id"`
	Include string `query:"include" doc:"works = attach the series' member works"`
	NSFW    bool   `query:"nsfw" doc:"true/1 = include r18 works among the members (default false = dropped)"`
	Limit   int    `query:"limit" doc:"Works per page 1-50 (default 50); above 50 is clamped to 50, a non-positive or non-numeric value is a 400"`
	Offset  int    `query:"offset" doc:"Rows to skip"`
}
type publicSeriesOutput struct {
	Body Envelope[dto.PublicSeriesDetail]
}

type publicSeriesListInput struct {
	Cursor string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit  int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW   bool   `query:"nsfw" doc:"true/1 = count r18 works in work_count (default false = excluded, matching what an sfw works?series_id= call returns)"`
	Source string `query:"source" doc:"Comma-separated lane filter on the SAME key each row prints in source: curated (hand-filed), derived (built by the automatic series lane), dlsite (filed by that importer). An OPEN vocabulary, like works/search's olang — which sources file series is registry data, not a code-level enum, so an unrecognized token yields an empty page rather than a 400. Absent = no gate, byte-identical to a pre-A4 call. total counts the same filtered population as items"`
}
type publicSeriesListOutput struct {
	Body Envelope[dto.PublicSeriesListData]
}

type publicStatsOutput struct {
	Body Envelope[dto.PublicCatalogStats]
}

type publicTagInput struct {
	ID      int64  `path:"id" doc:"Canonical tag id (the cross-source tag vocabulary)"`
	Include string `query:"include" doc:"works = attach the works carrying any mapped source tag"`
	NSFW    bool   `query:"nsfw" doc:"true/1 = include r18 works (default false = dropped)"`
	Limit   int    `query:"limit" doc:"Works per page 1-50 (default 50); above 50 is clamped to 50, a non-positive or non-numeric value is a 400"`
	Offset  int    `query:"offset" doc:"Rows to skip"`
}
type publicTagOutput struct {
	Body Envelope[dto.PublicTagDetail]
}


type publicLabelsListInput struct {
	Kind     string `query:"kind" enum:"game_brand,bunko,publisher,anime_studio,doujin_circle,group" doc:"Filter by label kind; a token outside this closed set is a 400"`
	Cursor   string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit    int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW     bool   `query:"nsfw" doc:"true/1 = count r18 works in work_count (default false = excluded, matching what an sfw works?label_id= call returns)"`
	HasWorks bool   `query:"has_works" doc:"true/1 = only labels whose work_count is > 0 under the same nsfw setting (default false = every label); total converges with the filter"`
}
type publicLabelsListOutput struct {
	Body Envelope[dto.PublicLabelsListData]
}

type publicTagsListInput struct {
	Tier     string `query:"tier" enum:"core,longtail,hidden" doc:"Filter by display tier; a token outside this closed set is a 400"`
	Kind     string `query:"kind" enum:"content,meta" doc:"Filter by tag kind; a token outside this closed set is a 400"`
	Cursor   string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit    int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW     bool   `query:"nsfw" doc:"true/1 = count r18 works in work_count (default false = excluded, matching what an sfw works?tag_id= call returns)"`
	HasWorks bool   `query:"has_works" doc:"true/1 = only tags whose work_count is > 0 under the same nsfw setting (default false = every tag); total converges with the filter"`
}
type publicTagsListOutput struct {
	Body Envelope[dto.PublicTagsListData]
}

type publicEnginesListInput struct {
	Cursor string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit  int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW   bool   `query:"nsfw" doc:"true/1 = count r18 works in work_count (default false = excluded, matching what an sfw works?engine_id= call returns)"`
}
type publicEnginesListOutput struct {
	Body Envelope[dto.PublicEnginesListData]
}

type publicEngineInput struct {
	ID   int64 `path:"id" doc:"Catalog engine id"`
	NSFW bool  `query:"nsfw" doc:"true/1 = count r18 works in work_count (default false = excluded)"`
}
type publicEngineOutput struct {
	Body Envelope[dto.PublicEngine]
}


type publicCalendarInput struct {
	Month        string `query:"month" doc:"ISO month YYYY-MM; default = the CURRENT Asia/Tokyo month, echoed back in the response. A malformed value is a 400"`
	OLang        string `query:"olang" doc:"Original-language gate: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …) or 'all' to switch it off. Default = the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty bucket, never a 400"`
	ContentLimit string `query:"content_limit" doc:"Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis (the values claimed_by.content_limit renders), gating BUCKET MEMBERSHIP, the count and the meta frame alike. An unknown token is a 400 (a CLOSED vocabulary, unlike olang above). Absent = no gate (both values), byte-identical to the pre-A2-R5 bucket. NOT content_rating: that is the AGE axis (what the GAME is rated), this is whether the material you would RENDER is safe to publish. It rides in the ETag population key, so an sfw-gated and an ungated caller never share a validator"`
	Cursor       string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit        int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW         bool   `query:"nsfw" doc:"true/1 = include r18 works (default false = dropped)"`
	Include      string `query:"include" doc:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim (unknown tokens ignored)"`
}
type publicCalendarOutput struct {
	Body Envelope[dto.PublicCalendarData]
}

type publicCalendarPendingInput struct {
	Year         string `query:"year" doc:"YYYY; default = the CURRENT Asia/Tokyo year, echoed back in the response. A malformed value is a 400"`
	OLang        string `query:"olang" doc:"Original-language gate: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …) or 'all' to switch it off. Default = the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty bucket, never a 400"`
	ContentLimit string `query:"content_limit" doc:"Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis (the values claimed_by.content_limit renders), gating BUCKET MEMBERSHIP, the count and the meta frame alike. An unknown token is a 400 (a CLOSED vocabulary, unlike olang above). Absent = no gate (both values), byte-identical to the pre-A2-R5 bucket. NOT content_rating: that is the AGE axis (what the GAME is rated), this is whether the material you would RENDER is safe to publish. It rides in the ETag population key, so an sfw-gated and an ungated caller never share a validator"`
	Cursor       string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit        int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW         bool   `query:"nsfw" doc:"true/1 = include r18 works (default false = dropped)"`
	Include      string `query:"include" doc:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim (unknown tokens ignored)"`
}
type publicCalendarPendingOutput struct {
	Body Envelope[dto.PublicCalendarData]
}

type publicCalendarTBAInput struct {
	OLang        string `query:"olang" doc:"Original-language gate: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …) or 'all' to switch it off. Default = the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty bucket, never a 400"`
	ContentLimit string `query:"content_limit" doc:"Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis (the values claimed_by.content_limit renders), gating BUCKET MEMBERSHIP, the count and the meta frame alike. An unknown token is a 400 (a CLOSED vocabulary, unlike olang above). Absent = no gate (both values), byte-identical to the pre-A2-R5 bucket. NOT content_rating: that is the AGE axis (what the GAME is rated), this is whether the material you would RENDER is safe to publish. It rides in the ETag population key, so an sfw-gated and an ungated caller never share a validator"`
	Cursor       string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit        int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW         bool   `query:"nsfw" doc:"true/1 = include r18 works (default false = dropped)"`
	Include      string `query:"include" doc:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim (unknown tokens ignored)"`
}
type publicCalendarTBAOutput struct {
	Body Envelope[dto.PublicCalendarData]
}


type publicReleasesInput struct {
	Sort         string `query:"sort" enum:"date_desc,date_asc" doc:"date_desc (default) = newest first, the feed reading; date_asc walks forwards from the earliest release. The tiebreak inside one date is id ASC in BOTH directions. Keyset-paged over (date, id) — a cursor minted in one direction is refused in the other"`
	DateFrom     string `query:"date_from" doc:"YYYY-MM-DD, INCLUSIVE lower bound on the release's own date. Note the precision rule: a month-precision release is addressed at its month's START (2024-06 sits at 2024-06-00, i.e. before 2024-06-01), so date_from=2024-06-01 excludes it while date_from=2024-05-31 includes it. A malformed value is a 400"`
	DateTo       string `query:"date_to" doc:"YYYY-MM-DD, INCLUSIVE upper bound, same ordinal and the same precision rule as date_from. A malformed value is a 400"`
	OLang        string `query:"olang" doc:"Original-language gate on the PARENT WORK: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …) or 'all' to switch it off. Default = the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty feed, never a 400. This is the work's original language, NOT the language of the release — for that use lang= below"`
	Lang         string `query:"lang" doc:"RELEASE-language gate: comma-separated BCP-47 tags, or 'all'. Default (absent) = NO gate. Matched over COALESCE(release.lang, work.olang): the store-SKU lanes (dlsite / getchu) create one release per work and record no language of its own, and such a SKU is by construction in its work's original language — so lang=ja matches them, while the item still prints the raw (possibly empty) release lang. An OPEN vocabulary: an unknown tag is an empty feed, never a 400"`
	Kind         string `query:"kind" doc:"Comma-separated CLOSED vocabulary: default,digital,physical,trial,patch — the five strings each item's kind key prints. An unknown token is a 400. DEFAULT (absent) = default,digital,physical ONLY: this is a 发售动态 surface, so demos and translation patches are EXCLUDED unless you ask for them. Pass kind=default,digital,physical,trial,patch for the genuinely unfiltered feed, or kind=patch to watch localisations land"`
	Official     string `query:"official" enum:"true,false" doc:"Gate on the release's official flag; absent = no gate. The flag is written by the VNDB release lane only, where false means a fan translation or unofficial edition — a row WITHOUT the key counts as OFFICIAL, because every other lane materialises store SKUs (dlsite worknos, getchu product pages) which are official by construction. official=true therefore drops only the explicitly-unofficial rows; official=false selects exactly those"`
	Platform     string `query:"platform" doc:"vndb platform code (win/and/ios/…) matched against the release's PRIMARY platform — the release leg of the works-list filter of the same name, verbatim. A release's platforms[] display list is not filtered on. OPEN vocabulary: an unknown code is an empty feed, never a 400"`
	ContentLimit string `query:"content_limit" doc:"Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis on the parent work (the values claimed_by.content_limit renders), gating FEED MEMBERSHIP and the count alike. An unknown token is a 400 (a CLOSED vocabulary, unlike olang / lang / platform above). Absent = no gate (both values). NOT content_rating: that is the AGE axis (what the GAME is rated), this is whether the material you would RENDER is safe to publish. It rides in the ETag population key, so an sfw-gated and an ungated caller never share a validator"`
	Cursor       string `query:"cursor" doc:"Opaque keyset cursor from a prior next_cursor; omit for the first page"`
	Limit        int    `query:"limit" doc:"Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	NSFW         bool   `query:"nsfw" doc:"true/1 = include releases of r18 works (default false = dropped)"`
	Include      string `query:"include" doc:"Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim, applied to each item's attached work block (unknown tokens ignored). The RELEASE's own refs[] are always present and are not affected by this parameter"`
}
type publicReleasesOutput struct {
	Body Envelope[dto.PublicReleaseFeedData]
}

func SetupCatalogPublicSpec(app *fiber.App) huma.API {
	InstallErrorEnvelope()

	cfg := huma.DefaultConfig("NextMoe Open API — Catalog", "1.0.0")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	api := humafiber.New(app, cfg)

	tags := []string{"catalog-public"}
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}",
		Summary: "Frozen work record: identity + titles + exact cross-source refs + claim pointer; include=relations,credits", Tags: tags,
	}, func(context.Context, *publicWorkInput) (*publicWorkOutput, error) { return &publicWorkOutput{}, nil })
	huma.Register(api, huma.Operation{
		OperationID: "lookupCatalogPublic", Method: http.MethodGet, Path: "/v1/catalog/lookup",
		Summary: "Reverse-lookup an external id via an EXACT anchor (killer feature); type=work|name|character|label, 404 on miss/hidden", Tags: tags,
	}, func(context.Context, *publicLookupInput) (*publicLookupOutput, error) {
		return &publicLookupOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "lookupCatalogBatchPublic", Method: http.MethodPost, Path: "/v1/catalog/lookup/batch",
		Summary: "Batch external-id reverse-lookup (≤100 pairs, per-pair type); misses return null blocks in order", Tags: tags,
	}, func(context.Context, *publicLookupBatchInput) (*publicLookupBatchOutput, error) {
		return &publicLookupBatchOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "resolveCatalogPublic", Method: http.MethodPost, Path: "/v1/catalog/resolve",
		Summary: "Batch old id → canonical id (redirect flattening) for a given entity_type", Tags: tags,
	}, func(context.Context, *publicResolveInput) (*publicResolveOutput, error) {
		return &publicResolveOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogRedirectsPublic", Method: http.MethodGet, Path: "/v1/catalog/redirects",
		Summary: "Keyset feed of id-convergence (merge) events for stored-id cleanup; filter by entity_type", Tags: tags,
	}, func(context.Context, *publicRedirectsInput) (*publicRedirectsOutput, error) {
		return &publicRedirectsOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogNamePublic", Method: http.MethodGet, Path: "/v1/catalog/names/{id}",
		Summary: "Credited identity (same-person grouping via public links); include=credits attaches works + roles", Tags: tags,
	}, func(context.Context, *publicPersonInput) (*publicPersonOutput, error) {
		return &publicPersonOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogCharacterPublic", Method: http.MethodGet, Path: "/v1/catalog/characters/{id}",
		Summary: "Character identity; include=works attaches the works it appears in with voice names", Tags: tags,
	}, func(context.Context, *publicCharacterInput) (*publicCharacterOutput, error) {
		return &publicCharacterOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogLabelPublic", Method: http.MethodGet, Path: "/v1/catalog/labels/{id}",
		Summary: "Label (brand / circle / publisher …) identity; include=works attaches attributed works", Tags: tags,
	}, func(context.Context, *publicLabelInput) (*publicLabelOutput, error) { return &publicLabelOutput{}, nil })
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogLabelRelationGraphPublic", Method: http.MethodGet, Path: "/v1/catalog/labels/{id}/relation-graph",
		Summary: "Corporate-structure graph around a label: the connected family (parents, subsidiaries, imprints, spin-offs, succession) in one call",
		Description: "labels/{id}.relations[] is ONE HOP; this is the whole component, which is what a picture needs — standing on " +
			"a brand you can see its parent's other brands without walking the one-hop face once per neighbour. " +
			"A breadth-first walk from the seed over the label-relation graph, bounded at depth 4 and 60 nodes; the bound is " +
			"applied breadth-first, so a truncated answer keeps the neighbourhood NEAREST the seed. Soft-deleted (merged-away) " +
			"labels never appear. There is no pagination — a graph served in slices is not a graph. " +
			"nodes[0] is always the seed, and a label with no relations is a one-node, zero-edge graph, not a 404; an unknown id " +
			"is a 404 and a merged id is the same 301 labels/{id} serves. " +
			"EDGE SEMANTICS: {from, to, relation} reads \"`to` is the `relation` of `from`\" — the same reading " +
			"relations[].relation has, where `from` is the label being viewed. So {from: Key, to: VisualArt's, relation: parent} " +
			"means \"VisualArt's is the parent of Key\". " +
			"The underlying graph is stored MIRRORED, but each fact is emitted ONCE: only the canonical side of each inverse " +
			"pair (parent, imprint, spawned, succeeded_by) is rendered, and the four inverses (subsidiary, imprint_of, origin, " +
			"formerly) are implied by reading the edge backwards — for \"the subsidiaries of X\", take the edges whose `to` is X " +
			"and whose relation is parent. " +
			"Every node's work_count is the SAME nsfw-aware number labels/{id} and the labels browse lane report for this caller.",
		Tags: tags,
	}, func(context.Context, *publicLabelGraphInput) (*publicLabelGraphOutput, error) {
		return &publicLabelGraphOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "searchCatalogEntitiesPublic", Method: http.MethodGet, Path: "/v1/catalog/search",
		Summary: "Entity autocomplete over names / characters / labels / works / tags, projected to public briefs",
		Description: "The identity finder: up to 20 flat hits of ONE family, no filters and no pagination — what a picker or a " +
			"jump-to-entity box needs. For a works RESULTS PAGE (filters, facets, sort, paging, full works-list rows) use " +
			"GET /v1/catalog/works/search instead.",
		Tags: tags,
	}, func(context.Context, *publicEntitySearchInput) (*publicEntitySearchOutput, error) {
		return &publicEntitySearchOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "searchCatalogWorksPublic", Method: http.MethodGet, Path: "/v1/catalog/works/search",
		Summary: "Works product search: free text + the works-list filter set, page-paginated, with opt-in facets and five sort lanes",
		Description: "Searches the LIVE galgame registry (claimed + bodyless) by any indexed title or alias and narrows it with the " +
			"same filters GET /v1/catalog/works accepts. Items are works-list rows VERBATIM (PublicWorkListItem, include= and all), " +
			"re-hydrated from the registry — the search documents never reach the wire. " +
			"total, the facet distribution and items are three views of ONE filtered set: page through total and you collect exactly " +
			"that many rows, and an sfw caller's total already excludes the r18 works it can never receive.",
		Tags: tags,
	}, func(context.Context, *publicWorksSearchInput) (*publicWorksSearchOutput, error) {
		return &publicWorksSearchOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogWorksPublic", Method: http.MethodGet, Path: "/v1/catalog/works",
		Summary: "Keyset works browse lane: the LIVE galgame registry set (claimed + bodyless) with conjunctive filters; sort=id|updated", Tags: tags,
	}, func(context.Context, *publicWorksListInput) (*publicWorksListOutput, error) {
		return &publicWorksListOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogChangesPublic", Method: http.MethodGet, Path: "/v1/catalog/changes",
		Summary: "Incremental works changes feed ((updated,id) keyset; next_cursor always present — keep polling it for new rows)",
		Description: "Creations and updates of LIVE galgame works, ordered by (updated_at, id) ASC. " +
			"The feed deliberately trails real time by ~5 seconds: updated_at is statement time, not commit time, " +
			"so serving rows younger than that lag would let a slow transaction commit behind an already-advanced " +
			"consumer cursor and be skipped forever. " +
			"DELETIONS DO NOT FLOW THROUGH THIS FEED — a row that leaves the LIVE set simply stops appearing; " +
			"merge-style disappearances are covered by /v1/catalog/redirects, and mirror-style consumers should " +
			"periodically reconcile the full id set via works?sort=id.",
		Tags: tags,
	}, func(context.Context, *publicChangesInput) (*publicChangesOutput, error) {
		return &publicChangesOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogSeriesPublic", Method: http.MethodGet, Path: "/v1/catalog/series",
		Summary: "Keyset series browse lane (id ASC); each row carries an nsfw-aware work_count",
		Description: "The grouping entities works?series_id= filters on, id ascending. " +
			"work_count is the number of works THIS caller would page through via works?series_id=<id>. " +
			"Series are a curated, source-mirrored facet with no search index of their own, so this lane " +
			"is how a consumer discovers an id it does not already hold.",
		Tags: tags,
	}, func(context.Context, *publicSeriesListInput) (*publicSeriesListOutput, error) {
		return &publicSeriesListOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogSeriesPublic", Method: http.MethodGet, Path: "/v1/catalog/series/{id}",
		Summary: "Series record: identity + source anchor + intros; include=works attaches its member works in reading order",
		Description: "The address of the grouping entity works?series_id= filters on. Members are the LIVE galgame " +
			"fetchable set, paged by limit/offset exactly like labels/{id} and tags/{id}, ordered by release date " +
			"(members with no dated release last). members[] runs parallel to works[] — same page, same order — and " +
			"carries each membership's position (1-based; 0 = not ordered yet) and kind " +
			"(unknown/main/fandisc/side_story/collection). " +
			"Series carry no merge or soft-delete machinery, so an unknown id is a plain 404 — never a redirect.",
		Tags: tags,
	}, func(context.Context, *publicSeriesInput) (*publicSeriesOutput, error) { return &publicSeriesOutput{}, nil })
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogStatsPublic", Method: http.MethodGet, Path: "/v1/catalog/stats",
		Summary: "Slim catalogue counts: LIVE works per medium + the identity-family totals",
		Description: "The product-facing size of the registry, no parameters and one payload for every caller. " +
			"works counts LIVE rows only (stubs, merged-away rows and soft-deleted rows are not part of the " +
			"catalogue) and total is the sum of by_medium, so the two can never disagree. " +
			"R18 works ARE counted: these are aggregates with nothing renderable attached, and splitting them by " +
			"nsfw would publish exactly what the r18 gate exists to hide. " +
			"The INTERNAL dashboard (review queues, LLM verdicts, the anchor source × tier matrix, source " +
			"freshness, orphan and claim-state breakdowns) is curation telemetry and stays on the S2S face.",
		Tags: tags,
	}, func(context.Context, *struct{}) (*publicStatsOutput, error) { return &publicStatsOutput{}, nil })
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogTagPublic", Method: http.MethodGet, Path: "/v1/catalog/tags/{id}",
		Summary: "Canonical tag (cross-source vocabulary): name / tier / kind / intros; include=works attaches the tagged works", Tags: tags,
	}, func(context.Context, *publicTagInput) (*publicTagOutput, error) { return &publicTagOutput{}, nil })
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogLabelsPublic", Method: http.MethodGet, Path: "/v1/catalog/labels",
		Summary: "Keyset label browse lane (id ASC); filter by kind, each row carries an nsfw-aware work_count",
		Description: "Every label that has not been merged away, id ascending. " +
			"work_count is the number of works THIS caller would page through via works?label_id=<id> — " +
			"so an sfw caller's count excludes r18 works and always matches the list it can actually fetch.",
		Tags: tags,
	}, func(context.Context, *publicLabelsListInput) (*publicLabelsListOutput, error) {
		return &publicLabelsListOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogTagsPublic", Method: http.MethodGet, Path: "/v1/catalog/tags",
		Summary: "Keyset canonical-tag browse lane (id ASC); filter by tier / kind, each row carries an nsfw-aware work_count",
		Description: "The cross-source canonical tag vocabulary, id ascending. " +
			"work_count is the number of works THIS caller would page through via works?tag_id=<id> " +
			"(counted over the source-tag map, so a work carrying two mapped source tags counts once).",
		Tags: tags,
	}, func(context.Context, *publicTagsListInput) (*publicTagsListOutput, error) {
		return &publicTagsListOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogEnginesPublic", Method: http.MethodGet, Path: "/v1/catalog/engines",
		Summary: "Keyset engine browse lane (id ASC); each row carries an nsfw-aware work_count",
		Description: "The visual-novel / game engines works are built with, id ascending. " +
			"work_count is the number of works THIS caller would page through via works?engine_id=<id>.",
		Tags: tags,
	}, func(context.Context, *publicEnginesListInput) (*publicEnginesListOutput, error) {
		return &publicEnginesListOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogEnginePublic", Method: http.MethodGet, Path: "/v1/catalog/engines/{id}",
		Summary: "Engine record: name + nsfw-aware work_count + exact cross-source refs", Tags: tags,
	}, func(context.Context, *publicEngineInput) (*publicEngineOutput, error) {
		return &publicEngineOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogReleasesPublic", Method: http.MethodGet, Path: "/v1/catalog/releases",
		Summary: "Release-grain new-releases timeline: every dated release row, ports and re-editions included (date keyset)",
		Description: "The calendar one grain down. /v1/catalog/calendar places a WORK in the month of its EARLIEST release and shows " +
			"it once, so a Switch port, a Steam re-issue or a Chinese localisation of a title released years ago is invisible " +
			"there by construction. Here every release row is its own item, on its own date. " +
			"is_first tells the two apart: true = the work's earliest dated release (a title genuinely coming out), " +
			"false = a later edition of something already released — computed over the work's whole dated-release set, so it " +
			"does not change when you narrow the feed. Each item carries the parent work as a works-list row (include= and all). " +
			"Population: releases of LIVE galgame works whose date is known to at least the MONTH. Year-only and undated " +
			"releases are deliberately absent — they cannot be ordered against a real date, and they already have homes in " +
			"/v1/catalog/calendar/pending and /v1/catalog/calendar/tba. " +
			"Carries a feed-level ETag (count + newest created_at + highest release id over the whole filtered set): an " +
			"If-None-Match hit 304s before any page is loaded.",
		Tags: tags,
	}, func(context.Context, *publicReleasesInput) (*publicReleasesOutput, error) {
		return &publicReleasesOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogCalendarPublic", Method: http.MethodGet, Path: "/v1/catalog/calendar",
		Summary: "Release calendar, one ISO month (date ASC keyset); default = the current Asia/Tokyo month",
		Description: "Works whose EARLIEST year-carrying, non-deleted release falls inside the month — day precision and " +
			"month precision alike (a month-precision work sorts at the head of its month, it is never pinned to the 1st). " +
			"Same classification anchor as the works-list release_date, so a row's bucket and its printed date can never disagree. " +
			"Year-precision works live in /v1/catalog/calendar/pending, undated ones in /v1/catalog/calendar/tba, and a work with " +
			"no release row at all is in no bucket. Items are works-list rows verbatim, include= and all. " +
			"Carries a bucket-level ETag (count + newest updated_at over the whole filtered set): an If-None-Match hit 304s " +
			"before any page is loaded.",
		Tags: tags,
	}, func(context.Context, *publicCalendarInput) (*publicCalendarOutput, error) {
		return &publicCalendarOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogCalendarPendingPublic", Method: http.MethodGet, Path: "/v1/catalog/calendar/pending",
		Summary: "Release calendar, one year's month-still-unknown bucket (id ASC keyset); default = the current Asia/Tokyo year",
		Description: "Works whose earliest release is known only to the YEAR — they appear in no month view of that year, by design. " +
			"Same population, item shape, olang gate and ETag mechanics as the month bucket.",
		Tags: tags,
	}, func(context.Context, *publicCalendarPendingInput) (*publicCalendarPendingOutput, error) {
		return &publicCalendarPendingOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "listCatalogCalendarTBAPublic", Method: http.MethodGet, Path: "/v1/catalog/calendar/tba",
		Summary: "Release calendar, the global announced-but-undated bucket (id ASC keyset)",
		Description: "Works that HAVE release rows but none carrying a year. A work with no release row at all is 'unknown' " +
			"and deliberately enters no bucket — absence of a release is absence of an announcement, not a TBA date.",
		Tags: tags,
	}, func(context.Context, *publicCalendarTBAInput) (*publicCalendarTBAOutput, error) {
		return &publicCalendarTBAOutput{}, nil
	})

	registerWorkSubresourceSpecs(api, tags)

	RegisterPlaytimeOps(api, nil)
	return api
}
