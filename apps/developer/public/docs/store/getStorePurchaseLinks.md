# Your site's own DLsite purchase link for one product, plus the coupon link when a campaign is running · 分销链接 API（授权制）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## GET /v1/store/purchase-links/{product_id}

Your site's own DLsite purchase link for one product, plus the coupon link when a campaign is running

Returns a short link that belongs to YOUR application, not a shared one: every calling site gets its own alias for the same product so the clicks can be attributed back to it. Send readers to purchase_url as-is — it is the only URL that gets counted, and a bare affiliate URL bypasses the counter entirely. coupon_url and campaign are null whenever no coupon campaign is running. Clicks are de-duplicated: one (link, JST calendar day, fingerprint) counts once, where the fingerprint is SHA-256 of the client IP and User-Agent. That de-duplication is a commitment we made to DLsite, not a knob. Links are minted lazily on first request and then stable forever, so calling this per page render is fine, but caching the result on your side is cheaper. Each application may mint links for a bounded number of distinct products; asking for more returns 403.

- 所属 API：分销链接 API（授权制）（/v1/store）
- 鉴权：Authorization: Bearer nm_live_…
- scope：store:read

| 参数 | 位置 | 必填 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| `product_id` | path | 是 | string | DLsite product number: RJ###### (doujin) or VJ###### (commercial), 6-8 digits |

```bash
curl "https://api.nextmoe.dev/v1/store/purchase-links/value" \
  -H "Authorization: Bearer nm_live_<YOUR_KEY>"
```

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/store/getStorePurchaseLinks
