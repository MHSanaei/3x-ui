# 3x-ui
[![](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](#)
[![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)


> **Disclaimer: This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment**

xray panel supporting multi-protocol, **Multi-lang (English,Farsi,Chinese)**

# Install & Upgrade

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```
## Install custom version
To install your desired version you can add the version to the end of install command. Example for ver `v1.0.9`:
```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v1.0.9
```
# SSL
```
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

**If you think this project is helpful to you, you may wish to give a** :star2: 

# Default settings

- Port: 2053
- username and password will be generated randomly if you skip to modify your own security(x-ui "7")
- database path: /etc/x-ui/x-ui.db
- xray config path: /usr/local/x-ui/bin/config.json

before you set ssl on settings
- http:// ip or domain:2053/xui

After you set ssl on settings 
- https://yourdomain:2053/xui

# Enable Traffic For Users:

**copy and paste to xray Configuration :** (you don't need to do this if you have a fresh install)
- [enable traffic](./media/enable-traffic.txt)
- [enable traffic+block all IR IP address](./media/enable-traffic+block-IR-IP.txt)
- [enable traffic+block all IR domain](./media/enable-traffic+block-IR-domain.txt)

# Features

- System Status Monitoring
- Search within all inbounds and clients
- Support Dark/Light theme UI
- Support multi-user multi-protocol, web page visualization operation
- Supported protocols: vmess, vless, trojan, shadowsocks, dokodemo-door, socks, http
- Support for configuring more transport configurations
- Traffic statistics, limit traffic, limit expiration time
- Customizable xray configuration templates
- Support https access panel (self-provided domain name + ssl certificate)
- Support one-click SSL certificate application and automatic renewal
- For more advanced configuration items, please refer to the panel
- fix api routes (user setting will create with api)

# Tg robot use

X-UI supports daily traffic notification, panel login reminder and other functions through the Tg robot. To use the Tg robot, you need to apply for the specific application tutorial. You can refer to the [blog](https://coderfan.net/how-to-use-telegram-bot-to-alarm-you-when-someone-login-into-your-vps.html)
Set the robot-related parameters in the panel background, including:

- Tg robot Token
- Tg robot ChatId
- Tg robot cycle runtime, in crontab syntax
- Tg robot Expiration threshold
- Tg robot Traffic threshold
- Tg robot Enable send backup in cycle runtime
- Tg robot Enable CPU usage alarm threshold

Reference syntax:

- 30 * * * * * //Notify at the 30s of each point
- 0 */10 * * * * //Notify at the first second of each 10 minutes
- @hourly // hourly notification
- @daily // Daily notification (00:00 in the morning)
- @every 8h // notify every 8 hours

# Telegram Bot Features

- Report periodic
- Login notification
- CPU threshold notification
- Threshold for Expiration time and Traffic to report in advance
- Support client report menu if client's telegram username added to the user's configurations
- Support telegram traffic report searched with UID (VMESS/VLESS) or Password (TROJAN) - anonymously
- Menu based bot
- Search client by email ( only admin )
- Check all inbounds
- Check server status
- Check depleted users
- Receive backup by request and in periodic reports


## API routes

- `/login` with `PUSH` user data: `{username: '', password: ''}` for login
- `/xui/API/inbounds` base for following actions:

| Method | Path | Action |
| ------------- | ------------- | ------------- |
| GET | "/list" | Get all inbounds |
| GET | "/get/:id" | Get inbound with inbound.id |
| POST | "/add" | Add inbound |
| POST | "/del/:id" | Delete Inbound |
| POST | "/update/:id" | Update Inbound |
| POST | "/clientIps/:email" | Client Ip address |
| POST | "/clearClientIps/:email" | Clear Client Ip address |
| POST | "/addClient/" | Add Client to inbound |
| POST | "/delClient/:email" | Delete Client |
| POST | "/updateClient/:index" | Update Client |
| POST | "/:id/resetClientTraffic/:email" | Reset Client's Traffic |
| POST | "/resetAllTraffics" | Reset traffics of all inbounds |
| POST | "/resetAllClientTraffics/:id" | Reset traffics of all clients in an inbound |

# A Special Thanks To
- [alireza0](https://github.com/alireza0/)
- [FranzKafkaYu](https://github.com/FranzKafkaYu)

# Suggestion System
- Ubuntu 20.04+
- Debian 10+
- CentOS 8+
- Fedora 36+

# Pictures

![1](./media/1.png)
![2](./media/2.png)
![3](./media/3.png)
![4](./media/4.png)

## Stargazers over time

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg)](https://starchart.cc/MHSanaei/3x-ui)
