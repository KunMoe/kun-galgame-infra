# 服务器开荒(裸机初始化 · Debian 13)

> 从一台**全新 Debian 服务器**到「可以开始部署」的全部初始化:登录 → 系统更新 →
> 装基础工具 → 建用户 `kun` → SSH 加固(禁 root / 禁密码登录)→ 防火墙 + fail2ban →
> 自动安全更新 → 克隆仓库。做完接 [QUICKSTART.md](./QUICKSTART.md) 部署三站。
>
> **顺序至关重要 —— 别把自己锁在门外**:先建好 `kun` 用户 + 公钥 + sudo,并
> **另开一个新终端验证能用 kun 登录**,再去禁用 root / 密码登录。每次改 SSH/防火墙后
> **保留当前会话**,用新会话验证通过,再关旧会话。再留一个 VPS 厂商的 Web Console
> 作最后兜底。

**本文占位符**(按你的实际值替换):
- `<PORT>` — SSH 端口(你用 `ssh -p <PORT>` 的那个)
- `<SERVER_IP>` — 服务器公网 IP
- `<your_key>` — 你本地的私钥文件(如 `~/.ssh/id_ed25519`)

> 本地若还没有密钥:`ssh-keygen -t ed25519 -C "kun@laptop"`,公钥是 `~/.ssh/<key>.pub`。

---

## 本地 SSH config(建议;经跳板机时必看)

把连接参数写进**本地** `~/.ssh/config`,之后直接 `ssh kungal-neo` 即可,不用每次敲 `-p -i`:

```sshconfig
Host kungal-neo
    HostName <SERVER_IP>
    Port <PORT>
    User kun
    IdentityFile ~/.ssh/<your_key>
    ServerAliveInterval 60
```

**目标机只能经「跳板机」到达时**(它在内网,或路由要先过一台已知主机),再加一台跳板别名并 `ProxyJump`:

```sshconfig
Host kungal-jump            # 跳板机(你能直连的那台)
    HostName <JUMP_IP>
    Port <JUMP_PORT>
    User root
    IdentityFile ~/.ssh/<your_key>

Host kungal-neo             # 目标机:经跳板到达
    HostName <SERVER_IP>
    Port <PORT>
    User kun
    IdentityFile ~/.ssh/<your_key>
    ProxyJump kungal-jump
    ServerAliveInterval 60
```

> **ProxyJump 只是隧道**:公钥认证发生在「**本地 ↔ 目标机**」之间,所以公钥要装在
> **目标机的 `kun`** 上,**跳板机不持有它**。下面 §2 的 `ssh-copy-id … kungal-neo` /
> 平时的 `ssh kungal-neo` 都会自动走这条跳板链路。配好别名后,§0/§9 里凡是
> `ssh -p <PORT> -i … kun@<IP>` 都可简写成 `ssh kungal-neo`。

---

## 0. 首次登录(初始 root)

```bash
ssh -p <PORT> -i ~/.ssh/<your_key> root@<SERVER_IP>
```
若 VPS 只给了 root **密码**:先用密码登录,§2 会给 `kun` 配公钥、§3 再关掉密码登录。

## 1. 系统更新 + 基础工具

```bash
apt update && apt -y full-upgrade
apt -y install \
  sudo curl wget git vim ufw fail2ban ca-certificates gnupg \
  btop tmux unzip rsync jq ncdu tree dnsutils \
  unattended-upgrades apt-listchanges
# fastfetch:trixie 仓库自带;万一装不到,用官方 .deb 兜底
apt -y install fastfetch || {
  curl -fsSLo /tmp/ff.deb https://github.com/fastfetch-cli/fastfetch/releases/latest/download/fastfetch-linux-amd64.deb
  apt -y install /tmp/ff.deb && rm -f /tmp/ff.deb
}
# 时区 + 时间同步(用系统自带的 timesyncd,不额外装 chrony)
timedatectl set-timezone Asia/Shanghai
timedatectl set-ntp true
```

## 2. 新建用户 `kun`(sudo + 公钥)

```bash
adduser kun                          # 交互式设强密码
usermod -aG sudo kun                 # 加入 sudo 组

# 给 kun 配 SSH 公钥(方式 A/B 在服务器上做;方式 C 从本地做,见下)
install -d -m 700 -o kun -g kun /home/kun/.ssh
# 方式 A(省事):复制 root 当前已授权的 key
cp /root/.ssh/authorized_keys /home/kun/.ssh/authorized_keys 2>/dev/null || true
# 方式 B(推荐):只放你要授权的公钥
#   vim /home/kun/.ssh/authorized_keys   # 粘贴一行 ssh-ed25519/ssh-rsa AAAA... your-comment
chmod 600 /home/kun/.ssh/authorized_keys
chown -R kun:kun /home/kun/.ssh
# 关键权限:家目录不能被同组/其他人可写,否则 sshd 的 StrictModes 会【忽略 key】、
#    回落到密码登录 —— 这是「配了 key 还在要密码」最常见的原因(很多 VPS 默认 /home/kun
#    是 755 反而没事,但有的给成 775/g+w 就会中招)。
chmod go-w /home/kun
```

