# x-ui

> **Disclaimer: This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment**

# Install & Upgrade

```
bash <(curl -Ls https://raw.githubusercontent.com/quydang04/x-ui/master/install.sh)
```

# SSL

```
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

You also can use `x-ui` menu then select `SSL Certificate Management`

# Manual Install & Upgrade

<details>
  <summary>Click for Manual Install details</summary>

1. To download the latest version of the compressed package directly to your server, run the following command:

```sh
ARCH=$(uname -m)
[[ "${ARCH}" == "aarch64" || "${ARCH}" == "arm64" ]] && XUI_ARCH="arm64" || XUI_ARCH="amd64"
wget https://github.com/quydang04/x-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
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

# Clone the Project Repository:

   ```sh
   git clone https://github.com/quydang04/x-ui.git
   cd x-ui
   ```

</details>

# Default settings

<details>
  <summary>Click for Default settings details</summary>

- Port: 2053
- username and password will be generated randomly if you skip to modify your own security(x-ui "7")
- database path: /etc/x-ui/x-ui.db
- xray config path: /usr/local/x-ui/bin/config.json

Before you set ssl on settings

- http://ip:2053/panel
- http://domain:2053/panel

After you set ssl on settings

- https://yourdomain:2053/panel
</details>

# Xray Configurations:

<details>
  <summary>Click for Xray Configurations details</summary>

**copy and paste to xray Configuration :** (you don't need to do this if you have a fresh install)

- [traffic](./media/configs/traffic.json)
- [traffic + Block all Iran IP address](./media/configs/traffic+block-iran-ip.json)
- [traffic + Block all Iran Domains](./media/configs/traffic+block-iran-domains.json)
- [traffic + Block Ads + Use IPv4 for Google](./media/configs/traffic+block-ads+ipv4-google.json)
- [traffic + Block Ads + Route Google + Netflix + Spotify + OpenAI (ChatGPT) to WARP](./media/configs/traffic+block-ads+warp.json)

</details>

# [WARP Configuration](https://github.com/fscarmen/warp) (Optional)

<details>
  <summary>Click for WARP Configuration details</summary>

If you want to use routing to WARP follow steps as below:

1. If you already installed warp, you can uninstall using below command:

   ```sh
   warp u
   ```

2. Install WARP on **socks proxy mode**:

   ```sh
   bash <(curl -sSL https://raw.githubusercontent.com/hamid-gh98/x-ui-scripts/main/install_warp_proxy.sh)
   ```

3. Turn on the config you need in panel or [Copy and paste this file to Xray Configuration](./media/configs/traffic+block-ads+warp.json)

   Config Features:

   - Block Ads
   - Route Google + Netflix + Spotify + OpenAI (ChatGPT) to WARP
   - Fix Google 403 error

</details>

# IP Limit

<details>
  <summary>Click for IP Limit details</summary>

**Note: IP Limit won't work correctly when using IP Tunnel**

- For versions up to `v1.6.1`:

  - IP limit is built-in into the panel.

- For versions `v1.7.0` and newer:

  - To make IP Limit work properly, you need to install fail2ban and its required files by following these steps:

    1. Use the `x-ui` command inside the shell.
    2. Select `IP Limit Management`.
    3. Choose the appropriate options based on your needs.

</details>

# Telegram Bot

<details>
  <summary>Click for Telegram Bot details</summary>

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

- 30 \* \* \* \* \* //Notify at the 30s of each point
- 0 \*/10 \* \* \* \* //Notify at the first second of each 10 minutes
- @hourly // hourly notification
- @daily // Daily notification (00:00 in the morning)
- @weekly // weekly notification
- @every 8h // notify every 8 hours

# Telegram Bot Features

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
</details>

# A Special Thanks To

- [alireza0](https://github.com/alireza0/)
- [MHSanaei](https://github.com/MHSanaei)
- [Hossin Asaadi](https://github.com/hossinasaadi)
- [NidukaAkalanka](https://github.com/NidukaAkalanka)
- [vaxilu](https://github.com/vaxilu)
  
# Suggestion System

- Ubuntu 20.04+
- Debian 10+
- CentOS 7+
- Fedora 30+
- Arch Linux
- Armbian
- Windows 7 and newer(coming soon...)
