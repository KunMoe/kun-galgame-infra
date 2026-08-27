# Slim catalogue counts: LIVE works per medium + the identity-family totals · 目录数据 API（只读）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/catalog/stats

Slim catalogue counts: LIVE works per medium + the identity-family totals

The product-facing size of the registry, no parameters and one payload for every caller. works counts LIVE rows only (stubs, merged-away rows and soft-deleted rows are not part of the catalogue) and total is the sum of by_medium, so the two can never disagree. R18 works ARE counted: these are aggregates with nothing renderable attached, and splitting them by nsfw would publish exactly what the r18 gate exists to hide. The INTERNAL dashboard (review queues, LLM verdicts, the anchor source × tier matrix, source freshness, orphan and claim-state breakdowns) is curation telemetry and stays on the S2S face.

- 所属 API：目录数据 API（只读）（/v1/catalog）
- 鉴权：无需凭据
- scope：无需凭据

无参数。

```bash
curl "https://api.nextmoe.dev/v1/catalog/stats"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/catalog/getCatalogStatsPublic