> 方式 C(最稳,从**本地**装,自动走 ProxyJump 跳板):
> ```bash
> ssh-copy-id -i ~/.ssh/<your_key>.pub kungal-neo   # 输一次 kun 密码,公钥即追加进 kun
> ```
> 装完仍要密码 → 多半是上面那条 `chmod go-w /home/kun`(详见文末「排错」)。

> **现在另开一个本地终端验证(别关 root 会话!)**:
> ```bash
> ssh kungal-neo          # 或 ssh -p <PORT> -i ~/.ssh/<your_key> kun@<SERVER_IP>
> sudo whoami             # 应输出 root
> ```
> **必须是免密钥直接进**(不再提示 `kun@...'s password:`),才继续 §3。还要密码见文末「排错」。

(可选)给 `kun` **免密 sudo**(自动化 / Dokploy 多机会用到;牺牲一点安全):
```bash
echo 'kun ALL=(ALL) NOPASSWD:ALL' | sudo tee /etc/sudoers.d/kun
sudo chmod 440 /etc/sudoers.d/kun
```

## 3. SSH 加固(禁 root + 禁密码登录)

**确认 §2 的 kun 公钥登录 + sudo 已通过**,再做。用 drop-in 配置(不动主文件):

```bash
sudo tee /etc/ssh/sshd_config.d/99-hardening.conf >/dev/null <<'EOF'
Port <PORT>
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no
PubkeyAuthentication yes
AllowUsers kun
MaxAuthTries 3
LoginGraceTime 30
X11Forwarding no
ClientAliveInterval 300
ClientAliveCountMax 2
EOF
sudo sshd -t        # 校验语法;有报错就别重启,先改对
```
重启 sshd:
```bash
sudo systemctl restart ssh
```
> **socket 激活的坑**:若 `systemctl status ssh.socket` 显示 active,则 `sshd_config`
> 里的 `Port` **不生效**(端口由 socket 控制)。改用 service 即可:
> ```bash
> sudo systemctl disable --now ssh.socket
> sudo systemctl enable  --now ssh.service
> ```

> **再次另开终端验证**:`ssh -p <PORT> -i key kun@ip` 能进;`ssh root@ip` 被拒、
> 密码登录被拒。**全部通过后**才关掉旧的 root 会话。

## 4. 防火墙(ufw)

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow <PORT>/tcp comment 'ssh'    # 先放行 SSH 端口,否则 enable 后立刻断连
sudo ufw allow 80/tcp  comment 'http'
sudo ufw allow 443/tcp comment 'https'
sudo ufw --force enable
sudo ufw status verbose
```
> 上线后:80/443 给 Dokploy 的 Traefik。**Dokploy 面板 `3000` 由 Docker 直接发布、ufw
> 管不住它**(Docker 绕过 ufw 自写 iptables)→ 要用域名 / SSH 隧道 + 云厂商防火墙 /
> `ufw-docker` 单独收口(见 [QUICKSTART §9](./QUICKSTART.md))。要**隐藏源站 IP / 只放行
> Cloudflare 段**见 [QUICKSTART §10](./QUICKSTART.md)。

> **入站只需要这几个端口,别的都不用开**:
> - **`<PORT>` SSH / 80 / 443**(+ 初装临时的 `3000`)。
> - **邮件不需要任何入站规则**:本项目发信走**外部 SMTP 中继**(mxroute `tuesday.mxrouting.net:587`),是**出站**连接 —— ufw 默认放行出站即可,源站不收信、不跑邮件服务,**不开 25/465/587/993/143**。
>   - 唯一相关的是**出站**:个别 VPS 默认封**出站 25**(反垃圾),但本项目用 **587(submission)**,基本不受影响;若发信失败先确认出站 587 没被运营商挡。
> - **数据库 / Redis / oauth / image / galgame / meilisearch / minio 全是容器内网通信**(prod compose 用 `expose` 不发布宿主端口),**无需任何入站规则**;临时要从笔记本连库就走 **SSH 隧道**(`ssh -L`),别长期开 pg 端口。

## 5. fail2ban(挡 SSH 爆破)

```bash
sudo tee /etc/fail2ban/jail.local >/dev/null <<EOF
[DEFAULT]
backend  = systemd
bantime  = 1h
findtime = 10m
maxretry = 5

[sshd]
enabled = true
port    = <PORT>
EOF
sudo systemctl enable --now fail2ban
sudo fail2ban-client status sshd        # 看 jail 是否生效
```

## 6. 自动安全更新

```bash
sudo dpkg-reconfigure -plow unattended-upgrades   # 选 <Yes>
# 或非交互启用:
printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' \
  | sudo tee /etc/apt/apt.conf.d/20auto-upgrades
