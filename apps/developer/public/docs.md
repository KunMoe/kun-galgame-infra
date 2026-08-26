# API 文档

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## 6 个 API

- [目录数据 API（只读）](https://developer.nextmoe.dev/docs/catalog.md) — `/v1/catalog`，37 个端点，Authorization: Bearer nmk_live_…
- [游玩时长 API](https://developer.nextmoe.dev/docs/playtime.md) — `/v1/playtime`，5 个端点，Authorization: Bearer <用户访问令牌>
- [编辑提案 API](https://developer.nextmoe.dev/docs/edit.md) — `/api/v1/user/catalog/edit`，6 个端点，Authorization: Bearer <用户访问令牌>
- [资讯 API](https://developer.nextmoe.dev/docs/news.md) — `/v1/news`，3 个端点，Authorization: Bearer nmk_live_…
- [分销链接 API（授权制）](https://developer.nextmoe.dev/docs/store.md) — `/v1/store`，2 个端点，Authorization: Bearer nm_live_…
- [Public API v2](https://developer.nextmoe.dev/docs/v2.md) — `/v2`，84 个端点，Authorization: Bearer nmk_live_…

## 鉴权模型

- 应用密钥（`Authorization: Bearer nmk_live_…`）——在 https://developer.nextmoe.dev 控制台自助创建应用与密钥，无需申请；自助可勾选的 scope 只有 catalog:read。/v2 只收 nmk_ 前缀的密钥，v1 两代都收。
- 用户访问令牌（`Authorization: Bearer <access token>`）——/v2/me 与 /v2/moderation（以及 v1 的游玩时长、编辑提案）读写的是某个用户自己的东西，用该用户经 OAuth 授权码 + PKCE 授权后的令牌，不是应用密钥。
- 资讯面不再是授权制（2026-08-25 退役）：/v1/news 只要一把有效密钥，任意 scope 均可；news:read 不再存在申请或审批。/v2/news 则匿名即可调。
- store:read 是授权制：分销链接按调用站签发，谁能拿由平台逐个决定。在 https://developer.nextmoe.dev 控制台提交申请并说明用途，批准后即可自助为密钥勾选它；没有它调 /v1/store 一律 403。
- `/v2/news`、`/v2/vocabularies`、`/v2/problems`、`/v2/catalog/stats` 与 `/v2/catalog/schemas/{object}`（以及 `/v1/catalog/stats`）不要任何凭据，匿名即可调。

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs
