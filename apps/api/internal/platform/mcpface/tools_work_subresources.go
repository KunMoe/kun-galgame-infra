package mcpface

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const workBlockNote = " Returns ONE PAGED block of a work, so prefer it over catalog_work_get whenever this block is all " +
	"you need. Items are the same objects the parent block carries — same schema, same order, same election. " +
	"limit/offset page it (1-100, default 100); next_offset is present only while rows remain, and absent means the " +
	"block is exhausted. Visibility is catalog_work_get's verbatim: a work that 404s there 404s here too."

func registerWorkSubresourceTools(s *mcp.Server, t *tools) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_covers",
		Title:       "Cover images of one work",
		Description: descCatalogWorkCovers,
		Annotations: readOnly,
	}, t.catalogWorkCovers)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_screenshots",
		Title:       "Screenshots of one work",
		Description: descCatalogWorkScreenshots,
		Annotations: readOnly,
	}, t.catalogWorkScreenshots)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_tags",
		Title:       "Source tags of one work",
		Description: descCatalogWorkTags,
		Annotations: readOnly,
	}, t.catalogWorkTags)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_characters",
		Title:       "Character roster of one work",
		Description: descCatalogWorkCharacters,
		Annotations: readOnly,
	}, t.catalogWorkCharacters)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_credits",
		Title:       "Staff credits of one work",
		Description: descCatalogWorkCredits,
		Annotations: readOnly,
	}, t.catalogWorkCredits)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_releases",
		Title:       "Release rows of one work",
		Description: descCatalogWorkReleases,
		Annotations: readOnly,
	}, t.catalogWorkReleases)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_intros",
		Title:       "Synopses of one work",
		Description: descCatalogWorkIntros,
		Annotations: readOnly,
	}, t.catalogWorkIntros)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_ratings",
		Title:       "Per-source ratings of one work",
		Description: descCatalogWorkRatings,
		Annotations: readOnly,
	}, t.catalogWorkRatings)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_relations",
		Title:       "Related works of one work",
		Description: descCatalogWorkRelations,
		Annotations: readOnly,
	}, t.catalogWorkRelations)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_series",
		Title:       "The series one work belongs to",
		Description: descCatalogWorkSeries,
		Annotations: readOnly,
	}, t.catalogWorkSeries)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_links",
		Title:       "Outbound links of one work",
		Description: descCatalogWorkLinks,
		Annotations: readOnly,
	}, t.catalogWorkLinks)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "catalog_work_engines",
		Title:       "Engines one work is built with",
		Description: descCatalogWorkEngines,
		Annotations: readOnly,
	}, t.catalogWorkEngines)
}

type workSubresourceInput struct {
	ID     int  `json:"id" jsonschema:"The catalog work id (required) — the very id catalog_work_get takes."`
	Limit  int  `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 100); above 100 is clamped, a non-positive value is a 400."`
	Offset int  `json:"offset,omitempty" jsonschema:"Rows to skip."`
	Nsfw   bool `json:"nsfw,omitempty" jsonschema:"true = include r18 (default false = hidden). It gates the WORK, not the rows: without it an r18 work 404s on every block, and a row's own sexual/violence level is reported rather than filtered on."`
}

type workTagsInput struct {
	ID       int  `json:"id" jsonschema:"The catalog work id (required) — the very id catalog_work_get takes."`
	Limit    int  `json:"limit,omitempty" jsonschema:"Items per page 1-100 (default 100); above 100 is clamped, a non-positive value is a 400."`
	Offset   int  `json:"offset,omitempty" jsonschema:"Rows to skip."`
	Nsfw     bool `json:"nsfw,omitempty" jsonschema:"true = include r18 (default false = hidden). It gates the WORK and also decides work_count: an sfw caller's count excludes the r18 works it can never fetch."`
	Spoilers int  `json:"spoilers,omitempty" jsonschema:"Max tag spoiler level 0-2 (default 0 = safe). Rows above the ceiling are omitted entirely and consume no page slot, so raising it can change what a given offset points at."`
}

func workBlockPath(id int, block string) string {
	return pathID("/v1/catalog/works", id) + "/" + block
}

func (t *tools) workBlock(ctx context.Context, req *mcp.CallToolRequest, tool, block string, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setInt(q, "limit", in.Limit)
	setInt(q, "offset", in.Offset)
	setBool(q, "nsfw", in.Nsfw)
	return t.run(ctx, req, tool, workBlockPath(in.ID, block), q)
}

const descCatalogWorkCovers = "Cover images of one work — the block a store card or a shelf needs on its own. Rows the " +
	"CDN cannot render are dropped before paging rather than after, so a short page means the block is exhausted, not " +
	"that rows were filtered out. Want only the two display slots instead of every stored image? catalog_work_get's " +
	"cover_slots picks them for you." + workBlockNote

func (t *tools) catalogWorkCovers(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_covers", "covers", in)
}

