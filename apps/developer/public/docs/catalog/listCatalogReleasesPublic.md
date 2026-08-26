# Release-grain new-releases timeline: every dated release row, ports and re-editions included (date keyset) · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/releases

Release-grain new-releases timeline: every dated release row, ports and re-editions included (date keyset)

The calendar one grain down. /v1/catalog/calendar places a WORK in the month of its EARLIEST release and shows it once, so a Switch port, a Steam re-issue or a Chinese localisation of a title released years ago is invisible there by construction. Here every release row is its own item, on its own date. is_first tells the two apart: true = the work's earliest dated release (a title genuinely coming out), false = a later edition of something already released — computed over the work's whole dated-release set, so it does not change when you narrow the feed. Each item carries the parent work as a works-list row (include= and all). Population: releases of LIVE galgame works whose date is known to at least the MONTH. Year-only and undated releases are deliberately absent — they cannot be ordered against a real date, and they already have homes in /v1/catalog/calendar/pending and /v1/catalog/calendar/tba. Carries a feed-level ETag (count + newest created_at + highest release id over the whole filtered set): an If-None-Match hit 304s before any page is loaded.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `sort` | query | 否 | string | date_desc (default) = newest first, the feed reading; date_asc walks forwards from the earliest release. The tiebreak inside one date is id ASC in BOTH directions. Keyset-paged over (date, id) — a cursor minted in one direction is refused in the other 取值：date_desc \| date_asc |
| `date_from` | query | 否 | string | YYYY-MM-DD, INCLUSIVE lower bound on the release's own date. Note the precision rule: a month-precision release is addressed at its month's START (2024-06 sits at 2024-06-00, i.e. before 2024-06-01), so date_from=2024-06-01 excludes it while date_from=2024-05-31 includes it. A malformed value is a 400 |
| `date_to` | query | 否 | string | YYYY-MM-DD, INCLUSIVE upper bound, same ordinal and the same precision rule as date_from. A malformed value is a 400 |
| `olang` | query | 否 | string | Original-language gate on the PARENT WORK: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …) or 'all' to switch it off. Default = the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty feed, never a 400. This is the work's original language, NOT the language of the release — for that use lang= below |
| `lang` | query | 否 | string | RELEASE-language gate: comma-separated BCP-47 tags, or 'all'. Default (absent) = NO gate. Matched over COALESCE(release.lang, work.olang): the store-SKU lanes (dlsite / getchu) create one release per work and record no language of its own, and such a SKU is by construction in its work's original language — so lang=ja matches them, while the item still prints the raw (possibly empty) release lang. An OPEN vocabulary: an unknown tag is an empty feed, never a 400 |
| `kind` | query | 否 | string | Comma-separated CLOSED vocabulary: default,digital,physical,trial,patch — the five strings each item's kind key prints. An unknown token is a 400. DEFAULT (absent) = default,digital,physical ONLY: this is a 发售动态 surface, so demos and translation patches are EXCLUDED unless you ask for them. Pass kind=default,digital,physical,trial,patch for the genuinely unfiltered feed, or kind=patch to watch localisations land |
| `official` | query | 否 | string | Gate on the release's official flag; absent = no gate. The flag is written by the VNDB release lane only, where false means a fan translation or unofficial edition — a row WITHOUT the key counts as OFFICIAL, because every other lane materialises store SKUs (dlsite worknos, getchu product pages) which are official by construction. official=true therefore drops only the explicitly-unofficial rows; official=false selects exactly those 取值：true \| false |
| `platform` | query | 否 | string | vndb platform code (win/and/ios/…) matched against the release's PRIMARY platform — the release leg of the works-list filter of the same name, verbatim. A release's platforms[] display list is not filtered on. OPEN vocabulary: an unknown code is an empty feed, never a 400 |
| `content_limit` | query | 否 | string | Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis on the parent work (the values claimed_by.content_limit renders), gating FEED MEMBERSHIP and the count alike. An unknown token is a 400 (a CLOSED vocabulary, unlike olang / lang / platform above). Absent = no gate (both values). NOT content_rating: that is the AGE axis (what the GAME is rated), this is whether the material you would RENDER is safe to publish. It rides in the ETag population key, so an sfw-gated and an ungated caller never share a validator |
| `cursor` | query | 否 | string | Opaque keyset cursor from a prior next_cursor; omit for the first page |
| `limit` | query | 否 | integer (int64) | Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400 |
| `nsfw` | query | 否 | boolean | true/1 = include releases of r18 works (default false = dropped) |
| `include` | query | 否 | string | Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim, applied to each item's attached work block (unknown tokens ignored). The RELEASE's own refs[] are always present and are not affected by this parameter |

```bash
curl "https://api.nextmoe.dev/v1/catalog/releases" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/listCatalogReleasesPublic
