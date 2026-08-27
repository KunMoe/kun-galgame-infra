# Galgame news feed republished from partner sites, newest upstream publication first · 资讯 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/news

Galgame news feed republished from partner sites, newest upstream publication first

Every item carries its source block and source_url unconditionally: the partners authorised an INDEX, not a mirror. The article body is never served here and is not stored — preview plus banner is the whole authorisation, and readers reach the full text by following source_url to the partner's own site. Items we withdrew, and items whose upstream original has disappeared, are absent from this feed. Two lanes share the feed — 'news' (short bulletins) and 'column' (longer editorial pieces) — and every item states which one it came from.

- 所属 API：资讯 API（/v1/news）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：无需凭据

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `source` | query | 否 | string | Comma-separated source keys to keep (ymgal, galgame_hihyou); omit or 'all' for every source |
| `lane` | query | 否 | string | Comma-separated lanes to keep: news, column. Omit or 'all' for both. An unknown lane is rejected, never silently empty |
| `work_id` | query | 否 | integer (int64) | Keep only items anchored to this catalog work id |
| `published_after` | query | 否 | string | RFC3339 lower bound on the UPSTREAM publication time |
| `published_before` | query | 否 | string | RFC3339 upper bound on the UPSTREAM publication time |
| `cursor` | query | 否 | string | Opaque keyset cursor from a prior response's next_cursor; omit for the first page |
| `limit` | query | 否 | integer (int64) | Items per page 1-50 (default 20) |

```bash
curl "https://api.nextmoe.dev/v1/news" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/news/listNewsPublic
