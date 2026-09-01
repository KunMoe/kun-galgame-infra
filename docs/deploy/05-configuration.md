# 5 · 配置参考

每个服务读 12-factor 环境变量:**本地 dev** 走 `env_file`(`docker/*.env`),**生产** 走 `docker-compose.prod.yml` 内联的 `environment:` + Dokploy 面板的 `${VAR}`(见 [15-environment §15.8](./15-environment.md));前端 public 走 build args / `NUXT_PUBLIC_*`。下面按服务列关键项的**值**,**主机名一律用容器服务名**(`postgres`/`redis`/`oauth`/`galgame`/`image`/`minio`/`meilisearch`)。

## 跨服务共享值(测试)

| 项 | 测试值 | 谁用 |
|---|---|---|
| Postgres 密码 | `191007` | 全部(连同一 pg) |
| JWT 签名密钥 | `kun-docker-test-jwt-secret-change-me-please` | infra oauth 签发、各下游验签 |
| MinIO 凭据 | `minioadmin` / `minioadmin` | infra image、(下游图床经 image 服务) |
| Meili master key | `kun_docker_test_meili_master_key_change_me` | infra galgame、kungal |

> **JWT 密钥只需 infra 三服务一致**:oauth 用它 HS256 签发 access_token,image/galgame 用同一密钥**本地验签**——三者不一致 → image/galgame 401。下游 kungal/moyu **不本地验签**(走 `/oauth/userinfo` 网络校验),无需与 infra 共用:kungal 的 `JWT_SECRET` 只签自己的会话,moyu 没有 `JWT_SECRET`。详见 [15-environment.md §15.3](./15-environment.md)。

## infra · oauth(`nextmoe-infra/docker/oauth.env`)

| 变量 | 值 | 说明 |
|---|---|---|
| `KUN_FIBER_SERVER_HOST` | `0.0.0.0` | **容器内必须 0.0.0.0**,否则外部连不进 |
| `KUN_FIBER_SERVER_PORT` | `9277` | |
| `KUN_PG_HOST` / `_PASSWORD` / `_DATABASE` | `postgres` / `191007` / `kun_galgame_infra` | `config.validate()` 要求密码非空 |
| `REDIS_ENABLED` / `REDIS_HOST` | `true` / `redis` | |
| `JWT_SECRET` | (共享) | 必填 |
| `KUN_IMAGE_S3_*` | minio 一套 | oauth 内嵌图床 admin 端点要连 S3 + images 库 |
| `KUN_IMAGE_PUBLIC_BASE_URL` | `http://localhost:15002/kun-images` | 浏览器取图的公网前缀 |

## infra · image(`docker/image.env`)

要点同上,外加:`KUN_IMAGE_SERVICE_HOST=0.0.0.0`、`KUN_IMAGE_UPLOAD_ENABLED=true`、`KUN_IMAGES_PG_DATABASE=kun_images`、`KUN_IMAGE_S3_FORCE_PATH_STYLE=true`(MinIO 必须)。`KUN_IMAGE_PRESETS_PATH` 已在镜像内固定为 `/app/configs/image_presets.yaml`,勿覆盖。

## infra · galgame 面(由 catalog 服务承载,W3/W5)

`KUN_CATALOG_PORT=9281`、`KUN_GALGAME_PG_DATABASE`(生产 `kun_catalog`,本地 dev `kun_galgame_wiki`)、`KUN_MEILISEARCH_HOST=http://meili:7700`、`KUN_MEILISEARCH_API_KEY=`(共享 master key)。独立 galgame 服务(:9280)已退休。

## moyu · api(`kun-galgame-patch/docker/api.env`)

| 变量 | 值 |
|---|---|
| `KUN_DATABASE_URL` | `postgresql://postgres:191007@postgres:5432/kungalgame_patch?sslmode=disable` |
| `OAUTH_SERVER_URL` | `http://oauth:9277/api/v1` |
| `OAUTH_CLIENT_ID` / `_SECRET` | `df3ff6008d740bfacbe46aa8cf483cf2` / (注册时的明文) |
| `KUN_NEXTMOE_API_BASE` / `KUN_NEXTMOE_API_KEY` | `http://catalog:9281` / (internal-tier `nm_` key)——galgame 富读走 catalog internal 面(客户端拼 `/internal`);W5 起硬依赖 key,空则启动 fail-fast(旧名 `KUN_GALGAME_WIKI_BASE_URL` + legacy `/api` 读面已退役) |
| `KUN_IMAGE_SERVICE_BASE_URL` / `KUN_IMAGE_CDN_BASE` | `http://image:9278` / `http://localhost:15002/kun-images`(CDN 在 prod 模式**必填**) |
| `KUN_VISUAL_NOVEL_S3_*` | B2(补丁文件,**非**图床)— 测试机为 `__SET_ME__`,补丁下载需填真值 |

## kungal · api(`kun-galgame-forum/docker/api.env`)

kungal 仓库**自带的 api.env 是本地默认值**(密码 `kungal_dev_pw`、`meilisearch`、空 OAuth)。接 infra(生产则用 Dokploy Environment)必须改成:

| 变量 | 仓库默认 | **接 infra 改为** |
|---|---|---|
| `KUN_DATABASE_URL` 密码 | `kungal_dev_pw` | **`191007`** |
| `OAUTH_CLIENT_ID` / `_SECRET` | 空 | **`kungal-web` / (注册时明文)** ——空则 `requireEnv` 启动失败 |
| `JWT_SECRET` | 空 | **(共享密钥)** |
| `MEILISEARCH_KEY` | 空 | **(共享 master key)** ——否则被 meili 403 |
| `MEILISEARCH_URL` | `http://meilisearch:7700` | 不用改(infra 的 meili 已加 `meilisearch` 网络别名) |

`OAUTH_REDIRECT_URI` 改成 web 的 host 回调:`http://localhost:15013/auth/callback`。

## 前端 public 配置(build args)

各 web 的浏览器侧配置在镜像构建时烘焙(见 [02-build.md](./02-build.md))。要在**不重构**的前提下改,在对应 web 容器设 Nitro 约定名:

```
NUXT_PUBLIC_API_BASE=...
NUXT_PUBLIC_OAUTH_SERVER_URL=...    # moyu/kungal
NUXT_PUBLIC_OAUTH_CLIENT_ID=...
NUXT_PUBLIC_OAUTH_REDIRECT_URI=...
NUXT_PUBLIC_IMAGE_CDN_BASE=...
```
(moyu/kungal 的 `docker/web.env.example` 列了各自可覆盖的键。)

## 生产必须轮换的密钥清单

- `POSTGRES_PASSWORD`(及所有 `KUN_DATABASE_URL` / `KUN_PG_PASSWORD`)——别再用 `191007`。
- `JWT_SECRET`(三仓同步换)。
- 每个 OAuth client 的 secret(注册时重新生成,哈希入库)。
- MinIO `MINIO_ROOT_USER`/`PASSWORD`、Meili `MEILI_MASTER_KEY`。
- 各 S3/B2 access key、SMTP 密码。

生产应改用 `docker secret` 或外部 vault,而非明文 `env_file`。`docker/*.env` 已被 `.dockerignore` + `.gitignore` 挡在镜像与仓库之外。
