# 分销链接 API（授权制）

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

- 路径前缀：`/v1/store`
- 凭据：Authorization: Bearer nm_live_…（机器 API 密钥,但须带 store:read —— 授权制 scope,登录门户后在控制台申请,批准后即可自助勾选）
- 端点数：1

## 使用须知

- 一站一链：同一个商品，每个调用站拿到的短链都不一样，点击才归得到你站上。拿到 purchase_url 就原样用，别自己拼 DLsite 联盟地址——裸联盟链不经过计数器，等于这次点击白送。
- 去重是我们对 DLsite 的承诺，不是开关：同一条短链、同一个 JST 日、同一个指纹（IP 与 User-Agent 的 SHA-256）只算一次。刷次数刷不出量，只会让全生态的点击/销售比失真。
- 券链看活动：没有进行中的活动时 coupon_url 与 campaign 都是 null，前端要能在只有购买链接时正常渲染。
- 铸链是懒的、结果是稳的：第一次请求某个商品时才去铸短链，之后永远是同一条，所以自己缓存一份最省事。每个应用能铸链的不同商品数有上限，超了返回 403。

## 端点

- `GET /v1/store/purchase-links/{product_id}` — Your site's own DLsite purchase link for one product, plus the coupon link when a campaign is running [详情](https://developer.nextmoe.dev/docs/store/getStorePurchaseLinks.md)

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/store
