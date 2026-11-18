# 3x-ui 离线部署与更新

`offline-localization` 面向内网部署。浏览器从面板本身加载界面，面板停用公网 IP 查询、赞助内容、版本检查、远程订阅刷新和自动下载规则数据。Telegram、Discord 和公共 VPN 注册服务在离线版停用。节点批量在线更新入口已关闭。

## Linux systemd 部署要求

使用支持 cgroup IP 过滤的 Linux 主机，systemd 版本至少为 240。安装器会给 `x-ui.service` 写入一条拒绝公网 IP 的规则，允许回环、RFC 1918 私网和 IPv6 ULA。此规则也适用于 Xray 子进程，公网入站和公网代理流量因此不可用。通过内网地址登录管理页面。严格隔离场景下，目标网段需在防火墙上禁止公网出站，并使用内网 DNS。请勿给服务配置能访问公网的内网代理或转发器。节点、SMTP 和 LDAP 使用内网 IP 地址或 `localhost`。

上传更新目前支持 systemd 和 SQLite，适用标准 unit `x-ui.service`。新装使用 `/usr/local/x-ui`、`/etc/x-ui` 及 `/var/log/x-ui`。建议以目标主机兼容的 Linux 和 CPU 架构构建二进制。

## 制作替换包

在构建机准备 Go（对应 `go.mod`）、Node（对应 `frontend/package.json`）、npm 及 Python 3.11 或更新版本。构建机安装依赖可用已有缓存或内部镜像。部署机只接收构建产物和本地文件。

```bash
(cd frontend && npm ci --no-audit && npm run build)
mkdir -p release
go build -trimpath -o release/x-ui .
python3 scripts/pack-offline.py release/3x-ui-offline.zip \
  --panel release/x-ui \
  --xray /path/to/local/xray \
  --geo /path/to/local/geoip.dat \
  --geo /path/to/local/geosite.dat
./release/x-ui offline-verify release/3x-ui-offline.zip
```

首次安装的包需含面板和 Xray。替换包可以只含需要更新的组件。GeoData 支持 `geoip.dat`、`geosite.dat` 以及对应的 `_IR`、`_RU` 文件。上传上限为 256 MiB，解压上限为 512 MiB。打包脚本输出 SHA-256，便于通过可信渠道交叉验证。面板检查 ZIP 完整性、文件白名单和二进制架构。只上传自行构建或通过可信渠道取得的包。包内的可执行文件会以服务账户权限运行。

## 首次启用离线版

新装时把源码中的安装脚本和对应 systemd unit 模板，以及构建好的面板和 ZIP 一起带入内网。

```bash
sudo bash ./install.sh release/3x-ui-offline.zip release/x-ui
sudo journalctl -u x-ui.service -n 100
```

已有上游 systemd 安装时，用构建好的离线二进制引导一次替换。先确认当前 `x-ui.service` 的 `ExecStart`、工作目录和数据库路径。下面使用默认路径。

```bash
sudo ./release/x-ui offline-update release/3x-ui-offline.zip /usr/local/x-ui/x-ui
sudo install -m 0755 ./x-ui.sh /usr/bin/x-ui
```

如果原系统曾开启 ACME 自动续签，迁移时在主机上停用为该面板配置的续签任务，之后通过面板设置现有本地证书文件路径。旧版的主机级计划任务由系统自身管理。

## 日常更新

1. 使用管理员账户打开面板概览，点击面板版本，选择「离线更新」。
2. 选取 ZIP 并点击「上传并应用」。面板会短暂停止，然后报告本次任务结果。
3. 更新会保留账户、入站、SQLite 配置和证书路径。启动失败时恢复原二进制、相关规则文件和 SQLite 状态。
4. 主机上的本地文件也可使用 `sudo x-ui update /path/to/update.zip`。Web 更新日志使用 `journalctl -u x-ui-offline-update.service` 查看。

通过 API 上传时，请求体是 ZIP 原始字节，类型为 `application/zip`。使用现有管理员认证调用 `POST /panel/api/server/uploadUpdate`，用返回的 `runId` 查询 `GET /panel/api/server/getUpdateStatus`。控制台上传支持现有面板路径前缀。反向代理须允许对应的请求体大小。

## 容器构建

容器没有面板内替换权限。内网运行容器时，从本地归档加载新镜像并重建容器，保留 `db` 和 `cert` 卷。Compose 默认只使用已经加载的 `3x-ui-offline:local`，绑定主机回环地址，使用内部网络。

准备 `release/docker/x-ui`、`release/docker/bin/xray-linux-<arch>` 及放入 `bin/` 的规则文件。先加载构建所需的本地基础镜像，再使用 `docker build --network none --pull=false -t 3x-ui-offline:local .` 构建并将镜像存为归档。在部署端执行 `docker load -i <本地镜像归档>` 和 `docker compose up -d --pull never`。
