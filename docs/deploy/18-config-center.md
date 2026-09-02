# 18 · 配置中心(运行期可调项)

> 你在这里:某个开关或阈值要改(比如关闭上传、把 trust 从 shadow 切到 live),不想改面板再重部署。
> 本篇说明配置中心管什么、值从哪来、怎么改、改了怎么验证。环境变量全集仍在
> [15-environment](./15-environment.md);配置中心是它之上的**运行期一层**,不是替代。

## 18.0 一句话模型

每个可调项是代码里声明的一个**键**(`internal/platform/settings/keys`),有类型、默认值、范围和说明。
进程内的生效值按三层解析,后者压前者:

| 层 | 载体 | 何时生效 | 谁能改 |
|---|---|---|---|
| 1. 代码默认 | `keys.go` 里的声明 | 编译期 | 改代码 + 部署 |
| 2. 环境变量地板 | 键声明的 `EnvVar`(和 15-environment 里的同名变量) | 进程启动时读一次 | 面板 / compose + 重部署 |
| 3. 数据库覆盖值 | 主库 `kun_galgame_infra` 的 `setting_overrides` | 写入后 30 秒内所有服务生效(oauth / image / artifact 经 Redis 推送为毫秒级) | 控制台 `/settings`,需 `settings.write` |

读取是进程内原子指针,请求路径零网络;每个服务每 30 秒向主库拉一次全表(几十行)。
刷新失败沿用上一份;启动时连续失败则只带 1+2 两层运行并打 `settings: initial load failed` 的
ERROR 日志。数据库里的一行若类型不符、越界或枚举外,会被忽略并退回 2/1 层,日志有 WARN。

**边界**:配置中心只收运行期可调项(开关、阈值、TTL、配额)。密钥、host/port/dsn/bucket、S3
path style、PG 连接池、OIDC 签名算法、aff URL 模板等启动期接线永远留在环境变量里。

## 18.1 现有键(W1,18 个)

| 键 | 类型 | 默认 | 环境变量地板 | 生效位置 |
|---|---|---|---|---|
| `auth.verification_code_ttl_minutes` | int 1..1440 | 15 | `KUN_AUTH_VERIFICATION_CODE_TTL_MINUTES` | oauth 注册/改邮箱验证码 |
| `image.upload_enabled` | bool | false | `KUN_IMAGE_UPLOAD_ENABLED` | image `POST /image/upload`(关闭时 503) |
| `artifact.upload_enabled` | bool | false | `KUN_ARTIFACT_UPLOAD_ENABLED` | artifact 上传/续传/完成三个入口 |
| `artifact.multipart_threshold_bytes` | int | 52428800 | `KUN_ARTIFACT_MULTIPART_THRESHOLD` | 超过此大小走分片 |
| `artifact.part_size_bytes` | int ≥ 5 MiB | 16777216 | `KUN_ARTIFACT_PART_SIZE` | 分片大小 |
| `artifact.presign_upload_ttl_seconds` | int | 3600 | `KUN_ARTIFACT_PRESIGN_UPLOAD_TTL_SECONDS` | 上传预签名有效期 |
| `artifact.presign_download_ttl_seconds` | int | 86400 | `KUN_ARTIFACT_PRESIGN_DOWNLOAD_TTL_SECONDS` | 下载预签名有效期 |
| `artifact.orphan_ttl_hours` | int | 24 | `KUN_ARTIFACT_ORPHAN_TTL_HOURS` | `artifact-gc` 孤儿上传回收 |
| `artifact.softdelete_ttl_hours` | int | 168 | `KUN_ARTIFACT_SOFTDELETE_TTL_HOURS` | `artifact-gc` 软删物理回收 |
| `artifact.reclaim_min_idle_seconds` | int | 3600 | `KUN_ARTIFACT_RECLAIM_MIN_IDLE_SECONDS` | 控制台回收文件的最小空闲时长 |
| `trust.scan_enabled` | bool | false | `KUN_TRUST_SCAN_ENABLED` | community 异步扫描是否发往 trust |
| `trust.check_enabled` | bool | false | `KUN_TRUST_CHECK_ENABLED` | community 同步词表门 |
| `trust.scan_mode` | enum shadow / live | shadow | `KUN_TRUST_SCAN_MODE` | trust 平台默认执法姿态(站点策略可再覆盖) |
| `trust.scan_sample_rate` | float 0..0.05 | 0 | `KUN_TRUST_SCAN_SAMPLE_RATE` | trust 平台默认抽检比例 |
| `ai.escalate_threshold` | float 0..1 | 0.4 | `KUN_AI_ESCALATE_THRESHOLD` | Tier1 → Tier2 升级阈值 |
| `ai.negative_sample_rate` | float 0..1 | 0.05 | `KUN_AI_NEGATIVE_SAMPLE_RATE` | 阴性抽样比例 |
| `ai.force_escalate` | 列表,每项 `site:kind` | 空 | `KUN_AI_FORCE_ESCALATE`(逗号分隔) | 强制升级的 (站点, 内容类型) 对 |
| `store.link_quota_per_client` | int | 5000 | `KUN_STORE_LINK_QUOTA_PER_CLIENT` | 每个调用站可铸链的商品数上限 |

