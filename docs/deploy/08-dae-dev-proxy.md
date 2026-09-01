# 8 · 附录:dae 透明代理下的开发环境配置(仅开发机)【历史】

> **本章已随 wiki 退役 W5 归档为历史**:它服务的本地 build `docker-compose.yml` 与 `docker-compose.dae.yml` override 均已移除。现行本地开发栈(`docker-compose.dev.yml`)全部 `network_mode: host` —— 容器出站属于 dae 的 `wan_interface`(本机流量)路径,**无需任何网桥固定/override**(见 8.1 的实测对照)。仅当你再引入 bridge 网络的 compose 项目时,本章的网桥固定配方才有参考价值。
>
> **本章只适用于「宿主机跑了 dae(或同类内核级透明代理)」的开发机器。**
> **生产环境必须纯净** —— 不做任何 dae 步骤。本章所有内容都是为了让「带透明代理的开发机」能正常构建/外呼,与生产部署无关。

## 8.1 为什么需要它

[dae](https://github.com/daeuniverse/dae) 用 eBPF 在接口上拦流量,且把「本机流量」和「转发流量」分开处理:

- **`wan_interface`** 只接管**本机进程**发起的出站(local-originated)。所以宿主机自己、以及 `--network=host` 的容器会被代理。
- **`lan_interface`** 才接管**经本机转发**的流量(dae 作为「中间设备」)。

Docker 默认 bridge 容器的包路径是 `容器 → veth → docker 网桥 → 内核 FORWARD → wan 出口`,在 dae 眼里是**转发流量**。dae 默认**不绑任何 `lan_interface`**,于是容器出站根本没进 dae、直连外网就超时(典型表现:`go mod download` 卡 `proxy.golang.org` i/o timeout,但宿主机本身能连)。

> 实测对照(本机):默认 bridge 容器连 `proxy.golang.org:443` **超时**;`--network=host` 容器**连通**;宿主机**连通**。

## 8.2 设计原则:生产纯净,dae 用 opt-in override

为不把「某台机器的代理妥协」烘进通用部署产物:

| 文件 | 作用 | 谁用 |
|---|---|---|
| `docker-compose.yml` | 基础栈,**不含任何代理/网桥定制**,默认自动网桥 `br-<hash>` | 所有机器(含生产) |
| `docker-compose.dae.yml` | **dev-only override**,把生态网络的 Linux 网桥固定成 `kungal-br0`,给 dae 一个稳定接口可绑 | 仅 dae 开发机 |

- **生产 / 无代理机**:`docker compose up -d`(只用基础文件)。固定网桥名在那里多余,且同主机跑两份栈会撞名。
- **dae 开发机**:`docker compose -f docker-compose.yml -f docker-compose.dae.yml up -d`。

> 下游 moyu / kungal **无需** dae override:它们以 `external` 方式加入 infra 的 `kun-galgame-infra_default` 网络,不创建网桥;infra 用不用 override 决定了那个网桥叫 `br-<hash>` 还是 `kungal-br0`,网络名不变,下游引用不受影响。

## 8.3 配置流程(dae 开发机)

前置(本机现状已满足,异机需自查):`net.ipv4.ip_forward=1`、`rp_filter` 为 `0` 或 `2`、无防火墙拦 FORWARD。

### Step 1 · 用 dae override 重建网络,拿到 `kungal-br0`
网桥固定名要先存在,dae 才能绑它。三个 compose 项目都挂在该网络上,需先停下游再停 infra 才能重建网络(`down` **不带 `-v`**,数据卷保留):

```bash
# 1) 停下游
cd kun-galgame-forum && docker compose down
cd ../kun-galgame-patch && docker compose down
# 2) 停 infra(网络归 infra project)
cd ../nextmoe-infra && docker compose down
# 3) infra 用 dae override 重新起 → 新网桥名
docker compose -f docker-compose.yml -f docker-compose.dae.yml up -d
ip -o link show kungal-br0 && echo "OK: kungal-br0 已就位"
# 4) 下游起回来
cd ../kun-galgame-patch && docker compose up -d api web
cd ../kun-galgame-forum && docker compose up -d api web
```

> 之后日常启停 infra 都要带 `-f docker-compose.yml -f docker-compose.dae.yml`(否则网络会用回自动网桥名)。建议设个别名:
> ```bash
> alias infra='docker compose -f docker-compose.yml -f docker-compose.dae.yml'
> ```

### Step 2 · 编辑 dae 配置(sudo)
```bash
sudo $EDITOR /etc/dae/config.dae
```
在 `global { … }` 段设(`lan_interface` 逗号分隔多接口):
```
global {
    wan_interface: auto                      # 保留:本机出站
    lan_interface: docker0, kungal-br0       # 新增:两个 docker 网桥
    auto_config_kernel_parameter: true       # 保留:兜底维护内核参数
    # …其余原样…
}
```
- **`docker0`** 覆盖**构建期**——legacy builder 的 `RUN`(`go mod download` / `pnpm install`)在默认 `docker0` 上跑。
- **`kungal-br0`** 覆盖**运行期**所有生态容器外呼(sync-vndb 调 VNDB、SMTP、B2 等)。

### Step 3 · 校验并 reload(sudo)
```bash
sudo dae validate -c /etc/dae/config.dae && sudo systemctl reload dae && systemctl is-active dae
```
> `reload` 不支持就 `sudo systemctl restart dae`(会短暂断本机代理)。

### Step 4 · 内核参数 / 防火墙(本机无需操作)
仅供异机排查:
```bash
cat /proc/sys/net/ipv4/ip_forward          # 需为 1
sysctl net.ipv4.conf.all.rp_filter         # 0 或 2;为 1 会丢被代理改道的转发包
sudo ufw status                            # 若开了 ufw,需放行 FORWARD(见 dae#364)
```

### Step 5 · 验证
```bash
# A) bridge 容器现在应能连(之前超时)
docker run --rm golang:1.25-bookworm bash -c \
  'timeout 12 bash -c "echo > /dev/tcp/proxy.golang.org/443" && echo "OK: bridge 经 dae 通了" || echo "FAIL: 仍不通"'
# B) 实跑构建,确认默认 proxy(无 goproxy.cn)经 dae 通过
cd nextmoe-infra && docker build --no-cache -f docker/go.Dockerfile \
  --build-arg CMD=galgame -t nextmoe-infra/galgame:daetest . 2>&1 | grep -iE "go mod download|error|Successfully"
```

## 8.4 注意

- **顺序**:先 Step 1(网桥出现)再 Step 2/3(dae 绑它);反了 dae 可能因接口不存在报错。
- 网桥名 `kungal-br0` 已固定,`down`/`up`(带 override)后不变,dae 配置无需再改。
- 新增其他自定义 docker 网络的项目,要么复用 `kun-galgame-infra_default`,要么同样固定网桥名并加进 `lan_interface`。
- 这是**纯开发机便利**;生产机不装 dae、不用 override、走默认 `proxy.golang.org`,本章可整章忽略。

**Sources:** [daeuniverse/dae](https://github.com/daeuniverse/dae) · [dae 文档(WAN/LAN interface)](https://github.com/daeuniverse/dae/blob/main/docs/en/README.md) · [dae#364(ufw 下 LAN 转发不通)](https://github.com/daeuniverse/dae/issues/364)
