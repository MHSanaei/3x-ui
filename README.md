# 3X-UI

**An Advanced Web Panel • Built on Xray Core**

[![](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](#)
[![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **Disclaimer:** This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment

**If this project is helpful to you, you may wish to give it a**:star2:

<a href="#">
  <img width="125" alt="image" src="https://github.com/MHSanaei/3x-ui/assets/115543613/7aa895dd-048a-42e7-989b-afd41a74e2e1.jpg"></a>

- USDT (TRC20): `TXncxkvhkDWGts487Pjqq1qT9JmwRUz8CC`

## Install & Upgrade

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

## Install Custom Version

To install your desired version, add the version to the end of the installation command. e.g., ver `v2.1.3`:

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v2.1.3
```

## SSL Certificate

<details>
  <summary>Click for SSL Certificate</summary>

### Cloudflare

The Management script has a built-in SSL certificate application for Cloudflare. To use this script to apply for a certificate, you need the following:

- Cloudflare registered email
- Cloudflare Global API Key
- The domain name has been resolved to the current server through cloudflare

**1:** Run the`x-ui`command on the terminal, then choose `Cloudflare SSL Certificate`.


### Certbot
```
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

***Tip:*** *Certbot is also built into the Management script. You can run the `x-ui` command, then choose `SSL Certificate Management`.*

</details>

## Manual Install & Upgrade

<details>
  <summary>Click for manual install details</summary>

#### Usage

1. To download the latest version of the compressed package directly to your server, run the following command:

```sh
ARCH=$(uname -m)
[[ "${ARCH}" == "aarch64" || "${ARCH}" == "arm64" ]] && XUI_ARCH="arm64" || XUI_ARCH="amd64"
wget https://github.com/MHSanaei/3x-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
```

2. Once the compressed package is downloaded, execute the following commands to install or upgrade x-ui:

```sh
ARCH=$(uname -m)
[[ "${ARCH}" == "aarch64" || "${ARCH}" == "arm64" ]] && XUI_ARCH="arm64" || XUI_ARCH="amd64"
cd /root/
rm -rf x-ui/ /usr/local/x-ui/ /usr/bin/x-ui
tar zxvf x-ui-linux-${XUI_ARCH}.tar.gz
chmod +x x-ui/x-ui x-ui/bin/xray-linux-* x-ui/x-ui.sh
cp x-ui/x-ui.sh /usr/bin/x-ui
cp -f x-ui/x-ui.service /etc/systemd/system/
mv x-ui/ /usr/local/
systemctl daemon-reload
systemctl enable x-ui
systemctl restart x-ui
```

</details>

## Install with Docker

<details>
  <summary>Click for Docker details</summary>

#### Usage

1. Install Docker:

   ```sh
   bash <(curl -sSL https://get.docker.com)
   ```

2. Clone the Project Repository:

   ```sh
   git clone https://github.com/MHSanaei/3x-ui.git
   cd 3x-ui
   ```

3. Start the Service

   ```sh
   docker compose up -d
   ```

   OR

   ```sh
   docker run -itd \
      -e XRAY_VMESS_AEAD_FORCED=false \
      -v $PWD/db/:/etc/x-ui/ \
      -v $PWD/cert/:/root/cert/ \
      --network=host \
      --restart=unless-stopped \
      --name 3x-ui \
      ghcr.io/mhsanaei/3x-ui:latest
   ```

update to latest version

   ```sh
    cd 3x-ui
    docker compose down
    docker compose pull 3x-ui
    docker compose up -d
   ```

remove 3x-ui from docker 

   ```sh
    docker stop 3x-ui
    docker rm 3x-ui
    cd --
    rm -r 3x-ui
   ```

</details>


## Recommended OS

- Ubuntu 20.04+
- Debian 11+
- CentOS 8+
- Fedora 36+
- Arch Linux
- Manjaro
- Armbian
- AlmaLinux 9+
- Rockylinux 9+

## Compatible Architectures & Devices

Supports a variety of different architectures and devices. Here are some of the main architectures that we support:

- **amd64**: This is the most common architecture for personal computers and servers. It supports most modern operating systems. 

- **x86 / i386**: This architecture is prevalent in desktop and laptop computers. It's widely supported by various operating systems and applications. (Ex: Most Windows, macOS, and Linux systems)

- **armv8 / arm64 / aarch64**: This is the architecture for modern mobile and embedded devices, including smartphones and tablets. (Ex: Raspberry Pi 4, Raspberry Pi 3, Raspberry Pi Zero 2/Zero 2 W, Orange Pi 3 LTS,...)

- **armv7 / arm / arm32**: This is the architecture for older mobile and embedded devices. It is still widely used in many devices. (Ex: Orange Pi Zero LTS, Orange Pi PC Plus, Raspberry Pi 2,...)

- **armv6 / arm / arm32**: This is the architecture for very old embedded devices. While not as common as before, there are still some devices using this architecture. (Ex: Raspberry Pi 1, Raspberry Pi Zero/Zero W,...)

- **armv5 / arm / arm32**: This is an older architecture primarily used in early embedded systems. While it's less common today, some legacy devices may still rely on this architecture. (Ex: Early versions of Raspberry Pi, some older smartphones)
  
## Languages

- English
- Farsi
- Chinese
- Russian
- Vietnamese
- Spanish


## Features

- System Status Monitoring
- Search within all inbounds and clients
- Dark/Light theme
- Supports multi-user and multi-protocol
- Supports protocols, including VMess, VLESS, Trojan, Shadowsocks, Dokodemo-door, Socks, HTTP, wireguard
- Supports XTLS native Protocols, including RPRX-Direct, Vision, REALITY
- Traffic statistics, traffic limit, expiration time limit
- Customizable Xray configuration templates
- Supports HTTPS access panel (self-provided domain name + SSL certificate)
- Supports One-Click SSL certificate application and automatic renewal
- For more advanced configuration items, please refer to the panel
- Fixes API routes (user setting will be created with API)
- Supports changing configs by different items provided in the panel.
- Supports export/import database from the panel


## Default Settings

<details>
  <summary>Click for default settings details</summary>

  ### Information

- **Port:** 2053
- **Username & Password:** It will be generated randomly if you skip modifying.
- **Database Path:**
  - /etc/x-ui/x-ui.db
- **Xray Config Path:**
  - /usr/local/x-ui/bin/config.json
- **Web Panel Path w/o Deploying SSL:**
  - http://ip:2053/panel
  - http://domain:2053/panel
- **Web Panel Path w/ Deploying SSL:**
  - https://domain:2053/panel
 
</details>

## [WARP Configuration](https://gitlab.com/fscarmen/warp)

<details>
  <summary>Click for WARP configuration details</summary>

#### Usage

If you want to use routing to WARP before v2.1.0 follow steps as below:

**1.** Install WARP on **SOCKS Proxy Mode**:

   ```sh
   bash <(curl -sSL https://raw.githubusercontent.com/hamid-gh98/x-ui-scripts/main/install_warp_proxy.sh)
   ```

**2.** If you already installed warp, you can uninstall using below command:

   ```sh
   warp u
   ```

**3.** Turn on the config you need in panel

   Config Features:

   - Block Ads
   - Route Google + Netflix + Spotify + OpenAI (ChatGPT) to WARP
   - Fix Google 403 error

</details>

## IP Limit

<details>
  <summary>Click for IP limit details</summary>

#### Usage

**Note:** IP Limit won't work correctly when using IP Tunnel

- For versions up to `v1.6.1`:

  - IP limit is built-in into the panel.

- For versions `v1.7.0` and newer:

  - To make IP Limit work properly, you need to install fail2ban and its required files by following these steps:

    1. Use the `x-ui` command inside the shell.
    2. Select `IP Limit Management`.
    3. Choose the appropriate options based on your needs.
   
  - make sure you have ./access.log on your Xray Configuration after v2.1.3 we have an option for it
  
  ```sh
    "log": {
    "access": "./access.log",
    "dnsLog": false,
    "error": "./error.log",
    "loglevel": "warning"
    },
  ```

</details>

## Telegram Bot

<details>
  <summary>Click for Telegram bot details</summary>

#### Usage

The web panel supports daily traffic, panel login, database backup, system status, client info, and other notification and functions through the Telegram Bot. To use the bot, you need to set the bot-related parameters in the panel, including:

- Telegram Token
- Admin Chat ID(s)
- Notification Time (in cron syntax)
- Expiration Date Notification
- Traffic Cap Notification
- Database Backup
- CPU Load Notification


**Reference syntax:**

- `30 \* \* \* \* \*` - Notify at the 30s of each point
- `0 \*/10 \* \* \* \*` - Notify at the first second of each 10 minutes
- `@hourly` - Hourly notification
- `@daily` - Daily notification (00:00 in the morning)
- `@weekly` - weekly notification
- `@every 8h` - Notify every 8 hours

### Telegram Bot Features

- Report periodic
- Login notification
- CPU threshold notification
- Threshold for Expiration time and Traffic to report in advance
- Support client report menu if client's telegram username added to the user's configurations
- Support telegram traffic report searched with UUID (VMESS/VLESS) or Password (TROJAN) - anonymously
- Menu based bot
- Search client by email ( only admin )
- Check all inbounds
- Check server status
- Check depleted users
- Receive backup by request and in periodic reports
- Multi language bot

### Setting up Telegram bot

- Start [Botfather](https://t.me/BotFather) in your Telegram account:
    ![Botfather](./media/botfather.png)
  
- Create a new Bot using /newbot command: It will ask you 2 questions, A name and a username for your bot. Note that the username has to end with the word "bot".
    ![Create new bot](./media/newbot.png)

- Start the bot you've just created. You can find the link to your bot here.
    ![token](./media/token.png)

- Enter your panel and config Telegram bot settings like below:
![Panel Config](./media/panel-bot-config.png)

Enter your bot token in input field number 3.
Enter the user ID in input field number 4. The Telegram accounts with this id will be the bot admin. (You can enter more than one, Just separate them with ,)

- How to get Telegram user ID? Use this [bot](https://t.me/useridinfobot), Start the bot and it will give you the Telegram user ID.
![User ID](./media/user-id.png)

</details>

## API Routes

<details>
  <summary>Click for API routes details</summary>

#### Usage

- `/login` with `POST` user data: `{username: '', password: ''}` for login
- `/panel/api/inbounds` base for following actions:

| Method | Path                               | Action                                      |
| :----: | ---------------------------------- | ------------------------------------------- |
| `GET`  | `"/list"`                          | Get all inbounds                            |
| `GET`  | `"/get/:id"`                       | Get inbound with inbound.id                 |
| `GET`  | `"/getClientTraffics/:email"`      | Get Client Traffics with email              |
| `GET`  | `"/createbackup"`                  | Telegram bot sends backup to admins         |
| `POST` | `"/add"`                           | Add inbound                                 |
| `POST` | `"/del/:id"`                       | Delete Inbound                              |
| `POST` | `"/update/:id"`                    | Update Inbound                              |
| `POST` | `"/clientIps/:email"`              | Client Ip address                           |
| `POST` | `"/clearClientIps/:email"`         | Clear Client Ip address                     |
| `POST` | `"/addClient"`                     | Add Client to inbound                       |
| `POST` | `"/:id/delClient/:clientId"`       | Delete Client by clientId\*                 |
| `POST` | `"/updateClient/:clientId"`        | Update Client by clientId\*                 |
| `POST` | `"/:id/resetClientTraffic/:email"` | Reset Client's Traffic                      |
| `POST` | `"/resetAllTraffics"`              | Reset traffics of all inbounds              |
| `POST` | `"/resetAllClientTraffics/:id"`    | Reset traffics of all clients in an inbound |
| `POST` | `"/delDepletedClients/:id"`        | Delete inbound depleted clients (-1: all)   |
| `POST` | `"/onlines"`                       | Get Online users ( list of emails )       |

\*- The field `clientId` should be filled by:

- `client.id` for VMESS and VLESS
- `client.password` for TROJAN
- `client.email` for Shadowsocks


- [API Documentation](https://documenter.getpostman.com/view/16802678/2s9YkgD5jm)
- [<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 128px; height: 32px;">](https://app.getpostman.com/run-collection/16802678-1a4c9270-ac77-40ed-959a-7aa56dc4a415?action=collection%2Ffork&source=rip_markdown&collection-url=entityId%3D16802678-1a4c9270-ac77-40ed-959a-7aa56dc4a415%26entityType%3Dcollection%26workspaceId%3D2cd38c01-c851-4a15-a972-f181c23359d9)
</details>

## Environment Variables

<details>
  <summary>Click for environment variables details</summary>

#### Usage

| Variable       |                      Type                      | Default       |
| -------------- | :--------------------------------------------: | :------------ |
| XUI_LOG_LEVEL  | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | `"info"`      |
| XUI_DEBUG      |                   `boolean`                    | `false`       |
| XUI_BIN_FOLDER |                    `string`                    | `"bin"`       |
| XUI_DB_FOLDER  |                    `string`                    | `"/etc/x-ui"` |
| XUI_LOG_FOLDER |                    `string`                    | `"/var/log"`  |

Example:

```sh
XUI_BIN_FOLDER="bin" XUI_DB_FOLDER="/etc/x-ui" go build main.go
```

</details>

## Preview

![1](./media/1.png)
![2](./media/2.png)
![3](./media/3.png)
![4](./media/4.png)
![5](./media/5.png)
![6](./media/6.png)
![7](./media/7.png)

## A Special Thanks to

- [alireza0](https://github.com/alireza0/)

## Acknowledgment

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (License: **GPL-3.0**): _Enhanced v2ray/xray and v2ray/xray-clients routing rules with built-in Iranian domains and a focus on security and adblocking._
- [Vietnam Adblock rules](https://github.com/vuong2023/vn-v2ray-rules) (License: **GPL-3.0**): _A hosted domain hosted in Vietnam and blocklist with the most efficiency for Vietnamese._

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg)](https://starchart.cc/MHSanaei/3x-ui)
