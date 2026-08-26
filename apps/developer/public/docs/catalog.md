# 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

- 路径前缀：`/v1/catalog`
- 凭据：Authorization: Bearer nmk_live_…（机器 API 密钥,服务端持有）
- 端点数：37

## 端点

### 检索与反查

- `GET /v1/catalog/search` — Entity autocomplete over names / characters / labels / works / tags, projected to public briefs [详情](https://developer.nextmoe.dev/docs/catalog/searchCatalogEntitiesPublic.md)
- `GET /v1/catalog/works/search` — Works product search: free text + the works-list filter set, page-paginated, with opt-in facets and five sort lanes [详情](https://developer.nextmoe.dev/docs/catalog/searchCatalogWorksPublic.md)
- `GET /v1/catalog/lookup` — Reverse-lookup an external id via an EXACT anchor (killer feature); type=work|name|character|label, 404 on miss/hidden [详情](https://developer.nextmoe.dev/docs/catalog/lookupCatalogPublic.md)
- `POST /v1/catalog/lookup/batch` — Batch external-id reverse-lookup (≤100 pairs, per-pair type); misses return null blocks in order [详情](https://developer.nextmoe.dev/docs/catalog/lookupCatalogBatchPublic.md)
- `POST /v1/catalog/resolve` — Batch old id → canonical id (redirect flattening) for a given entity_type [详情](https://developer.nextmoe.dev/docs/catalog/resolveCatalogPublic.md)
- `GET /v1/catalog/redirects` — Keyset feed of id-convergence (merge) events for stored-id cleanup; filter by entity_type [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogRedirectsPublic.md)

### 作品

- `GET /v1/catalog/works` — Keyset works browse lane: the LIVE galgame registry set (claimed + bodyless) with conjunctive filters; sort=id|updated [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogWorksPublic.md)
- `GET /v1/catalog/works/{id}` — Frozen work record: identity + titles + exact cross-source refs + claim pointer; include=relations,credits [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkPublic.md)

### 作品子资源

- `GET /v1/catalog/works/{id}/covers` — Cover images of one work, paged — the block a store card or a shelf needs on its own [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkCoversPublic.md)
- `GET /v1/catalog/works/{id}/screenshots` — Screenshots of one work, paged, with dimensions and thumbhash [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkScreenshotsPublic.md)
- `GET /v1/catalog/works/{id}/tags` — Source tags of one work, paged, with the canonical mapping and the safety axis [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkTagsPublic.md)
- `GET /v1/catalog/works/{id}/characters` — The character roster of one work, paged, with voice credits [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkCharactersPublic.md)
- `GET /v1/catalog/works/{id}/credits` — Staff credits of one work grouped by role, paged over credit rows [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkCreditsPublic.md)
- `GET /v1/catalog/works/{id}/releases` — Release rows of one work, paged, each with its exact anchors and its own companies [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkReleasesPublic.md)
- `GET /v1/catalog/works/{id}/intros` — Synopses of one work, one per language, paged [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkIntrosPublic.md)
- `GET /v1/catalog/works/{id}/ratings` — Per-source ratings of one work, paged, with the vote histogram and the spread [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkRatingsPublic.md)
- `GET /v1/catalog/works/{id}/relations` — Related works of one work, paged — the block works/{id} only serves under include=relations [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkRelationsPublic.md)
- `GET /v1/catalog/works/{id}/series` — The series this work belongs to, paged [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkSeriesPublic.md)
- `GET /v1/catalog/works/{id}/links` — Non-identity outbound links of one work (official site, Steam, X, …), paged [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkLinksPublic.md)
- `GET /v1/catalog/works/{id}/engines` — Engines one work is built with, paged, each with an nsfw-aware work_count [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogWorkEnginesPublic.md)

### 发售与日历

- `GET /v1/catalog/releases` — Release-grain new-releases timeline: every dated release row, ports and re-editions included (date keyset) [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogReleasesPublic.md)
- `GET /v1/catalog/calendar` — Release calendar, one ISO month (date ASC keyset); default = the current Asia/Tokyo month [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogCalendarPublic.md)
- `GET /v1/catalog/calendar/pending` — Release calendar, one year's month-still-unknown bucket (id ASC keyset); default = the current Asia/Tokyo year [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogCalendarPendingPublic.md)
- `GET /v1/catalog/calendar/tba` — Release calendar, the global announced-but-undated bucket (id ASC keyset) [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogCalendarTBAPublic.md)

### 角色与人物

- `GET /v1/catalog/characters/{id}` — Character identity; include=works attaches the works it appears in with voice names [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogCharacterPublic.md)
- `GET /v1/catalog/names/{id}` — Credited identity (same-person grouping via public links); include=credits attaches works + roles [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogNamePublic.md)

### 厂牌与标签

- `GET /v1/catalog/labels` — Keyset label browse lane (id ASC); filter by kind, each row carries an nsfw-aware work_count [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogLabelsPublic.md)
- `GET /v1/catalog/labels/{id}` — Label (brand / circle / publisher …) identity; include=works attaches attributed works [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogLabelPublic.md)
- `GET /v1/catalog/labels/{id}/relation-graph` — Corporate-structure graph around a label: the connected family (parents, subsidiaries, imprints, spin-offs, succession) in one call [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogLabelRelationGraphPublic.md)
- `GET /v1/catalog/tags` — Keyset canonical-tag browse lane (id ASC); filter by tier / kind, each row carries an nsfw-aware work_count [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogTagsPublic.md)
- `GET /v1/catalog/tags/{id}` — Canonical tag (cross-source vocabulary): name / tier / kind / intros; include=works attaches the tagged works [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogTagPublic.md)

### 系列与引擎

- `GET /v1/catalog/series` — Keyset series browse lane (id ASC); each row carries an nsfw-aware work_count [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogSeriesPublic.md)
- `GET /v1/catalog/series/{id}` — Series record: identity + source anchor + intros; include=works attaches its member works in reading order [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogSeriesPublic.md)
- `GET /v1/catalog/engines` — Keyset engine browse lane (id ASC); each row carries an nsfw-aware work_count [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogEnginesPublic.md)
- `GET /v1/catalog/engines/{id}` — Engine record: name + nsfw-aware work_count + exact cross-source refs [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogEnginePublic.md)

### 变更流与统计

- `GET /v1/catalog/changes` — Incremental works changes feed ((updated,id) keyset; next_cursor always present — keep polling it for new rows) [详情](https://developer.nextmoe.dev/docs/catalog/listCatalogChangesPublic.md)
- `GET /v1/catalog/stats` — Slim catalogue counts: LIVE works per medium + the identity-family totals [详情](https://developer.nextmoe.dev/docs/catalog/getCatalogStatsPublic.md)

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog
