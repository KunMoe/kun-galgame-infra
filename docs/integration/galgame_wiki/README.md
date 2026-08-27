# galgame 面 · 墓碑

> **这个面已经完全不存在了。本页是墓碑,不是契约。**
>
> 下面没有任何可调用的路由。如果你在找「galgame 的 API」,答案是 **`/v2`**。

## 现状

| | |
|---|---|
| `GET /v1/galgame`、`/v1/galgame/*` | **`410 Gone`**(仓内标准信封 `{code, message}`,`code=11`;保留 `Link: rel="successor-version"`) |
| `/internal/galgame/*` 平台工作流面(15 读 + 2 S2S feed + meta) | **已摘除**(2026-07-31,N5 W 窗 P5) |
| `/internal` 用户写面(06a)、提案面(06b)、`/api` staff 面 | **已摘除**(同上) |
| 后继面 | **`/v2`** — <https://developer.nextmoe.dev/docs/v2>(2026-08-27 前指向 `/v1/catalog`,该面随 wave R3 一并退役) |

代码侧只剩 `apps/api/internal/galgameapp/publicgone.go` 一个文件,作用就是回 410。
`internal/platform/galgame/**` 已于 2026-08-04 整包删除,只留 `perm`(纯权限词表,不碰库)。

## 数据也没了

2026-08-04(波次 149)`kun_catalog` 里 **galgame 表族 27 张表全部 DROP**。这不是「下线但留着」,是真的删了。

数据没有丢,是**搬走了**——搬进 catalog 原生表:

| 原来 | 现在 |
|---|---|
| `galgame.name_*` | `catalog_work_title` |
| `galgame.intro_*` | `catalog_work_intro` |
| `galgame_cover` / `galgame_screenshot` | `catalog_work_cover` / `catalog_work_screenshot` |
| `galgame_tag_relation` | `catalog_work_tag` |
| `galgame_official*` / `galgame_engine*` | `catalog_label` / `catalog_engine` 及其边表 |
| `galgame_series` | `catalog_series` |
| `galgame_revision` | `edit_revision`(12,602 条已重锚到 `catalog.work`) |

用户手写的中文简介单独抢救过:16,646 条全量入 `src_wiki.intro_snapshot`,其中 2,148 条经质量判卷恢复进 `catalog_work_intro`。

**终档**(DROP 前最后一份完整快照,27 表 + 幸存编辑引擎里的 galgame 行):`kungal-neo:/root/wave149/`,已做真恢复验证(逐表行数零不符)。查旧账走它。

## 三段退役史(为什么这页曾经有 146 行)

1. **2026-07-21/22 · 开放 API Phase 2 W5** — wiki 前端、`wiki.kungal.com` 域、独立 galgame 服务、legacy `/api` 读面、Basic-auth feeds、`*_WIKI_BASE_URL` env 全部退役;桥面的 29 条公开数据读迁到 `/v1`。
2. **2026-07-30 · wave 146** — `/v1/galgame` 这个「新家」自己摘牌,26 条公开 op 整体转 `410`。原定 Sunset 2026-10-31 的 90 天绞杀窗因证据充分提前执行:12 小时窗内正典面 `/v1/catalog` 146,236 次,`/v1/galgame` 1 次且是裸路径扫描器。
3. **2026-07-31 ~ 08-04 · wave 149 / N5 W 窗** — 剩下的 `/internal` 工作流面、staff 面、两条 S2S feed 一并摘除,编辑面改由 catalog 原生的 `edit_proposal` / `edit_revision` 引擎承载(编辑面本体化);表族 DROP。

**本页早先版本里的路由表、鉴权说明、字段契约,全部是历史记录,不是实现指引。** 需要考古就查 git 历史——那份内容刻意不留在这里,因为它读起来像还能用。

## 相关

- 正典契约:`docs/catalog`(本仓 Tier-A 源)
- 退役任务书:`refs/proj/149-w1-final-retirement.md`、`refs/plans/10-data-layer-retirement/`
- 编辑面本体化定案:`refs/plans/10-data-layer-retirement/03-editing-face-nativization.md`