```

## 7.(可选)Swap —— 小内存机建议

若打算在**本机构建重镜像**(cgo + 4×Nuxt),内存 ≤ 4–8G 建议加 swap(改用 GHCR 预构建
则可不加,见 [QUICKSTART §3](./QUICKSTART.md)):
```bash
sudo fallocate -l 4G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
echo 'vm.swappiness=10' | sudo tee /etc/sysctl.d/99-swap.conf && sudo sysctl -p /etc/sysctl.d/99-swap.conf
```

## 8.(可选)主机名 + 基础内核加固

```bash
sudo hostnamectl set-hostname kun-prod
# 同步写进 /etc/hosts,否则之后每条 sudo 都会报 "unable to resolve host kun-prod"
grep -q 'kun-prod' /etc/hosts || echo '127.0.1.1 kun-prod' | sudo tee -a /etc/hosts
sudo tee /etc/sysctl.d/99-hardening.conf >/dev/null <<'EOF'
net.ipv4.conf.all.accept_redirects = 0
net.ipv6.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects   = 0
net.ipv4.tcp_syncookies            = 1
EOF
sudo sysctl --system
```

## 9. 以 `kun` 登录并克隆仓库

```bash
ssh -p <PORT> -i ~/.ssh/<your_key> kun@<SERVER_IP>
mkdir -p ~/app && cd ~/app
git clone https://github.com/next-moe/nextmoe-infra.git
git clone https://github.com/KunMoe/kun-galgame-forum.git
git clone https://github.com/KunMoe/kun-galgame-patch.git
fastfetch        # 欣赏一下你的新机器 :)
```
> 仓库若是私有:先给服务器配 **deploy key**(`ssh-keygen` 后把公钥加到 GitHub repo
> 的 Deploy keys)或用 PAT 走 https。

## 10. 下一步:部署

服务器开荒完成 → 接 [QUICKSTART.md](./QUICKSTART.md):
- 装 **Dokploy**(`dokploy.com/install.sh` 会**自动装 Docker**,所以本篇不单独装 Docker)
- 建 3 个 Compose 应用 → 填环境变量 → 挂域名(Traefik 自动 SSL)→ §10 隐藏源站 IP

## 排错:配了密钥却还要输密码

`kun@<IP>'s password:` 是 **SSH 登录**回落到了密码(= 公钥认证失败),**和 `sudo` 密码无关**(`NOPASSWD sudo` 只影响登录**之后**的 `sudo`,不影响这个登录提示)。按概率排查:

1. **权限(最常见)** —— sshd `StrictModes` 要求家目录 / `.ssh` / `authorized_keys` 不被同组或其他人可写,且属主是 kun:
   ```bash
   chmod go-w /home/kun
   chmod 700 /home/kun/.ssh
   chmod 600 /home/kun/.ssh/authorized_keys
   chown -R kun:kun /home/kun/.ssh
   ls -ld /home/kun /home/kun/.ssh        # 结尾别给 group/other 留 w(不是 drwxrwx…)
   ```
2. **看 sshd 日志(Debian 13 用 journald,没有 `/var/log/auth.log`)**:
   ```bash
   journalctl -t sshd -n 30 --no-pager | grep -iE 'refused|bad ownership|modes|publickey|Accepted'
   ```
   出现 `Authentication refused: bad ownership or modes for …` → 就是权限(回第 1 步)。
3. **本地** verbose(在你**笔记本**跑,不是服务器):
   ```bash
   ssh -v kungal-neo 2>&1 | grep -iE 'Offering|Server accepts key|publickey|password'
   ```
4. **经跳板**:公钥要装在**目标机的 kun**,跳板机不持有它(ProxyJump 只转发)。
5. **authorized_keys 路径被改**:`sudo sshd -T | grep -i authorizedkeysfile`(默认 `.ssh/authorized_keys`)。
6. **RSA 被新 sshd 拒**(少见;同把 key 在别的机器能用就基本排除):日志报 `signature algorithm ssh-rsa not in PubkeyAcceptedAlgorithms` → 换 **ed25519** key 最干净。

> 提示:确认 `ssh kungal-neo` **免密直接进**之后,才做 §3 的 `PasswordAuthentication no` —— 否则公钥没配通又关了密码,就把自己锁门外了。

## 收尾检查清单

- [ ] `ssh root@ip` 被拒;`ssh kun@ip`(密钥)能进、`sudo` 可用
- [ ] 密码登录被拒(`PasswordAuthentication no`)
- [ ] `ufw status` 只放行 `<PORT>` / 80 / 443
- [ ] `fail2ban-client status sshd` 正常
- [ ] 时区/时间正确(`timedatectl`)
- [ ] 三个仓库已 clone 到 `~/app`
- [ ] 留好一个 VPS Web Console 兜底入口
