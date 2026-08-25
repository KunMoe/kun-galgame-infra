# Keyset works browse lane: the LIVE galgame registry set (claimed + bodyless) with conjunctive filters; sort=id|updated · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/works

Keyset works browse lane: the LIVE galgame registry set (claimed + bodyless) with conjunctive filters; sort=id|updated

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nm_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `content_rating` | query | 否 | string | Filter by rating (r18 additionally requires nsfw=1) 取值：all_ages \| sensitive \| r18 |
| `claimed` | query | 否 | string | true = claimed works only; false = bodyless only; absent = both 取值：true \| false |
| `claim_state` | query | 否 | string | Comma-separated CLOSED vocabulary: none,live,draft,pending,declined,hidden — the values claimed_by.state renders on these very items (none = unclaimed registry row). Matching works must be in ANY of the listed states. An unknown token is a 400. Absent = no gate (every state), which keeps pre-existing callers byte-identical. Word-for-word the works/search parameter of the same name; a product site listing an entity's member works should pass claim_state=live, since the claimed parameter alone cannot tell a LIVE claim from a DRAFT (unpublished) or WITHDRAWN one. Unlike the search face this is a live registry predicate, so a claim-state change takes effect immediately — there is no index to wait for |
| `content_limit` | query | 否 | string | Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis, i.e. the values claimed_by.content_limit renders on these very items. An unknown token is a 400. Absent = no gate (both values), which keeps pre-existing callers byte-identical. Word-for-word the works/search parameter of the same name. This is NOT content_rating: content_rating is the AGE axis (what the GAME is rated), this is whether the material you would RENDER (cover, screenshots, synopsis) is safe to publish — a claimed work reads its wiki body's editorial flag, a bodyless one falls back to r18=nsfw. Unlike the search face this is a live registry predicate, so an editorial change takes effect immediately |
| `status` | query | 否 | string | Which population to browse. Absent or live = the public LIVE registry set, byte-identical to every pre-existing call. pending = the MODERATOR REVIEW QUEUE: the works whose claim is submitted and awaiting a curator's decision (claimed_by.state=pending), tenant-pinned. The queue view needs a SECOND credential — the moderator's own OAuth access token in Authorization: Bearer alongside the API key in X-API-Key (the dual-credential transport) — and refuses with 403 unless that token carries a user identity, was issued to a FIRST-PARTY site client bound to a catalog site (a third-party application is never a moderation surface), and holds the catalog.claim.review permission. The page is forcibly scoped to that client's own catalog site: site= may repeat it but naming any other site is a 403, never a silent re-point. Refusals are always explicit — this lane never degrades to the live set, because a moderator handed an empty page would read it as an empty queue. Platform-wide queues stay on the staff face. Passing claim_state= together with status=pending is a 400 (the parameter already IS that gate) 取值：live \| pending |
| `site` | query | 否 | string | Restrict to the works claimed by ONE site (the value claimed_by.site renders on these very items); absent = every tenant and every unclaimed work, which keeps pre-existing callers byte-identical. An unknown site matches nothing rather than erroring. Live registry predicate applied inside the page, so both the page and its next_cursor describe that one tenant — and a claim that moves tenant is reflected immediately, with no index to wait for. The works/search face deliberately does NOT take this parameter: its index carries no site facet, and a tenant queue must read the live registry anyway |
| `label_id` | query | 否 | integer (int64) | Only works attributed to this label (the catalog_work_label edge) |
| `label_rollup` | query | 否 | boolean | true/1 = widen label_id one hop DOWN the corporate graph, to the label's imprints and subsidiaries (wave 199) — the page a holding company that publishes nothing under its own name needs. Every row that came in through a child carries via_label naming that child; the company's own works carry none. The population is exactly work_count + imprint_work_count of labels/{id}. Spin-offs (spawned) and successions (succeeded_by) are NOT followed: those are other companies' catalogues. Ignored without label_id |
| `tag_id` | query | 否 | string | Only works carrying a source tag mapped to this canonical tag; up to 10 comma-separated ids are ANDed (a work must carry all of them), more than 10 or a non-positive/non-numeric entry is a 400 |
| `series_id` | query | 否 | integer (int64) | Only member works of this series |
| `engine_id` | query | 否 | integer (int64) | Only works built with this engine (the catalog_work_engine edge); browse the ids via GET /v1/catalog/engines |
| `platform` | query | 否 | string | vndb platform code (win/and/ios/...) — release-level and work-level rows unioned |
| `released_after` | query | 否 | string | YYYY-MM-DD, inclusive, over the EARLIEST release date per work |
| `released_before` | query | 否 | string | YYYY-MM-DD, inclusive |
| `ids` | query | 否 | string | Comma-separated work ids (max 100) — the batch-hydrate lane. This lane DOES NOT PAGINATE: every named id that also passes the other filters comes back in one response, next_cursor is always absent, and limit is ignored. Passing cursor= alongside it is a 400 |
| `sort` | query | 否 | string | id = ascending browse order (default); updated = newest-updated first 取值：id \| updated |
| `cursor` | query | 否 | string | Opaque keyset cursor from a prior next_cursor; omit for the first page. A 400 when ids= is also present: the batch-hydrate lane has no pages to walk |
| `limit` | query | 否 | integer (int64) | Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400. Ignored when ids= is present — that lane returns every hit — but a malformed value is still a 400 |
| `nsfw` | query | 否 | boolean | true/1 = include r18 works (default false = dropped). The parameter is caller-controlled but capability-gated: a key without the NSFW capability (nsfw_allowed, granted per key via the developer portal) is refused with 403 rather than degraded |
| `include` | query | 否 | string | Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs (default: none — the response is then byte-identical to the base contract). Unknown tokens are ignored. names carries latin + localized{}; intros carries one intro per language, detail-face shape; covers carries the portrait + banner slots with width/height/thumbhash; refs carries the work exact identity anchors, detail-face shape |
| `fields` | query | 否 | string | Comma-separated TOP-LEVEL keys of each ITEM to keep (default absent = every key, byte-identical to the base contract). The envelope — items/next_cursor — is never affected. id is always returned whether or not you name it. Unknown tokens are silently ignored, never a 400 (§3.5 clause 2). Trim-only: a kept key's value is byte-identical to the unprojected response. Applied AFTER include=, so naming an include-gated key (intros, labels, ratings, covers, refs, latin, localized) does NOT expand it — you still need both. The server is order- and duplicate-insensitive; WRITE THE TOKENS ALPHABETICALLY anyway, because the CDN keys on the raw URL and two orderings of the same selection are two cache entries |

```bash
curl "https://api.nextmoe.dev/v1/catalog/works" \
  -H "Authorization: Bearer nm_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/listCatalogWorksPublic