单位写在键名里,环境变量的写法与迁入前完全一致。

## 18.2 怎么改

控制台 `oauth.kungal.com` → 侧栏「配置中心」(`/settings`)。页面按域列出全部键:生效值、来源
(`已覆盖` = 有数据库行;`默认` = 无行)、环境变量名、备注与最后修改人。持有 `settings.write`
(默认只有 ren;可在权限矩阵委派给 admin)的人可「编辑」或「撤销覆盖」,每次变更记审计
(旧值 → 新值 + 备注),页面底部可见。

页面显示的「来源」不含环境变量:oauth 进程看不见其他容器的环境。所以当某键显示「默认」而所属
服务的 compose 里仍设着同名变量时,该服务实际用的是变量值。真值看 18.3。

API(供脚本):
```
GET    /api/v1/admin/settings                总览(domains[].keys[]:key/kind/default/effective/source/override)
GET    /api/v1/admin/settings/audit?limit=   最近变更
PUT    /api/v1/admin/settings/{key}          {"value": <json>, "note": "...", "version": <上次看到的 version,可选>}
DELETE /api/v1/admin/settings/{key}?note=    撤销覆盖
```
`version` 不符返回 409;类型/范围/枚举不符返回 400 并带原因。

## 18.3 怎么验证真的生效了

每个服务启动时打一行 `settings: loaded keys=18 overrides=N env_floor=M`,之后每当某键的值或来源变化
打一行 `settings: applied key=... value=... source=db|env|default`。改完值后 30 秒内在目标服务的
容器日志里应看到对应的 `applied` 行;没看到 = 该服务没连上主库或没启动分发器,先查它的启动日志。

```bash
docker logs --since 2m <container> 2>&1 | rg 'settings: (loaded|applied|initial load failed)'
```

## 18.4 首次上线顺序(从环境变量迁过来)

1. 主库先建表:`go run ./cmd/migrate`(`setting_overrides` / `setting_audit_logs`;部署不会自动跑)。
2. 部署。此时所有键仍由 compose 里的环境变量地板决定,行为不变。
3. 在控制台把 compose 里现设的值写成数据库行(至少 `image.upload_enabled` /
   `artifact.upload_enabled` / `trust.scan_enabled` / `trust.check_enabled` = true,以及面板上
   设过的 `trust.scan_mode` / `trust.scan_sample_rate` / `ai.*`)。
4. 确认各服务日志出现 `applied ... source=db` 后,再从 compose 拔掉这些环境变量行。**先建行再拔
   env**,反过来会有一段上传被关闭的窗口。

## 18.5 怎么新增一个键

1. 在 `internal/platform/settings/keys/keys.go` 对应域声明(`settings.Bool/Int/Float/String/Enum/StringList`),
   给默认值、范围/枚举、中英说明;要保留环境变量地板就填 `EnvVar`。
2. 把它加进该域的 `Keys` 列表;`TestLiveRegistryIsWellFormed` 的 golden 表补一行。
3. 消费处在**使用时**读 `keys.X.Get()`,不要在构造时读进结构体字段(那正是迁走的东西)。
4. 测试里用 `settings.Override(t, keys.X, v)` 注入,cleanup 自动还原。
5. 不需要迁移、不需要改控制台:注册表即界面。

不进配置中心的:密钥与接线(见 18.0 边界)、一次性 cmd 工具专用的 `KUN_*`(它们仍直接读 env)。
