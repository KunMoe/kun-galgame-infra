# API 文档

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## 四个 API

- [目录数据 API（只读）](https://developer.nextmoe.dev/docs/catalog.md) — `/v1/catalog`，37 个端点，Authorization: Bearer nm_live_…
- [游玩时长 API](https://developer.nextmoe.dev/docs/playtime.md) — `/v1/playtime`，5 个端点，Authorization: Bearer <用户访问令牌>
- [编辑提案 API](https://developer.nextmoe.dev/docs/edit.md) — `/api/v1/user/catalog/edit`，6 个端点，Authorization: Bearer <用户访问令牌>
- [资讯 API（授权制）](https://developer.nextmoe.dev/docs/news.md) — `/v1/news`，3 个端点，Authorization: Bearer nm_live_…
- [分销链接 API（授权制）](https://developer.nextmoe.dev/docs/store.md) — `/v1/store`，1 个端点，Authorization: Bearer nm_live_…
- [Public API v2（preview）](https://developer.nextmoe.dev/docs/v2.md) — `/v2`，74 个端点，Authorization: Bearer nmk_live_…

## 鉴权模型

- API 密钥（`Authorization: Bearer nm_live_…`）——在 https://developer.nextmoe.dev 控制台自助创建应用与密钥，自助可勾选的 scope 只有 catalog:read。
- 用户访问令牌（`Authorization: Bearer <access token>`）——游玩时长与编辑提案两个 API 读写的是某个用户自己的东西，用该用户经 OAuth 授权码 + PKCE 授权后的令牌，不是 API 密钥。
- news:read 是授权制：合作媒体授权给 NextMoe 的是一份索引，转授给谁由平台逐个决定。在 https://developer.nextmoe.dev 控制台提交申请并说明用途，批准后即可自助为密钥勾选它；没有它调 /v1/news 一律 403。
- store:read 同为授权制：分销链接按调用站签发，谁能拿由平台逐个决定。在 https://developer.nextmoe.dev 控制台提交申请并说明用途，批准后即可自助为密钥勾选它；没有它调 /v1/store 一律 403。
- `/v1/catalog/stats` 不要任何凭据，匿名即可调。

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs
