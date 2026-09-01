# 14 · 数据库备份与还原

> 本生态的**真数据全在 infra 的一套 Postgres(5 个库)**。Redis/MinIO/OpenSearch 多为缓存或派生数据(见 §6)。本篇详解 Postgres 的备份与还原:手动、自动、异地、各类还原场景与演练。
> [06-operations.md](./06-operations.md) 有速记;本篇是其展开。

## 0. 先搞清楚:备份什么

| 存储 | 内容 | 要不要备份 |
|---|---|---|
| **Postgres**(5 库) | 用户 / galgame / wiki / 图片元数据 / 论坛 / 补丁 —— **核心真数据** | **必须**,本篇重点 |
| Redis | 会话 / 缓存 / 限流 / 验证码 | 可选(多为临时,丢了重登/重发即可),已开 AOF |
| MinIO | 图片 blob | 可选:生产走 **Cloudflare R2**(自带冗余);自托管才需备份桶 |
| OpenSearch | 搜索索引 | 不必:**派生自 Postgres**,`reindex-catalog` 可重建,不必备份 |

**5 个库**:`kun_galgame_infra`(oauth/用户)、`kun_galgame_wiki`、`kun_images`、`kungalgame`(论坛)、`kungalgame_patch`(补丁)。

**容器名**:dev = `kun-galgame-infra-postgres-1`;**Dokploy 下用 `docker ps | grep postgres` 查实际名**。下文统一用变量:
```bash
PG=kun-galgame-infra-postgres-1     # ← 改成你的实际容器名
```
> 注:捕获备份输出时**不要加 `-t`**(TTY 会破坏文本/二进制流);还原时用 `-i`(喂 stdin)。

## 1. 备份方式选型

| 方式 | 工具 | 适用 | 取舍 |
|---|---|---|---|
| **逻辑备份**(本项目推荐) | `pg_dump` / `pg_dumpall` | 几百 MB 规模 | 可跨大版本还原、可选单库/单表;超大库重放慢 |
| 物理备份 | 卷快照 / `pg_basebackup` | 超大库、求快 | 与 PG **大版本绑定**;需一致性(停库或 basebackup) |
| **PITR**(进阶) | WAL 归档 + basebackup | 要"恢复到某一秒" | 配置复杂,见 §5 |

逻辑备份两种粒度,**两个都做**最稳:
- `pg_dumpall` —— **整集群**(5 库 + 角色/权限),灾难恢复 / 迁新机用。
- `pg_dump -Fc <db>` —— **单库** custom 格式(自带压缩、支持并行/选择性还原),日常 + 单站还原用,**还原最灵活**。

## 2. 手动备份

### 2.1 全集群(含角色,灾备/迁移)
```bash
docker exec "$PG" pg_dumpall -U postgres | gzip > cluster-$(date +%F).sql.gz   # 5 库 + 角色,gzip 压
```

### 2.2 单库(custom 格式,推荐日常)
```bash
for db in kun_galgame_infra kun_galgame_wiki kun_images kungalgame kungalgame_patch; do
  docker exec "$PG" pg_dump -U postgres -Fc -d "$db" > "${db}-$(date +%F).dump"   # -Fc=custom,自带压缩
done
```
`.dump` 用 `pg_restore` 还原(§4.2),支持 `-j` 并行、`-t` 选表。

### 2.3 只备结构 / 只备数据(按需)
```bash
docker exec "$PG" pg_dump -U postgres -s -d kungalgame > kungalgame-schema.sql        # 仅结构 -s
docker exec "$PG" pg_dump -U postgres -a -Fc -d kungalgame > kungalgame-data.dump     # 仅数据 -a
```

> **备份含用户隐私**(邮箱、密码哈希)。异地上传前**加密**:`gpg -c cluster-*.sql.gz`(对称加密),或用服务端加密的桶。

## 3. 自动备份 + 异地 + 保留

### 3.1 备份脚本(每日 + 自动清旧 + 上云)
`/home/kun/backup-pg.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail
PG=kun-galgame-infra-postgres-1
OUT=/home/kun/backups; mkdir -p "$OUT"
F="$OUT/cluster-$(date +%F_%H%M).sql.gz"
docker exec "$PG" pg_dumpall -U postgres | gzip > "$F"          # 1) 备份
find "$OUT" -name 'cluster-*.sql.gz' -mtime +14 -delete         # 2) 本地只留 14 天
rclone copy "$F" r2:kun-db-backups/                             # 3) 异地:传 R2(先 rclone config 配好;或用 aws s3 cp / mc cp)
```
```bash
chmod +x /home/kun/backup-pg.sh && /home/kun/backup-pg.sh      # 先手动跑一次验证
```

### 3.2 定时(cron)
```bash
( crontab -l 2>/dev/null; echo "30 3 * * * /home/kun/backup-pg.sh >> /home/kun/backups/backup.log 2>&1" ) | crontab -
#  每天 03:30 跑;日志追加到 backup.log
```

### 3.3 Dokploy 内置备份(生产更省事)
Dokploy 面板支持**数据库定时备份到 S3**:Settings → **Destinations** 填 R2/S3 凭证 → 数据库应用加 **Backup** 计划(cron + 目标桶)。生产直接用它,免自写脚本。

