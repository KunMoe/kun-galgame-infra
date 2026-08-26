# NextMoe 开发者平台

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

## 数据来源（六源）

- VNDB：身份主锚、关系、角色 traits
- Bangumi：中文名与条目、角色资料
- DLsite：同人与商业店铺条目
- ErogameScape：评分与发售信息
- Ci-en：创作者动态与厂牌外链
- Getchu：角色立绘、正文、截图

除六个上游站点外，未萌生态站点自己产出的条目、译名与整理，以及用户经编辑提案提交的修改，同样进入这一份记录。

## 鉴权模型

- 应用密钥（`Authorization: Bearer nmk_live_…`）——在 https://developer.nextmoe.dev 控制台自助创建应用与密钥，无需申请；自助可勾选的 scope 只有 catalog:read。/v2 只收 nmk_ 前缀的密钥，v1 两代都收。
- 用户访问令牌（`Authorization: Bearer <access token>`）——/v2/me 与 /v2/moderation（以及 v1 的游玩时长、编辑提案）读写的是某个用户自己的东西，用该用户经 OAuth 授权码 + PKCE 授权后的令牌，不是应用密钥。
- news:read 是授权制：合作媒体授权给 NextMoe 的是一份索引，转授给谁由平台逐个决定。在 https://developer.nextmoe.dev 控制台提交申请并说明用途，批准后即可自助为密钥勾选它；没有它调 /v1/news 一律 403。/v2/news 不受此限，无需凭据。
- `/v2/news`、`/v2/vocabularies`、`/v2/problems`、`/v2/catalog/stats` 与 `/v2/catalog/schemas/{object}`（以及 `/v1/catalog/stats`）不要任何凭据，匿名即可调。

## 三步开始

1. 用生态账号（NextMoe / 鲲 Galgame）登录门户，不必另外注册开发者身份。
2. 在控制台创建一个应用（每个账号最多 5 个），拿到独立的配额与用量视图。
3. 生成密钥并妥善保存（只显示一次），带上它请求只读端点；要读写用户自己的游玩记录或提交编辑提案，改用那个用户授权后的访问令牌。

完整端点参考见 https://developer.nextmoe.dev/llms-full.txt。

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/
