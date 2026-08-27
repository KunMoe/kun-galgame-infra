# Your application's own click counts, per link per JST day · 分销链接 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/store/me/stats

Your application's own click counts, per link per JST day

Per-day totals and de-duplicated uniques for every link this application has minted, purchase and coupon alike — the kind field tells them apart, and product_id / campaign_id say which link a row belongs to. Days are JST calendar days because the settlement month is DLsite's JST calendar month. The range is a closed interval of at most 92 days and defaults to the last 30. Days with no clicks are omitted entirely. uniques is the number settlement uses; total is the raw click count before de-duplication. The counts are synchronised from the redirector hourly, so the current day is always partial.

- 所属 API：分销链接 API（/v1/store）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：store:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `from` | query | 否 | string | First JST day to report, YYYY-MM-DD (inclusive). Defaults to 29 days before 'to' |
| `to` | query | 否 | string | Last JST day to report, YYYY-MM-DD (inclusive). Defaults to today in JST |

```bash
curl "https://api.nextmoe.dev/v1/store/me/stats" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/store/getStoreStatsMe
