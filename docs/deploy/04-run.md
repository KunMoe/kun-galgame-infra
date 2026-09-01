# 4 · 日常启停

## 网络模型

所有容器都跑在**枢纽创建的 docker 网络** `kun-galgame-infra_default` 上,靠服务名互相解析。三种把下游接上来的方式:

| 方式 | 说明 | 适用 |
|---|---|---|
| **增量并行**(本文推荐,已实跑) | 先起 infra(它建网络+基础设施),再在各下游仓 `up`,各自加入同一外部网络 | 单机、已有 infra 在跑 |
| **伞状编排** | `website/compose.yaml` 用 `include:` 把三仓拼成一个 project,共享一套网络 | 生产、一键起整套 |

## 增量并行(已验证)

### 1) 枢纽
```bash
cd nextmoe-infra
docker compose up -d            # 9 个服务全起
docker compose ps              # 应全 healthy
```

### 2) moyu
moyu 的 `docker-compose.yml` 自带:
```yaml
networks:
  default: { name: kun-galgame-infra_default, external: true }
```
所以直接:
```bash
cd kun-galgame-patch
docker compose up -d moyu-api web
```

### 3) kungal（与 moyu 完全一致)
kungal 的 `docker-compose.yml` 现在和 moyu 一样,base 自带:
```yaml
networks:
  default: { name: kun-galgame-infra_default, external: true }
```
api/migrate 不声明 pg/redis 依赖(它们在 infra、跨 project 无法 `depends_on`),靠 `restart: unless-stopped` 重连。所以直接:
```bash
cd kun-galgame-forum
docker compose up -d kungal-api web
```

## 启停顺序

- **启动**:Postgres/Redis(healthy)→ infra oauth/image/galgame → infra web/wiki → moyu → kungal。`depends_on: condition: service_healthy` 已把 infra 内部顺序串好;下游 `restart: unless-stopped` 会在 infra 起来后自动重连。
- **停止**:倒序无所谓(无状态);直接 `down` 即可。

## 常用命令

```bash
# 看全生态(跨三个 compose project)
docker ps --format '{{.Names}}\t{{.Status}}' | grep -E 'kun-galgame-infra-|moyu-|kungal-' | sort

# 单仓状态 / 日志
docker compose ps
docker compose logs -f oauth
docker compose logs -f api   # kungal(在 kun-galgame-forum 目录)

# 停某仓(保留数据卷)
docker compose down                 # 在对应仓目录
# 停 + 清空数据卷(危险)
docker compose down -v              # 仅在 infra 目录会删 pg/redis/minio/opensearch 卷
```

> **数据卷只在 infra**(`pg`/`redis`/`minio`/`opensearch`)。moyu/kungal 无自己的卷(无状态),`down -v` 对它们无影响;但在 **infra** 目录 `down -v` 会清空全生态数据。

## 伞状编排(历史方案)

> 现生产为 **Dokploy 三应用共享 `dokploy-network`**([12-dokploy](./12-dokploy.md));infra 的本地 build
> `docker-compose.yml` 已于 wiki 退役 W5 移除,本节仅留作历史参考。

在 `website/` 放一个 `compose.yaml`:
```yaml
include:
  - nextmoe-infra/docker-compose.yml   # (已移除,历史)
  - kun-galgame-patch/docker-compose.yml
  - kun-galgame-forum/docker-compose.yml
# 注意:include 各子 compose 的 `name:` 与 moyu/kungal 的 external network 块在伞状下需调整
# (同一 project 共享一张网络,external 块应去掉)。详见 07-troubleshooting.md。
```
前面再套 Caddy/Traefik 按域名分流到各 web/api,并统一 `/img` CDN 域。

## 访问入口(本测试机)

| 入口 | URL |
|---|---|
| web 管理端 | http://localhost:15008 |
| galgame-wiki | http://localhost:15009 |
| moyu 补丁站 | http://localhost:15011 |
| kungal 论坛 | http://localhost:15013 |
| MinIO 控制台 | http://localhost:15003(minioadmin/minioadmin) |
