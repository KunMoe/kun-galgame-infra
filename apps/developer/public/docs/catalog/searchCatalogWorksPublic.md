# Works product search: free text + the works-list filter set, page-paginated, with opt-in facets and five sort lanes · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/works/search

Works product search: free text + the works-list filter set, page-paginated, with opt-in facets and five sort lanes

Searches the LIVE galgame registry (claimed + bodyless) by any indexed title or alias and narrows it with the same filters GET /v1/catalog/works accepts. Items are works-list rows VERBATIM (PublicWorkListItem, include= and all), re-hydrated from the registry — the search documents never reach the wire. total, the facet distribution and items are three views of ONE filtered set: page through total and you collect exactly that many rows, and an sfw caller's total already excludes the r18 works it can never receive.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `q` | query | 否 | string | Free text over every indexed title / alias of a work (search hints included — findability only). A query that is EXACTLY a VNDB work id (v19658) short-circuits to that one work via its exact anchor instead of full-text, which would prefix-bleed (v1965 also matches v19650). Empty = a filter-only browse ordered by popularity |
| `content_rating` | query | 否 | string | Filter by rating (r18 additionally requires nsfw=1) 取值：all_ages \| sensitive \| r18 |
| `claimed` | query | 否 | string | true = claimed works only; false = bodyless only; absent = both 取值：true \| false |
| `claim_state` | query | 否 | string | Comma-separated CLOSED vocabulary: none,live,draft,pending,declined,hidden — the values claimed_by.state renders (none = unclaimed registry row). Matching works must be in ANY of the listed states. An unknown token is a 400. Absent = no gate (every state), which keeps pre-existing callers byte-identical. A product site rendering its own catalogue should pass claim_state=live: the claimed parameter alone cannot tell a LIVE claim from a DRAFT (unpublished) or WITHDRAWN one. Freshness follows the index: a claim-state change is reflected by the next reindex-catalog run (daily cron), like every other indexed facet |
| `content_limit` | query | 否 | string | Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis, i.e. the values claimed_by.content_limit renders. An unknown token is a 400. Absent = no gate (both values), which keeps pre-existing callers byte-identical. This is NOT content_rating: content_rating is the AGE axis (what the GAME is rated), this is whether the material you would RENDER (cover, screenshots, synopsis) is safe to publish. Most r18 games carry editorially sfw display material, so filtering by content_rating instead hides the majority of a healthy catalogue. Orthogonal to nsfw= and claim_state=: all of them AND together in one filter, so total, facets and items stay behind the same gate |
| `label_id` | query | 否 | integer (int64) | Only works attributed to this label |
| `tag_id` | query | 否 | string | Only works carrying a source tag mapped to this canonical tag; up to 10 comma-separated ids are ANDed (a work must carry all of them), more than 10 or a non-positive/non-numeric entry is a 400 |
| `series_id` | query | 否 | integer (int64) | Only member works of this series |
| `engine_id` | query | 否 | integer (int64) | Only works built with this engine |
| `released_after` | query | 否 | string | YYYY-MM-DD, inclusive, over the EARLIEST release date per work — the same anchor the works list filters and the calendar buckets on |
| `released_before` | query | 否 | string | YYYY-MM-DD, inclusive |
| `olang` | query | 否 | string | Original-language gate: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …), or 'all'. Default (absent) = NO gate, i.e. the whole population — search is the discovery surface, so a caller who did not name a language gets every language. This is deliberately NOT the calendar's default, which curates down to the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty result, never a 400 |
| `sort` | query | 否 | string | relevance (default; an empty q degenerates to popularity), released_desc/asc over the earliest release date (works with no dated release sort last in BOTH directions), updated = newest-updated first, popularity = the cross-source signal log1p(max(bangumi collect shelf, DLsite downloads)) 取值：relevance \| released_desc \| released_asc \| updated \| popularity |
| `facets` | query | 否 | string | Comma-separated CLOSED vocabulary: content_rating,olang,claimed,tag_id,label_id,engine_id,series_id,source. An unknown token is a 400. Each distribution is counted over the SAME filtered set as total and is keyed by the values you would pass back to that very filter (content_rating counts use the public strings, not enum ints). At most 100 values per facet |
| `page` | query | 否 | integer (int64) | 1-based page number (default 1); a non-positive or non-numeric value is a 400. A page past the end is an empty page |
| `limit` | query | 否 | integer (int64) | Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400 |
| `nsfw` | query | 否 | boolean | true/1 = include r18 works (default false = dropped from items, total AND facets alike). The parameter is caller-controlled but capability-gated: a key without the NSFW capability (nsfw_allowed, granted per key via the developer portal) is refused with 403 rather than degraded |
| `include` | query | 否 | string | Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim (unknown tokens ignored) |
| `fields` | query | 否 | string | Comma-separated TOP-LEVEL keys of each ITEM to keep (default absent = every key, byte-identical to the base contract). The envelope — total/page/limit/items/facets — is never affected. id is always returned whether or not you name it. Unknown tokens are silently ignored, never a 400 (§3.5 clause 2). Trim-only: a kept key's value is byte-identical to the unprojected response. Applied AFTER include=, so naming an include-gated key (intros, labels, ratings, covers, refs, latin, localized) does NOT expand it — you still need both. The server is order- and duplicate-insensitive; WRITE THE TOKENS ALPHABETICALLY anyway, because the CDN keys on the raw URL and two orderings of the same selection are two cache entries |
| `search_intro` | query | 否 | boolean | true/1 = also match q against the work SYNOPSIS, not just its titles and aliases (A2-1f). Default false = titles only, byte-identical to the pre-A2-1f result set. Indexed synopses are capped at 2000 characters per language, and a synopsis match can never outrank a title match (the title attributes are ranked first) |

```bash
curl "https://api.nextmoe.dev/v1/catalog/works/search" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/searchCatalogWorksPublic
