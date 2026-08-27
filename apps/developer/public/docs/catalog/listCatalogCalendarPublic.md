# Release calendar, one ISO month (date ASC keyset); default = the current Asia/Tokyo month · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/calendar

Release calendar, one ISO month (date ASC keyset); default = the current Asia/Tokyo month

Works whose EARLIEST year-carrying, non-deleted release falls inside the month — day precision and month precision alike (a month-precision work sorts at the head of its month, it is never pinned to the 1st). Same classification anchor as the works-list release_date, so a row's bucket and its printed date can never disagree. Year-precision works live in /v1/catalog/calendar/pending, undated ones in /v1/catalog/calendar/tba, and a work with no release row at all is in no bucket. Items are works-list rows verbatim, include= and all. Carries a bucket-level ETag (count + newest updated_at over the whole filtered set): an If-None-Match hit 304s before any page is loaded.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `month` | query | 否 | string | ISO month YYYY-MM; default = the CURRENT Asia/Tokyo month, echoed back in the response. A malformed value is a 400 |
| `olang` | query | 否 | string | Original-language gate: comma-separated olang values in the upstream BCP-47 spelling (ja, zh-Hans, en, …) or 'all' to switch it off. Default = the ja + zh* family. olang is an OPEN vocabulary, so an unrecognized value yields an empty bucket, never a 400 |
| `content_limit` | query | 否 | string | Comma-separated CLOSED vocabulary: sfw,nsfw — the EDITORIAL DISPLAY axis (the values claimed_by.content_limit renders), gating BUCKET MEMBERSHIP, the count and the meta frame alike. An unknown token is a 400 (a CLOSED vocabulary, unlike olang above). Absent = no gate (both values), byte-identical to the pre-A2-R5 bucket. NOT content_rating: that is the AGE axis (what the GAME is rated), this is whether the material you would RENDER is safe to publish. It rides in the ETag population key, so an sfw-gated and an ungated caller never share a validator |
| `cursor` | query | 否 | string | Opaque keyset cursor from a prior next_cursor; omit for the first page |
| `limit` | query | 否 | integer (int64) | Items per page 1-100 (default 20); above 100 is clamped to 100, a non-positive or non-numeric value is a 400 |
| `nsfw` | query | 否 | boolean | true/1 = include r18 works (default false = dropped) |
| `include` | query | 否 | string | Comma-separated rich-brief blocks: names,intros,labels,ratings,covers,refs — the works-list vocabulary verbatim (unknown tokens ignored) |

```bash
curl "https://api.nextmoe.dev/v1/catalog/calendar" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/listCatalogCalendarPublic
