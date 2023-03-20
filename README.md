# 3x-ui
![](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)
![](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)
![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)
![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)
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

before you set ssl on settings
- http:// ip or domain:2053/xui

After you set ssl on settings 
- https://yourdomain:2053/xui

# Enable Traffic For Users:

**copy and paste to xray Configuration :** (you don't need to do this if you have a fresh install)
- [for enable traffic](https://raw.githubusercontent.com/mhsanaei/3x-ui/main/media/for%20enable%20traffic.txt)
- [for enable traffic+block all iran ip address](https://raw.githubusercontent.com/mhsanaei/3x-ui/main/media/for%20enable%20traffic%2Bblock%20all%20iran%20ip.txt)

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

- @hourly // hourly notification
- @daily // Daily notification (00:00 in the morning)
- @every 8h // notify every 8 hours

# Telegram Bot Features

- Report periodic
- Login notification
- CPU threshold notification
- Threshold for Expiration time and Traffic to report in advance
- Support client report if client's telegram username is added to the end of `email` like 'test123@telegram_username'
- Support telegram traffic report searched with UID (VMESS/VLESS) or Password (TROJAN) - anonymously
- Menu based bot
- Search client by email ( only admin )
- Check all inbounds
- Check server status
- Check Exhausted users
- Receive backup by request and in periodic reports

# A Special Thanks To
- [alireza0](https://github.com/alireza0/)
- [HexaSoftwareTech](https://github.com/HexaSoftwareTech/)

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
