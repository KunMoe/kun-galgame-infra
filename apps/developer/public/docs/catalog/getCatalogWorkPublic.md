# Frozen work record: identity + titles + exact cross-source refs + claim pointer; include=relations,credits · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/works/{id}

Frozen work record: identity + titles + exact cross-source refs + claim pointer; include=relations,credits

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `id` | path | 是 | integer (int64) | Catalog work id |
| `include` | query | 否 | string | Comma-separated heavy blocks: relations,credits (default: none) |
| `nsfw` | query | 否 | boolean | true/1 = serve r18 works and r18 relation ends (default false = hidden). The parameter is caller-controlled but capability-gated: a key without the NSFW capability (nsfw_allowed, granted per key via the developer portal) is refused with 403 rather than degraded, so it can never read an sfw page as the whole truth |
| `spoilers` | query | 否 | integer (int32) | Max tag spoiler level 0-2 (default 0 = safe): tags[] carries per-edge spoiler + per-tag sexual flags, and rows above this ceiling are omitted entirely. The axis is populated for the VNDB-derived vocabulary only — Bangumi/DLsite folksonomy publishes no spoiler or category concept, so those rows read 0/false |
| `fields` | query | 否 | string | Comma-separated TOP-LEVEL keys of this response to keep (default absent = every key, byte-identical to the base contract). id is always returned whether or not you name it. Unknown tokens are silently ignored, never a 400 (§3.5 clause 2). Trim-only: a kept key's value is byte-identical to the unprojected response, never reshaped. Applied AFTER include=, so fields=relations WITHOUT include=relations does not expand the block — the key is simply absent. Selecting a derived key loads what it needs (release_date and refs both read the release rows) but the dependency's own key still only appears if you named it. The server is order- and duplicate-insensitive; WRITE THE TOKENS ALPHABETICALLY anyway, because the CDN keys on the raw URL and two orderings of the same selection are two cache entries |

```bash
curl "https://api.nextmoe.dev/v1/catalog/works/1" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/getCatalogWorkPublic
