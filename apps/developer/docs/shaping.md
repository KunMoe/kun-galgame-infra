---
title: 字段裁剪与批量读
eyebrow: API 基础
description: view / include / fields 三档如何叠加，以及 ids= 与 refs= 批量车道怎么一次读 100 条、怎么读 missing。
---

# 字段裁剪与批量读

默认瘦，按需胖。三个参数决定你拿到什么形状，另外两个决定你一次拿几条——用好它们，多数集成不需要 fan-out。

## 三档叠加 {#layers}

三个参数按固定顺序作用，每一档都是一个能写死在 spec 里的静态形状：

| 参数       | 做什么                               | 未知取值                 |
| ---------- | ------------------------------------ | ------------------------ |
| `view=`    | 选一档预设：`basic`（默认）或 `full` | `400 UNKNOWN_ENUM_VALUE` |
| `include=` | 点名要哪些块（逗号分隔）             | `400 UNKNOWN_INCLUDE`    |
| `fields=`  | 在上面的结果上做顶层键投影           | `400 UNKNOWN_FIELD`      |

```text
view=full   →  展开该族 full_set 里的块
+ include=  →  再并上你点名的块
+ fields=   →  最后只保留你要的顶层键（object 与 id 恒留）
```

> [!WARNING]
> 三者都不会静默忽略未知 token。`include=tags,nope` 是 `400`，不是「给你 tags 就算了」——200 + 缺块 + 零信号会被读成「数据里没有」，那是最贵的一种误解。

## view {#view}

`basic` 是身份内核：足够渲染一张卡片、做一次去重、建一条外链。`full` 展开这个族的「常用全集」，但**不是所有块**——像 `credits`（署名表）这种量大且属于明确意图的块不在 `full` 里，必须点名。

## include {#include}

每个族有自己的 include 词表，而且**列表面和详情面的词表不一样**——列表是批量水合的，只有能批量取的块才在列表词表里；只有逐条才有形状的块留在详情面和子资源面上。

| 面                                          | 可用 token                                                                                                                                                                                 |
| ------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `GET /v2/catalog/works/{id}`（详情，18 个） | `titles` `refs` `relations` `credits` `releases` `popularity` `ratings` `tags` `playtimes` `series` `platforms` `intros` `covers` `screenshots` `characters` `companies` `engines` `links` |
| `GET /v2/catalog/works`（列表，8 个）       | `titles` `refs` `intros` `covers` `companies` `ratings` `tags` `credits`                                                                                                                   |

不用记——发现面会告诉你，而且它和路由是同一份真相：

```bash
curl "https://api.nextmoe.dev/v2/catalog/schemas/work"

{
  "object": "object_schema",
  "target_object": "work",
  "include":         [ … ],   // 详情面吃的 token
  "full_set":        [ … ],   // view=full 在详情面展开的子集
  "list_include":    [ … ],   // 集合面吃的 token
  "list_full_set":   [ … ],
  "fields":          [ … ],   // 可编辑字段
  "creation_disabled": false
}
```

列表面拿不到的块，走作品的子资源面单独读：`works/{id}/characters`、`works/{id}/screenshots`、`works/{id}/relations` 等等——它们自带分页，也能单独缓存。

## fields {#fields}

`fields=` 在 `view` / `include` 之后做一次顶层键投影，用来把响应压到你真正要的那几个键。`object` 与 `id` 永远保留（否则结果就无法判别、无法回指）。

```http
GET /v2/catalog/works?fields=display_name,content_rating&limit=100

{ "object": "list", "items": [
  { "object": "work", "id": "207379", "display_name": "…", "content_rating": "all_ages" }
] }
```

投影发生在 `ETag` 计算之前，所以裁剪过的响应有它自己的验证器，条件请求照常工作。

## 批量读：ids 与 refs {#batch}

这是消灭 fan-out 的那个结构。任何可寻址的族都支持批量车道，一次最多 **100** 个：

| 参数    | 取什么                       | 例子                                                |
| ------- | ---------------------------- | --------------------------------------------------- |
| `ids=`  | 目录 id                      | `ids=207379,207380,207381`                          |
| `refs=` | 外部锚，`source:external_id` | `refs=vndb:v19658,bangumi:302835,dlsite:RJ01012345` |

- 批量车道**没有分页**：`next_cursor` 不会出现，`limit` 不参与。
- 超过 100 个是 `400 TOO_MANY_IDS`——自己分片，不要指望被截断。
- 请求了但不可见（不存在、被 nsfw 闸挡住、已合并）的键原样回在 `missing[]` 里。整个请求不会因为其中一个不存在就 404。
- `view` / `include` / `fields` 在批量车道上照常生效，所以「100 部作品各带标签和封面」是一次请求。

```http
GET /v2/catalog/works?refs=vndb:v19658,vndb:v99999&include=covers

{
  "object": "list",
  "items": [ { "object": "work", "id": "207379", … } ],
  "missing": ["vndb:v99999"]
}
```

> [!TIP]
> 把「先搜索、再对每条命中打一次详情」换成「一次集合读 + 一次 `ids=` 批量水合」，请求数从 N+1 掉到 2。这是限流下最划算的一次改写。
