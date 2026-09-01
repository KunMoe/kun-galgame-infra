# 1 · 前置条件

## 主机

- **Docker Engine** + **Docker Compose v2**(`docker compose`,非旧版 `docker-compose`)。本测试机:Docker 29.5 / Compose 5.1。
- 约 **8GB 磁盘**(镜像 + 数据卷)、**4GB+ 内存**(Postgres + OpenSearch + 5 个 Node SSR 进程)。
- Linux x86_64。**构建架构必须与运行架构一致**(amd64 或 arm64 全程统一)——Nuxt 的 sharp 是预编译原生二进制。本文全程 linux-x64。

## 构建网络

构建需要出网拉取依赖,确保**构建主机可访问外网**:Go modules 走默认 `proxy.golang.org`、前端走 `registry.npmjs.org`,无需任何镜像源。`go.sum` / `pnpm-lock.yaml` 已锁定哈希,构建可复现。

## buildx / BuildKit 现状

本机**没有 `docker buildx`**(`docker buildx version` → unknown command),`docker compose build` 回落到 **legacy builder**(会打印 `requires buildx plugin` 警告 + `Sending build context to Docker daemon`,均可忽略)。因此:

- 三仓 Dockerfile **不使用** `--mount=type=cache`(BuildKit 专属),只靠普通层缓存。
- 安装 buildx 后可自行加回 cache mount 以加速重复构建。

## 仓库布局

三仓必须平级 checkout 在同一父目录:

```
website/
├── nextmoe-infra/           # infra
├── kun-galgame-forum/      # kungal
└── kun-galgame-patch/ # moyu
```

每仓的 Docker 资产:

```
<repo>/
├── docker-compose.yml
├── .dockerignore
└── docker/
    ├── *.Dockerfile        # go / cgo(仅 infra）/ nuxt
    ├── *.env               # 运行时配置(env_file,含密钥,不入镜像/git)
    ├── *.env.example       # 模板
    └── README.md           # 该仓的 Docker 说明
```

- infra 额外有 `docker/initdb.d/`(Postgres 建库脚本)和 `docker/cgo.Dockerfile`。
- **infra 的本地 build `docker-compose.yml` 已于 wiki 退役 W5 移除**:本地开发用仓根 `docker-compose.dev.yml`(GHCR 预构建镜像,见 `docs/dev-environment.md`),生产用 `docker-compose.prod.yml`。
- kungal / moyu 的 `docker-compose.yml` **同构**:base 都自带 `external` 共享网络、不定义 pg/redis(连 infra),靠 `restart` 重连;生产各自用 `docker-compose.prod.yml`(GHCR 镜像)。见 [04-run.md](./04-run.md)。

## 密钥占位

`docker/*.env` 里凡是 `__SET_ME__` 或测试值的(OAuth client secret、S3/B2 keys、SMTP 密码、JWT)在生产都要填真值。本测试部署用的占位见 [05-configuration.md](./05-configuration.md)。
