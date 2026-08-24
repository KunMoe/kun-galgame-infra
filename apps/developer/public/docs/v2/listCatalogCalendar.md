# Release calendar · Public API v2（preview）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v2/catalog/calendar

Release calendar

One collection. month=/year= pick a window; precision= and status= select among the dated month, year-only, and undated views that were three v1 routes. Requires an application key. ids= is not accepted.

- 所属 API：Public API v2（preview）（/v2）
- 鉴权：Authorization: Bearer nmk_live_…
- scope：catalog:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `month` | query | 否 | string | Dated month window YYYY-MM. Default: current month in Asia/Tokyo. |
| `year` | query | 否 | string | Year-only window YYYY (v1 pending). Default with precision=year: current year in Asia/Tokyo. |
| `precision` | query | 否 | string | day, month, or year. year selects the year-only window. day and month use the dated month window. |
| `status` | query | 否 | string | released, dated, announced, cancelled, unknown. announced and unknown select the undated window. cancelled is empty until the catalog records cancellations. |

```bash
curl "https://api.nextmoe.dev/v2/catalog/calendar" \
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/v2/listCatalogCalendar
