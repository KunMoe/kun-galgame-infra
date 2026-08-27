package handler

import (
	"context"
	"net/http"

	"api/internal/platform/catalog/dto"

	"github.com/danielgtaylor/huma/v2"
)

type publicWorkSubresourceInput struct {
	ID     int64 `path:"id" doc:"Catalog work id — the very id works/{id} answers on"`
	Limit  int   `query:"limit" doc:"Items per page 1-100 (default 100); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	Offset int   `query:"offset" doc:"Rows to skip"`
	NSFW   bool  `query:"nsfw" doc:"true/1 = serve this sub-resource for an r18 work (default false = 404, exactly what works/{id} answers for the same work). It gates the WORK, not the rows: a cover's or screenshot's own sexual/violence level is reported, never filtered, and on the two blocks that carry a work_count that count is taken over the population this caller can actually fetch"`
}

type publicWorkTagsInput struct {
	ID       int64 `path:"id" doc:"Catalog work id — the very id works/{id} answers on"`
	Limit    int   `query:"limit" doc:"Items per page 1-100 (default 100); above 100 is clamped to 100, a non-positive or non-numeric value is a 400"`
	Offset   int   `query:"offset" doc:"Rows to skip"`
	NSFW     bool  `query:"nsfw" doc:"true/1 = serve this sub-resource for an r18 work (default false = 404). It also decides work_count: an sfw caller's count excludes the r18 works it can never fetch"`
	Spoilers int16 `query:"spoilers" doc:"Max tag spoiler level 0-2 (default 0 = safe) — the works/{id} parameter verbatim. Rows above the ceiling are omitted entirely and consume no page slot, so raising the ceiling can change what a given offset points at"`
}

type publicWorkCoversOutput struct {
	Body Envelope[dto.PublicWorkCoversData]
}

type publicWorkScreenshotsOutput struct {
	Body Envelope[dto.PublicWorkScreenshotsData]
}

type publicWorkTagsOutput struct {
	Body Envelope[dto.PublicWorkTagsData]
}

type publicWorkCharactersOutput struct {
	Body Envelope[dto.PublicWorkCharactersData]
}

type publicWorkCreditsOutput struct {
	Body Envelope[dto.PublicWorkCreditsData]
}

type publicWorkReleasesOutput struct {
	Body Envelope[dto.PublicWorkReleasesData]
}

type publicWorkIntrosOutput struct {
	Body Envelope[dto.PublicWorkIntrosData]
}

type publicWorkRatingsOutput struct {
	Body Envelope[dto.PublicWorkRatingsData]
}

type publicWorkRelationsOutput struct {
	Body Envelope[dto.PublicWorkRelationsData]
}

type publicWorkSeriesOutput struct {
	Body Envelope[dto.PublicWorkSeriesData]
}

type publicWorkLinksOutput struct {
	Body Envelope[dto.PublicWorkLinksData]
}

type publicWorkEnginesOutput struct {
	Body Envelope[dto.PublicWorkEnginesData]
}

const workSubresourceContract = "ONE block of works/{id}, addressable on its own so a consumer that wants it does not pay for the other thirty keys — " +
	"a data-rich work is 50 KB with include=relations,credits, of which the identity core is under a tenth. " +
	"Items are the SAME objects the parent block carries: same schema, same order, same election, same suppression rules. " +
	"There is deliberately no second \"detail\" shape for a sub-resource to drift into. " +
	"VISIBILITY IS THE PARENT'S, VERBATIM — LIVE galgame works only, and a work works/{id} 404s 404s on every sub-resource too. " +
	"PAGED with limit/offset (1-100, default 100); next_offset is present only while rows remain, and absent means the block is exhausted. " +
	"The array embedded in works/{id} stays UNCAPPED: capping a published field is not a backward-compatible change, so the two faces differ in their bounds and in nothing else."

func registerWorkSubresourceSpecs(api huma.API, tags []string) {
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkCoversPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/covers",
		Summary:     "Cover images of one work, paged — the block a store card or a shelf needs on its own",
		Description: workSubresourceContract + " Rows the CDN cannot render are dropped before paging rather than after, so a page is short only when the block is exhausted. Need just the two display slots instead of every stored image? works/{id}.cover_slots and works?include=covers pick them for you.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkCoversOutput, error) {
		return &publicWorkCoversOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkScreenshotsPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/screenshots",
		Summary:     "Screenshots of one work, paged, with dimensions and thumbhash",
		Description: workSubresourceContract + " Each row carries its own sexual and violence level: this face reports them, it does not filter on them.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkScreenshotsOutput, error) {
		return &publicWorkScreenshotsOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkTagsPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/tags",
		Summary:     "Source tags of one work, paged, with the canonical mapping and the safety axis",
		Description: workSubresourceContract + " Ordered count DESC, then name in BYTE order, then source — the parent's order exactly. Rows that map to the canonical vocabulary carry canonical_id/tier/kind plus an nsfw-aware work_count; unmapped rows omit them.",
		Tags:        tags,
	}, func(context.Context, *publicWorkTagsInput) (*publicWorkTagsOutput, error) {
		return &publicWorkTagsOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkCharactersPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/characters",
		Summary:     "The character roster of one work, paged, with voice credits",
		Description: workSubresourceContract + " Order is main, secondary, appears, then the credit-only entries. Like the parent block this face applies NO spoiler ceiling — every row publishes its own spoiler level for the caller to gate on, which is why there is no spoilers parameter here.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkCharactersOutput, error) {
		return &publicWorkCharactersOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkCreditsPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/credits",
		Summary:     "Staff credits of one work grouped by role, paged over credit rows",
		Description: workSubresourceContract + " PAGING IS OVER CREDIT ROWS, NOT GROUPS: limit and offset count individual credits, so a role whose credits straddle a page boundary appears in both pages as its own group carrying that page's slice. Concatenating pages means merging the groups that share a role_key. Counting groups instead would leave one page unbounded, which is the thing this face exists to stop.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkCreditsOutput, error) {
		return &publicWorkCreditsOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkReleasesPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/releases",
		Summary:     "Release rows of one work, paged, each with its exact anchors and its own companies",
		Description: workSubresourceContract + " Ordered by release id ASC. labels[] is THIS version's companies (who developed it, who published it) — the Switch port's publisher and the English edition's publisher are two releases' facts, and this is where each of them lives. For a cross-work release timeline use GET /v1/catalog/releases instead.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkReleasesOutput, error) {
		return &publicWorkReleasesOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkIntrosPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/intros",
		Summary:     "Synopses of one work, one per language, paged",
		Description: workSubresourceContract + " One row per language, elected the parent's way: a source-written synopsis beats a machine-translated one, and machine rows are flagged rather than hidden.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkIntrosOutput, error) {
		return &publicWorkIntrosOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkRatingsPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/ratings",
		Summary:     "Per-source ratings of one work, paged, with the vote histogram and the spread",
		Description: workSubresourceContract + " This is the work-DETAIL projection, so distribution and stats are carried in full — unlike the works-list ratings block, which drops them. Scores stay on each source's native scale and are never blended into one number.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkRatingsOutput, error) {
		return &publicWorkRatingsOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkRelationsPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/relations",
		Summary:     "Related works of one work, paged — the block works/{id} only serves under include=relations",
		Description: workSubresourceContract + " With nsfw absent or 0 an r18 relation end is dropped WHOLE, not emptied, and next_offset counts what survived that drop. Sequels and fandiscs also reach you through works/{id}.series_siblings, which is the transitive series component rather than the one-hop relation set.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkRelationsOutput, error) {
		return &publicWorkRelationsOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkSeriesPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/series",
		Summary:     "The series this work belongs to, paged",
		Description: workSubresourceContract + " member_count is the series' whole membership, not this page — follow it with works?series_id=<id> or series/{id}?include=works.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkSeriesOutput, error) {
		return &publicWorkSeriesOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkLinksPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/links",
		Summary:     "Non-identity outbound links of one work (official site, Steam, X, …), paged",
		Description: workSubresourceContract + " These are addresses, not anchors: the identity anchors are works/{id}.refs[]. Sources whose storefront URL cannot be reconstructed from the stored code (dlsite, dmm — the section is part of the address and the registry stores only the bare code) are absent here by design and stay reachable through refs[].",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkLinksOutput, error) {
		return &publicWorkLinksOutput{}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "getCatalogWorkEnginesPublic", Method: http.MethodGet, Path: "/v1/catalog/works/{id}/engines",
		Summary:     "Engines one work is built with, paged, each with an nsfw-aware work_count",
		Description: workSubresourceContract + " work_count is the number of works this caller would page through via works?engine_id=<id>, so it always matches the list it can actually fetch.",
		Tags:        tags,
	}, func(context.Context, *publicWorkSubresourceInput) (*publicWorkEnginesOutput, error) {
		return &publicWorkEnginesOutput{}, nil
	})
}
