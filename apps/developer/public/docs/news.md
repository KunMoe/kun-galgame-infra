# 资讯 API

> NextMoe·未萌 开放 API —— ACGN 数据，以此为准。同一部作品在六个源各有一个页面，NextMoe 把它们对齐成一条记录，逐字段给出裁定后的标准答案，并附上答案取自哪个源。

- Base URL：https://api.nextmoe.dev
- 文档：https://developer.nextmoe.dev/docs
- MCP 端点：https://mcp.nextmoe.dev/mcp
- 调用与编辑都完全免费，没有付费档位，只有一层防滥用的限流。

**署名**：目前阶段使用 NextMoe·未萌 API，可以将 API 的名字标记为『鲲 Galgame 论坛』（如果你使用 Galgame 数据）或『LetMoe·一启萌』（如果你使用同人游戏数据）。

- 路径前缀：`/v1/news`
- 凭据：Authorization: Bearer nmk_live_…（机器 API 密钥,门户自助铸造即可,任意 scope 均可）
- 端点数：3

## 使用须知

- 授权制已于 2026-08-25 退役：这三条路径只要一把有效密钥，任意 scope 均可，news:read 不再存在申请或审批。免凭据的同一批内容在 /v2/news。
- 这是索引，不是转载：每条只有标题、摘要与题图，正文既不下发也不留存。每一项都恒带来源块与 source_url，读者要看全文只能回到媒体自己的站点——渲染时必须把来源与链接一并展示。
- 撤回即不可寻址：我们撤下的、以及上游原文已消失的条目会从列表中消失，按 id 直取则 404。这个 404 是契约而不是查询失败，不要重试，也不要拿缓存副本顶上。

## 端点

- `GET /v1/news` — Galgame news feed republished from partner sites, newest upstream publication first [详情](https://developer.nextmoe.dev/docs/news/listNewsPublic.md)
- `GET /v1/news/sources` — The source registry: display name, homepage, column entry point, publisher uid, and the attribution text to render [详情](https://developer.nextmoe.dev/docs/news/listNewsSourcesPublic.md)
- `GET /v1/news/{id}` — One news item; 404 once it is unpublished, withdrawn, or gone upstream [详情](https://developer.nextmoe.dev/docs/news/getNewsPublic.md)

---
本页来源 · NextMoe 开发者平台 · https://developer.nextmoe.dev/docs/news