### 3.4 3-2-1 原则
至少 **3 份**副本、**2 种**介质、**1 份异地**。本机 `find -mtime` 留近期 + R2 留长期 + 偶尔下载一份到本地,即满足。

## 4. 还原

> 还原会**覆盖/删数据,不可逆**。先确认备份可用(§4.5),能演练就先在测试库/测试机演练。

### 4.1 全集群还原(灾难恢复 / 迁新机)
pg18 把卷挂载点改到了 `/var/lib/postgresql`,且 `initdb.d` 会在空库初始化时建 4 个库、与 `pg_dumpall` 的 `CREATE DATABASE` 冲突 —— 所以用**临时容器**(不挂 initdb.d、不设 `POSTGRES_DB`)回灌,最干净(即 pg16→18 升级实测用的法):
```bash
docker compose down && docker volume rm kun-galgame-infra_pg              # 清空目标(危险!确认有备份)
docker run -d --name pgrestore -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=<pw> \
  -v kun-galgame-infra_pg:/var/lib/postgresql postgres:18-alpine          # 临时纯净 pg18
until docker exec pgrestore pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done   # 等就绪
gunzip -c cluster-YYYY-MM-DD.sql.gz | docker exec -i pgrestore psql -U postgres        # 回灌(唯一无害错:role "postgres" already exists)
docker stop pgrestore && docker rm pgrestore                              # 卷已填好,删临时容器
docker compose up -d                                                      # 正常起栈(卷已初始化,initdb.d 不再触发)
```

### 4.2 单库还原(custom 格式 `.dump`)
某个库坏了/要回滚,只还原它、不动其它库:
```bash
# --clean --if-exists:先删旧对象再建;-j 4:并行加速
docker exec -i "$PG" pg_restore -U postgres -d kungalgame --clean --if-exists -j 4 < kungalgame-YYYY-MM-DD.dump
```
- 库不存在则先建:`docker exec "$PG" psql -U postgres -c 'CREATE DATABASE kungalgame;'`,再 `pg_restore`(去掉 `--clean`)。
- 要"全成功或全回滚"用 `-1`(单事务),但 `-1` 不能和 `-j` 并存。

### 4.3 单表 / 部分还原
```bash
docker exec -i "$PG" pg_restore -U postgres -d kun_galgame_wiki -t galgame < kun_galgame_wiki-*.dump   # 仅 galgame 表
```
plain SQL 备份(`.sql`)则:`gunzip -c x.sql.gz | docker exec -i "$PG" psql -U postgres -d <db>`。

### 4.4 还原到新服务器(迁移)
新机装 Docker + 起 infra 基础设施(`up -d postgres ...`)→ 拷备份过去 → 按 §4.1(全量)或逐库 §4.2 还原 → 起全部服务 → `go run ./cmd/reindex-catalog` 重建 OpenSearch 索引。

### 4.5 验证还原(务必)
```bash
docker exec "$PG" psql -U postgres -d kun_galgame_infra -tAc 'select count(*) from users'        # 对比备份时的行数
docker exec "$PG" psql -U postgres -tAc "select datname from pg_database where datistemplate=false order by 1"   # 5 库齐全
```
**定期演练**:每隔一段把最新备份还到临时库/测试机,确认能起来 —— **没演练过的备份等于没有**。

## 5. 进阶:PITR(可选,近零丢失)
要"恢复到任意一秒"(RPO≈0):postgres 开 `archive_mode=on` + `archive_command` 把 WAL 推到对象存储,定期 `pg_basebackup`;恢复时用 basebackup + 重放 WAL 到目标时刻。本项目数据量小、**每日逻辑备份已足够**,一般不必上 PITR;真要做用 **pgBackRest** 或 **WAL-G**(封装好归档/恢复),见 PG 官方 Continuous Archiving 文档。

## 6. 其它数据存储

- **Redis**:已开 AOF(`--appendonly yes`),数据在 `kun-galgame-infra_redis` 卷。要备份:`docker exec <redis> redis-cli BGSAVE` 后拷 `/data` 里的 `dump.rdb`/AOF,或直接卷快照。多为缓存/会话,丢失影响小。
- **MinIO**:生产用 **R2**(自带多副本,无需自备);自托管才需 `mc mirror local/kun-images r2/kun-images-backup` 镜像桶。
- **OpenSearch**:**不用备份**,派生自 Postgres,还原后 `go run ./cmd/reindex-catalog` 重建即可。

## 7. 速查

| 目的 | 命令 |
|---|---|
| 全集群备份 | `docker exec "$PG" pg_dumpall -U postgres \| gzip > cluster-$(date +%F).sql.gz` |
| 单库备份 | `docker exec "$PG" pg_dump -U postgres -Fc -d <db> > <db>.dump` |
| 全集群还原 | 临时容器法(§4.1) |
| 单库还原 | `pg_restore -U postgres -d <db> --clean --if-exists -j4 < <db>.dump` |
| 验证 | `psql -U postgres -d <db> -c 'select count(*) …'` + 库清单 |
| 自动 | `backup-pg.sh` + cron(§3),或 Dokploy 内置备份到 S3 |
