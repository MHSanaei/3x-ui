[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/Kuzz007/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/Kuzz007/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/Kuzz007/3x-ui.svg" alt="GO Version"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
</p>

**这是 [3X-UI](https://github.com/MHSanaei/3x-ui) 的个人分支（fork）**——一个先进的开源 Web 控制面板，用于管理 [Xray-core](https://github.com/XTLS/Xray-core)——增加了一项主要功能：**原生 AmneziaWG 支持**，作为与 VLESS、VMess、Trojan 等并列的一等协议。3X-UI 原本具备的一切（多协议入站、按客户端流量统计、订阅、多节点、Telegram 机器人）均保持不变，运行方式与原项目完全一致。

这个分支是为作者自己的路由器和个人服务器构建的；它并不打算替代或与原项目竞争。如果您需要一个通用面板，请前往 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui)——以下内容仅记录本分支的差异之处。

> [!IMPORTANT]
> 本项目仅供个人使用。请勿将其用于非法目的，也请勿在生产环境中使用。

## 本分支的不同之处：AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) 是 WireGuard 的一个变体，增加了一层混淆（垃圾数据包、随机填充、重写魔术头部），旨在击败基于 DPI 的协议指纹识别——同样的隧道，但在线路上不再表现得像一条隧道。

- **原生实现，而非 Docker。** AmneziaWG 作为真正的内核接口运行在宿主机上，通过 `awg-quick`/`awg` 启动和停止——采用与拥有原生 `wg0` 接口相同的 DKMS 内核模块方案。无需特权 sidecar 容器。
- **一等协议。** AmneziaWG 入站与其他协议共享同一张 `Inbound` 表，因此可以免费获得批量操作、二维码/配置下载弹窗以及订阅链接——无需学习任何新东西。
- **完整的 AmneziaWG 2.0 混淆功能**——Jc/Jmin/Jmax（垃圾数据包）、S1–S4（数据包填充）、H1–H4（魔术头部）以及 I1 签名数据包，均可按入站单独编辑，并提供一键随机化按钮，同时为旧版客户端提供 1.x 兼容模式。
- **原生 IPv6**，支持按客户端的 NDP 代理，使每个对等端都获得可直接访问的 IPv6 地址——无需 NAT66。
- **按客户端端口转发**——将特定端口/端口范围直接 DNAT 到某个对等端的隧道地址。
- **将客户端流量通过 Xray 路由**——每个 AmneziaWG 入站都会自动获得属于自己的本地回环 Xray 网桥（无需任何开关）；通过面板中已有的"路由"页面，将任意客户端的流量路由到任意已配置的 Xray 出站，方式与路由其他协议完全相同。
- **`install.sh` 会为您安装内核模块**，适用于 Ubuntu/Debian/Armbian（`ppa:amnezia/ppa`），其他发行版则有回退方案。它唯一无法为您做的事：预先**禁用 VPS/VM 上的 Secure Boot**——DKMS 构建的模块未经签名，只要 Secure Boot 处于启用状态，内核就会拒绝加载它。
- 协调（reconcile）方式与 [`internal/mtproto`](internal/mtproto) 管理 `mtg` sidecar 的方式完全相同：一个后台任务持续保持运行中的接口与数据库中存储的内容同步，并尽可能通过 `awg syncconf` 而非完整的接口重启来应用对等端变更。

## 功能特性

- **多协议入站** — VLESS、VMess、Trojan、Shadowsocks、WireGuard、**AmneziaWG**、Hysteria2、HTTP、SOCKS (Mixed)、Dokodemo-door / Tunnel 和 TUN。
- **现代传输与安全** — TCP (Raw)、mKCP、WebSocket、gRPC、HTTPUpgrade 和 XHTTP，并通过 TLS、XTLS 和 REALITY 加密。
- **回落 (Fallback)** — 通过 Xray 的 fallback 功能在单个端口上提供多种协议（例如在 443 端口上同时使用 VLESS 和 Trojan）。
- **按客户端管理** — 流量配额、到期日期、IP 限制、实时在线状态，以及一键分享链接、二维码和订阅。
- **流量统计** — 按入站、按客户端、按出站统计，并支持重置控制。
- **多节点支持** — 从单一面板管理并扩展到多台服务器。
- **出站与路由** — WARP、NordVPN、自定义路由规则、负载均衡器和出站代理链。
- **内置订阅服务器**，支持多种输出格式和[自定义页面模板](docs/custom-subscription-templates.md)。
- **Telegram 机器人**，用于远程监控和管理。
- **RESTful API**，带有面板内置的 Swagger 文档。
- **灵活的存储** — SQLite（默认）或 PostgreSQL。
- **13 种界面语言**，支持深色和浅色主题。
- **Fail2ban 集成**，用于强制执行按客户端的 IP 限制。

## 截图

<details>
<summary>点击展开</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Overview" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Inbounds" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Add client" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Configs" src="./media/05-add-nodes-light.png">
</picture>

</details>

## 快速开始

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

本分支仅发布滚动更新的预发布版本 **`dev-latest`**（每次 push 到 `main` 都会自动重新构建）——目前还没有任何打标签的稳定版本，因此 `dev` 是当前唯一能解析到实际内容的渠道。

安装过程中会生成随机的用户名、密码和访问路径。安装完成后，运行 `x-ui` 打开管理菜单，您可以在其中启动/停止服务、查看或重置登录凭据、管理 SSL 证书等。

有关本 README 未涵盖的完整面板文档，请参阅[原项目 Wiki](https://github.com/MHSanaei/3x-ui/wiki)——其中没有任何内容专属于本分支，因此完全适用。

### 无人值守安装

安装程序也可以**非交互式**运行，适用于 cloud-init。
设置 `XUI_NONINTERACTIVE=1`（或在无 TTY 的情况下通过管道传入），它就会全程
零提示地完成端到端安装，生成随机凭据并写入
`/etc/x-ui/install-result.env`。请参阅 [`deploy/`](deploy/)：

- [Cloud-init user-data](deploy/cloud-init/) — 在任意云平台上无人值守安装（Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle）
- [Hetzner Cloud 说明](deploy/marketplace/hetzner/) — 在 Hetzner 上基于 cloud-init 的部署

## 支持的平台

**操作系统：** Ubuntu、Debian、Armbian、Fedora、CentOS、RHEL、AlmaLinux、Rocky Linux、Oracle Linux、Amazon Linux、Virtuozzo、Arch、Manjaro、Parch、openSUSE (Tumbleweed / Leap) 和 Alpine。（原项目也发布 Windows 版本；本分支的 CI 不这样做——这里的一切都面向运行 Linux 的服务器/路由器，而且 AmneziaWG 无论如何都需要 Linux 内核模块。）

**架构：** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`。

AmneziaWG 特别需要真正的 Linux 内核以及 AmneziaWG 专用的 DKMS 内核模块——它无法在 Windows 上运行，而目前 `install_amneziawg` 只能在 Ubuntu/Debian/Armbian 上自动完成内核模块安装（参见[本分支的不同之处](#本分支的不同之处amneziawg)一节）。

## 数据库选项

3X-UI 支持两种后端，可在安装时选择：

- **SQLite**（默认）— 位于 `/etc/x-ui/x-ui.db` 的单个文件。无需配置，适合中小型部署。
- **PostgreSQL** — 推荐用于大量客户端或多节点设置。安装程序可以为您在本地安装 PostgreSQL，或接受指向现有服务器的 DSN。

运行时通过环境变量选择后端（安装程序会为您写入 `/etc/default/x-ui`）：

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### 将现有的 SQLite 安装迁移到 PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# 然后在 /etc/default/x-ui 中设置 XUI_DB_TYPE 和 XUI_DB_DSN 并重启：
systemctl restart x-ui
```

源 SQLite 文件保持不变；在确认新后端正常工作后，请手动删除它。

### Docker

默认的 `docker compose up -d` 仍使用 SQLite。若要使用捆绑的 PostgreSQL 服务运行，请取消注释 `docker-compose.yml` 中的两行 `XUI_DB_*` 环境变量，并使用该 profile 启动：

```bash
docker compose --profile postgres up -d
```

> [!NOTE]
> AmneziaWG 入站需要**宿主机**上的 `awg-quick`/`awg` 和 AmneziaWG 内核模块——这正是[本分支的不同之处](#本分支的不同之处amneziawg)一节中所述的"不使用 Docker"设计初衷所在。将面板本身运行在 Docker 中对其他任何协议仍然有效，但由运行在容器中的面板创建的 AmneziaWG 入站将无法启动其接口，除非该容器获得宿主机级别的网络/内核访问权限，而这会违背整体设计初衷。如果您打算使用 AmneziaWG，请在宿主机上原生运行面板。

该镜像捆绑了 Fail2ban（默认启用），用于强制执行按客户端的 **IP 限制**。Fail2ban 使用 `iptables` 封禁违规者，这需要 `NET_ADMIN` 权限。`docker-compose.yml` 已通过 `cap_add` 授予该权限；如果您改用 `docker run` 启动容器，请自行添加这些权限，否则封禁只会被记录而永远不会生效：

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `XUI_DB_TYPE` | 数据库后端：`sqlite` 或 `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL 连接字符串（当 `XUI_DB_TYPE=postgres` 时） | — |
| `XUI_DB_FOLDER` | SQLite 数据库文件所在目录 | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | 最大打开连接数（PostgreSQL 连接池） | — |
| `XUI_DB_MAX_IDLE_CONNS` | 最大空闲连接数（PostgreSQL 连接池） | — |
| `XUI_INIT_WEB_BASE_PATH` | Web 面板的初始 URI 路径 | `/` |
| `XUI_ENABLE_FAIL2BAN` | 启用基于 Fail2ban 的 IP 限制 | `true` |
| `XUI_LOG_LEVEL` | 日志级别（`debug`、`info`、`warning`、`error`） | `info` |
| `XUI_DEBUG` | 启用调试模式 | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | 启用隧道健康监控（探测某个 URL，在连续多次失败后重启 xray；重启会断开所有客户端） | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | 探测请求所经过的代理；将其指向本地 xray 入站，使探测能够测试隧道（例如 `socks5://127.0.0.1:1080`）。留空表示探测仅检查主机连通性 | — |
| `XUI_TUNNEL_HEALTH_URL` | 用于检测隧道健康状况的探测 URL | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | 两次探测之间的间隔 | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | 单次探测的超时时间 | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | 触发重启前的连续失败次数 | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | 两次连续重启之间的最小间隔 | `5m` |

## 支持的语言

面板界面提供 13 种语言：

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## 开发者说明

这是一个个人分支，不寻求外部贡献者，但如果您自己也在这份代码基础上进行开发，[CONTRIBUTING.md](/CONTRIBUTING.md) 仍然包含了搭建本地开发环境（Go/Node 版本、CGo 所需的 C 编译器、build/lint/test 命令）的详细且有用的说明。

## 致谢

本分支完全构建于 [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) 之上——整个面板、多协议支持以及底层架构都是他们的成果；**AmneziaWG 支持是这里唯一新增的内容。** 如果您觉得原项目有用，原作者的赞助链接依然是表达支持的正确去处：

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

本分支中原生 AmneziaWG 的实现参考/借鉴自：

- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — 针对原项目提出的原始 AmneziaWG PR（Docker sidecar 方案）；本分支重用了其 schema/前端结构，但将后端替换为原生、无 Docker 的管理器。
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — 一个独立的分支，已经在生产环境中运行原生 AmneziaWG；本分支中 `awg-quick` 进程管理、配置生成以及 AmneziaWG 2.0 混淆参数生成器均源自其 `awg/` 包。

## 特别感谢

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (许可证: **GPL-3.0**): _增强的 v2ray/xray 和 v2ray/xray-clients 路由规则，内置伊朗域名，专注于安全性和广告拦截。_
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (许可证: **GPL-3.0**): _此仓库包含基于俄罗斯被阻止域名和地址数据自动更新的 V2Ray 路由规则。_

## 社区工具

社区围绕 3x-ui 构建的工具和集成。

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (许可证: **MIT**): _使用 Terraform / OpenTofu 通过代码管理入站、客户端、面板设置和 Xray 配置。_
