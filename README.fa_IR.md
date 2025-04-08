[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

**یک پنل وب پیشرفته • ساخته شده بر پایه Xray Core**

[![](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](#)
[![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](#)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

> **سلب مسئولیت:** این پروژه صرفاً برای اهداف آموزشی و تحقیقاتی است. استفاده از آن برای مقاصد غیرقانونی یا در محیط‌های عملیاتی ممنوع است.

**اگر این پروژه برای شما مفید بوده، می‌توانید با دادن یک**:star2: از آن حمایت کنید.

<p align="left">
  <a href="https://buymeacoffee.com/mhsanaei" target="_blank">
    <img src="./media/buymeacoffe.png" alt="Image">
  </a>
</p>

- USDT (TRC20): `TXncxkvhkDWGts487Pjqq1qT9JmwRUz8CC`
- MATIC (polygon): `0x41C9548675D044c6Bfb425786C765bc37427256A`
- LTC (Litecoin): `ltc1q2ach7x6d2zq0n4l0t4zl7d7xe2s6fs7a3vspwv`

## نصب و ارتقا

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

## نصب نسخه‌های قدیمی (توصیه نمی‌شود)

برای نصب نسخه خاصی از دستور زیر استفاده کنید. مثال برای نسخه `v1.7.9`:

```
VERSION=v1.7.9 && bash <(curl -Ls "https://raw.githubusercontent.com/mhsanaei/3x-ui/$VERSION/install.sh") $VERSION
```

## گواهی SSL

<details>
  <summary>جزئیات گواهی SSL</summary>

### ACME

برای مدیریت گواهی‌های SSL با استفاده از ACME:

1. اطمینان حاصل کنید دامنه شما به درستی به سرور متصل است.
2. دستور `x-ui` را در ترمینال اجرا کرده و گزینه `مدیریت گواهی SSL` را انتخاب کنید.
3. گزینه‌های زیر نمایش داده می‌شوند:

   - **دریافت SSL:** دریافت گواهی SSL
   - **لغو:** لغو گواهی‌های موجود
   - **تمدید اجباری:** تمدید اجباری گواهی‌ها
   - **نمایش دامنه‌های موجود:** نمایش تمام دامنه‌های دارای گواهی  
   - **تنظیم مسیر گواهی برای پنل:** تنظیم مسیر گواهی برای دامنه شما

### Certbot

نصب و استفاده از Certbot:

```sh
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

### Cloudflare

اسکریپت داخلی برای دریافت گواهی SSL از Cloudflare. نیازمند:

- ایمیل ثبت‌شده در Cloudflare
- کلید API جهانی Cloudflare
- دامنه باید از طریق Cloudflare به سرور متصل باشد

**دریافت کلید API جهانی Cloudflare:**

1. دستور `x-ui` را اجرا و گزینه `گواهی SSL کلادفلر` را انتخاب کنید.
2. به لینک [Cloudflare API Tokens](https://dash.cloudflare.com/profile/api-tokens) مراجعه کنید.
3. روی "View Global API Key" کلیک کنید:
   ![](media/APIKey1.PNG)
4. پس از احراز هویت، کلید API نمایش داده می‌شود:
   ![](media/APIKey2.png)

در هنگام استفاده، نام دامنه، ایمیل و کلید API را وارد کنید:
   ![](media/DetailEnter.png)

</details>

## نصب دستی و ارتقا

<details>
  <summary>جزئیات نصب دستی</summary>

#### استفاده

1. دریافت آخرین نسخه از سرور:

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;;
  i*86 | x86) XUI_ARCH="386" ;;
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;;
  armv7* | armv7) XUI_ARCH="armv7" ;;
  armv6* | armv6) XUI_ARCH="armv6" ;;
  armv5* | armv5) XUI_ARCH="armv5" ;;
  s390x) echo 's390x' ;;
  *) XUI_ARCH="amd64" ;;
esac

wget https://github.com/MHSanaei/3x-ui/releases/latest/download/x-ui-linux-${XUI_ARCH}.tar.gz
```

2. نصب یا ارتقا:

```sh
ARCH=$(uname -m)
case "${ARCH}" in
  x86_64 | x64 | amd64) XUI_ARCH="amd64" ;;
  i*86 | x86) XUI_ARCH="386" ;;
  armv8* | armv8 | arm64 | aarch64) XUI_ARCH="arm64" ;;
  armv7* | armv7) XUI_ARCH="armv7" ;;
  armv6* | armv6) XUI_ARCH="armv6" ;;
  armv5* | armv5) XUI_ARCH="armv5" ;;
  s390x) echo 's390x' ;;
  *) XUI_ARCH="amd64" ;;
esac

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

## نصب با Docker

<details>
  <summary>جزئیات Docker</summary>

#### استفاده

1. **نصب Docker:**

   ```sh
   bash <(curl -sSL https://get.docker.com)
   ```

2. **کلون پروژه:**

   ```sh
   git clone https://github.com/MHSanaei/3x-ui.git
   cd 3x-ui
   ```

3. **راه‌اندازی سرویس:**

   ```sh
   docker compose up -d
   ```

   یا

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

4. **به‌روزرسانی:**

   ```sh
   cd 3x-ui
   docker compose down
   docker compose pull 3x-ui
   docker compose up -d
   ```

5. **حذف:**

   ```sh
   docker stop 3x-ui
   docker rm 3x-ui
   cd --
   rm -r 3x-ui
   ```

</details>

## تنظیمات Nginx
<details>
  <summary>پیکربندی Reverse Proxy</summary>

#### Nginx Reverse Proxy
```nginx
location / {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Range $http_range;
    proxy_set_header If-Range $http_if_range; 
    proxy_redirect off;
    proxy_pass http://127.0.0.1:2053;
}
```

#### مسیر فرعی در Nginx
- اطمینان حاصل کنید "URI Path" در تنظیمات پنل یکسان باشد.
- `url` در تنظیمات پنل باید با `/` پایان یابد.   

```nginx
location /sub {
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header Range $http_range;
    proxy_set_header If-Range $http_if_range; 
    proxy_redirect off;
    proxy_pass http://127.0.0.1:2053;
}
```
</details>

## سیستم‌عامل‌های توصیه شده

- Ubuntu 22.04+
- Debian 12+
- CentOS 8+
- OpenEuler 22.03+
- Fedora 36+
- Arch Linux
- Parch Linux
- Manjaro
- Armbian
- AlmaLinux 9.5+
- Rocky Linux 9.5+
- Oracle Linux 8+
- OpenSUSE Tubleweed
- Amazon Linux 2023
- Virtuozzo Linux 8+
- Windows x64

## معماری‌ها و دستگاه‌های پشتیبانی شده

<details>
  <summary>جزئیات معماری‌ها و دستگاه‌ها</summary>

- **amd64**: معماری استاندارد برای کامپیوترهای شخصی و سرورها
- **x86 / i386**: سیستم‌های دسکتاپ و لپ‌تاپ
- **armv8 / arm64 / aarch64**: دستگاه‌های موبایل و embedded مانند Raspberry Pi 4
- **armv7 / arm / arm32**: دستگاه‌های قدیمی مانند Orange Pi Zero
- **armv6 / arm / arm32**: دستگاه‌های بسیار قدیمی مانند Raspberry Pi 1
- **armv5 / arm / arm32**: سیستم‌های embedded قدیمی
- **s390x**: کامپیوترهای IBM mainframe
</details>

## زبان‌های پشتیبانی شده

- انگلیسی
- فارسی
- چینی سنتی
- چینی ساده‌شده
- ژاپنی
- روسی
- ویتنامی
- اسپانیایی
- اندونزیایی
- اوکراینی
- ترکی
- پرتغالی (برزیل)

## ویژگی‌ها

- مانیتورینگ وضعیت سیستم
- جستجو در بین inboundها و کلاینت‌ها
- تم تاریک/روشن
- پشتیبانی از چند کاربر و پروتکل
- پروتکل‌های VMESS، VLESS، Trojan، Shadowsocks، Dokodemo-door، Socks، HTTP، WireGuard
- پشتیبانی از XTLS شامل RPRX-Direct، Vision، REALITY
- آمار ترافیک، محدودیت ترافیک، محدودیت زمانی
- تنظیمات سفارشی Xray
- پشتیبانی از HTTPS برای پنل
- دریافت خودکار گواهی SSL
- مسیرهای API اصلاح شده
- پشتیبانی از تغییر تنظیمات از طریق پنل
- امکان export/import دیتابیس

## تنظیمات پیش‌فرض پنل

<details>
  <summary>جزئیات تنظیمات پیش‌فرض</summary>

### نام کاربری، رمز عبور، پورت و مسیر وب

در صورت عدم تغییر، این موارد به صورت تصادفی ایجاد می‌شوند (به جز Docker).

**تنظیمات پیش‌فرض Docker:**
- **نام کاربری:** admin
- **رمز عبور:** admin
- **پورت:** 2053

### مدیریت دیتابیس:

  امکان Backup و Restore دیتابیس از طریق پنل.

- **مسیر دیتابیس:**
  - `/etc/x-ui/x-ui.db`

### مسیر پایه وب

1. **بازنشانی مسیر:**
   - اجرای دستور `x-ui`
   - انتخاب گزینه `Reset Web Base Path`

2. **ساخت یا تنظیم مسیر:**
   - مسیر به صورت تصادفی ساخته شده یا قابل تنظیم است

3. **مشاهده تنظیمات فعلی:**
   - استفاده از دستور `x-ui settings` یا `View Current Settings` در `x-ui`

**توصیه امنیتی:**
- استفاده از مسیرهای طولانی و تصادفی برای افزایش امنیت

**مثال:**
- `http://ip:port/*webbasepath*/panel`
- `http://domain:port/*webbasepath*/panel`

</details>

## پیکربندی WARP

<details>
  <summary>جزئیات WARP</summary>

#### استفاده

**برای نسخه‌های `v2.1.0` و جدیدتر:**

WARP به صورت داخلی پشتیبانی می‌شود. تنها نیاز به فعال‌سازی در پنل است.

</details>

## محدودیت IP

<details>
  <summary>جزئیات محدودیت IP</summary>

#### استفاده

**توجه:** محدودیت IP در صورت استفاده از IP Tunnel کار نمی‌کند.

- **تا نسخه `v1.6.1`:**
  - محدودیت IP به صورت داخلی در پنل وجود دارد

**برای نسخه‌های `v1.7.0` و جدیدتر:**

برای فعال‌سازی نیاز به نصب `fail2ban` است:

1. اجرای دستور `x-ui` و انتخاب `مدیریت محدودیت IP`
2. گزینه‌های موجود:

   - **تغییر مدت زمان Ban**
   - **حذف تمام Banها**
   - **مشاهده لاگ‌ها**
   - **وضعیت Fail2ban**
   - **راه‌اندازی مجدد Fail2ban**
   - **حذف Fail2ban**

3. تنظیم مسیر `Access log` در پنل به `./access.log` و ذخیره و راه‌اندازی مجدد Xray

- **قبل از نسخه `v2.1.3`:**
  - تنظیم دستی `access.log` در تنظیمات Xray:

    ```sh
    "log": {
      "access": "./access.log",
      "dnsLog": false,
      "loglevel": "warning"
    },
    ```

- **از نسخه `v2.1.3`:**
  - امکان تنظیم `access.log` از طریق پنل

</details>

## ربات تلگرام

<details>
  <summary>جزئیات ربات تلگرام</summary>

#### استفاده

ربات تلگرام برای اطلاع‌رسانی ترافیک، ورود به پنل، Backup دیتابیس و ... استفاده می‌شود. نیازمند تنظیم:

- توکن تلگرام
- Chat ID ادمین‌ها
- زمان اطلاع‌رسانی (Cron syntax)
- اطلاع‌رسانی انقضا
- اطلاع‌رسانی ترافیک
- Backup دیتابیس
- اطلاع‌رسانی مصرف CPU

**سینتکس نمونه:**

- `30 \* \* \* \* \*` - اطلاع در ثانیه 30 هر دقیقه
- `@hourly` - هر ساعت
- `@daily` - هر روز

### ویژگی‌های ربات

- گزارش دوره‌ای
- اطلاع ورود به پنل
- اطلاع مصرف CPU
- اطلاع پیش‌از موعد انقضا و ترافیک
- گزارش ترافیک کلاینت‌ها
- منوی مبتنی بر دستور
- جستجوی کلاینت بر اساس ایمیل
- بررسی inboundها
- بررسی وضعیت سرور
- دریافت Backup
- چندزبانه

### راه‌اندازی ربات

- شروع [Botfather](https://t.me/BotFather) در تلگرام:
    ![Botfather](./media/botfather.png)

- ساخت ربات جدید با دستور /newbot:
    ![Create new bot](./media/newbot.png)

- شروع ربات ساخته شده:
    ![token](./media/token.png)

- تنظیمات پنل:
![Panel Config](./media/panel-bot-config.png)

وارد کردن توکن و Chat ID (دریافت از [این ربات](https://t.me/useridinfobot)):
![User ID](./media/user-id.png)

</details>

## مسیرهای API

<details>
  <summary>جزئیات API</summary>

#### استفاده

- [مستندات API](https://www.postman.com/hsanaei/3x-ui/collection/q1l5l0u/3x-ui)
- `/login` با `POST` داده کاربر: `{username: '', password: ''}`

| Method | مسیر                               | عملکرد                                      |
| :----: | ---------------------------------- | ------------------------------------------- |
| `GET`  | `"/list"`                          | دریافت تمام inboundها                      |
| `GET`  | `"/get/:id"`                       | دریافت inbound بر اساس id                  |
| `POST` | `"/add"`                           | افزودن inbound                              |
| `POST` | `"/del/:id"`                       | حذف inbound                                 |

- [<img src="https://run.pstmn.io/button.svg" alt="Run In Postman" style="width: 128px; height: 32px;">](https://app.getpostman.com/run-collection/5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5?action=collection%2Ffork&source=rip_markdown&collection-url=entityId%3D5146551-dda3cab3-0e33-485f-96f9-d4262f437ac5%26entityType%3Dcollection%26workspaceId%3Dd64f609f-485a-4951-9b8f-876b3f917124)
</details>

## متغیرهای محیطی

<details>
  <summary>جزئیات متغیرها</summary>

#### استفاده

| متغیر         |                      نوع                      | پیش‌فرض       |
| ------------- | :--------------------------------------------: | :------------ |
| XUI_LOG_LEVEL | `"debug"` \| `"info"` \| `"warn"` \| `"error"` | `"info"`      |
| XUI_DEBUG     |                   `boolean`                    | `false`       |
| XUI_BIN_FOLDER|                    `string`                    | `"bin"`       |

مثال:

```sh
XUI_BIN_FOLDER="bin" XUI_DB_FOLDER="/etc/x-ui" go build main.go
```

</details>

## پیش‌نمایش

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="3x-ui" src="./media/01-overview-light.png">
</picture>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-inbounds-dark.png">
  <img alt="3x-ui" src="./media/02-inbounds-light.png">
</picture>

## قدردانی ویژه از

- [alireza0](https://github.com/alireza0/)

## تشکر و قدردانی

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (مجوز: **GPL-3.0**)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (مجوز: **GPL-3.0**)

## Stargazers over Time

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)