const descCatalogWorkScreenshots = "Screenshots of one work, each with its dimensions and a thumbhash placeholder. " +
	"Every row publishes its own sexual and violence level: this face REPORTS them, it never filters on them, so gate " +
	"rendering yourself." + workBlockNote

func (t *tools) catalogWorkScreenshots(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_screenshots", "screenshots", in)
}

const descCatalogWorkTags = "Source tags of one work with their mapping into the canonical vocabulary and the safety " +
	"axis, ordered count DESC then name then source. A row that maps carries canonical_id/tier/kind plus an nsfw-aware " +
	"work_count — feed that canonical_id to catalog_works_search tag_id=; unmapped rows omit those fields." + workBlockNote

func (t *tools) catalogWorkTags(ctx context.Context, req *mcp.CallToolRequest, in workTagsInput) (*mcp.CallToolResult, any, error) {
	q := newQuery()
	setInt(q, "limit", in.Limit)
	setInt(q, "offset", in.Offset)
	setBool(q, "nsfw", in.Nsfw)
	setInt(q, "spoilers", in.Spoilers)
	return t.run(ctx, req, "catalog_work_tags", workBlockPath(in.ID, "tags"), q)
}

const descCatalogWorkCharacters = "The character roster of one work with its voice credits, ordered main, secondary, " +
	"appears, then the credit-only entries. NO spoiler ceiling is applied here and there is deliberately no spoilers " +
	"parameter: every row publishes its own spoiler level for you to gate on." + workBlockNote

func (t *tools) catalogWorkCharacters(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_characters", "characters", in)
}

const descCatalogWorkCredits = "Staff credits of one work grouped by role. PAGING IS OVER CREDIT ROWS, NOT GROUPS: " +
	"limit and offset count individual credits, so a role whose credits straddle a page boundary appears in BOTH pages " +
	"as its own group carrying that page's slice — concatenating pages means merging the groups that share a role_key." + workBlockNote

func (t *tools) catalogWorkCredits(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_credits", "credits", in)
}

const descCatalogWorkReleases = "Release rows of one work, release id ascending, each with its exact source anchors and " +
	"its own labels[] — THIS version's companies, which is where the Switch port's publisher and the English edition's " +
	"publisher each live. For a cross-work release timeline use catalog_releases instead." + workBlockNote

func (t *tools) catalogWorkReleases(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_releases", "releases", in)
}

const descCatalogWorkIntros = "Synopses of one work, one row per language, elected the parent's way: a source-written " +
	"synopsis beats a machine-translated one, and machine rows are FLAGGED rather than hidden — check the flag before " +
	"quoting one as the publisher's own words." + workBlockNote

func (t *tools) catalogWorkIntros(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_intros", "intros", in)
}

const descCatalogWorkRatings = "Per-source ratings of one work with the full vote histogram and the spread — this is " +
	"the work-DETAIL projection, unlike the works-list ratings block which drops them. Scores stay on each source's " +
	"NATIVE scale and are never blended into one number, so compare within a source, not across." + workBlockNote

func (t *tools) catalogWorkRatings(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_ratings", "ratings", in)
}

const descCatalogWorkRelations = "Related works of one work — the block catalog_work_get serves only under " +
	"include=relations. Without nsfw an r18 relation end is dropped WHOLE rather than emptied, and next_offset counts " +
	"what survived that drop. Sequels and fandiscs also reach you through catalog_work_get's series_siblings, which is " +
	"the transitive series component rather than this one-hop relation set." + workBlockNote

func (t *tools) catalogWorkRelations(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_relations", "relations", in)
}

const descCatalogWorkSeries = "The series this work belongs to. member_count is the series' WHOLE membership, not this " +
	"page — follow it with catalog_works_list series_id= or catalog_series_get include_works, which is also the only " +
	"way to get the members in reading order." + workBlockNote

func (t *tools) catalogWorkSeries(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_series", "series", in)
}

const descCatalogWorkLinks = "Non-identity outbound links of one work: official site, Steam, X and friends. These are " +
	"ADDRESSES, not anchors — the identity anchors are catalog_work_get's refs[]. Sources whose storefront URL cannot " +
	"be rebuilt from the bare stored code (dlsite, dmm) are absent here by design and stay reachable through refs[]." + workBlockNote

func (t *tools) catalogWorkLinks(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_links", "links", in)
}

const descCatalogWorkEngines = "Engines one work is built with, each with an nsfw-aware work_count — the number of works " +
	"this caller would page through via catalog_works_search engine_id=, so it always matches the list it can actually " +
	"fetch." + workBlockNote

func (t *tools) catalogWorkEngines(ctx context.Context, req *mcp.CallToolRequest, in workSubresourceInput) (*mcp.CallToolResult, any, error) {
	return t.workBlock(ctx, req, "catalog_work_engines", "engines", in)
}
