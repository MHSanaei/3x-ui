[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md) | [کوردی](/README.ckb_IQ.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/MHSanaei/3x-ui/releases"><img src="https://img.shields.io/github/v/release/mhsanaei/3x-ui" alt="Release"></a>
  <a href="https://github.com/MHSanaei/3x-ui/actions"><img src="https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg" alt="GO Version"></a>
  <a href="https://github.com/MHSanaei/3x-ui/releases/latest"><img src="https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/mhsanaei/3x-ui/v3"><img src="https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v3.svg" alt="Go Reference"></a>
</p>

**3X-UI** پانێلێکی کۆنترۆڵی وێبی پێشکەوتوو و سەرچاوە-کراوەیە بۆ بەڕێوەبردنی سێرڤەرەکانی [Xray-core](https://github.com/XTLS/Xray-core). ڕووکارێکی پاک و فرەزمان دابین دەکات بۆ بەکارخستن، شێوەپێدان و چاودێریکردنی کۆمەڵێک فراوان لە پڕۆتۆکۆلی پڕۆکسی و VPN — لە یەک VPS ـەوە تا بەکارخستنی فرە-نۆد.

وەک فۆرکێکی باشترکراو لە پڕۆژەی سەرەکی X-UI دروستکراوە، 3X-UI پشتگیریی فراوانتری پڕۆتۆکۆل، جێگیریی باشتر، ژمێریاری تڕافیک بۆ هەر کڕیارێک و زۆر تایبەتمەندیی کاریگەر زیاد دەکات.

> [!IMPORTANT]
> ئەم پڕۆژەیە تەنها بۆ بەکارهێنانی کەسی دیزاینکراوە. تکایە بۆ مەبەستی نایاسایی یان لە ژینگەیەکی production ـدا بەکاری مەهێنە.

## تایبەتمەندییەکان

- **ئینباوەندی فرە-پڕۆتۆکۆل** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، Hysteria2، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **گواستنەوە و ئاسایشی سەردەم** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، پارێزراو بە TLS، XTLS و REALITY.
- **Fallback ـەکان** — پێشکەشکردنی چەند پڕۆتۆکۆل لەسەر یەک پۆرت (بۆ نموونە VLESS و Trojan لەسەر 443) بە بەکارهێنانی پشتگیریی fallback ی Xray.
- **بەڕێوەبردن بۆ هەر کڕیارێک** — سنووری تڕافیک، بەرواری بەسەرچوون، سنووری IP، دۆخی ئۆنلاینی زیندوو، و بەستەری هاوبەشکردن، کۆدی QR و بەشداریکردن بە یەک کلیک.
- **ئاماری تڕافیک** — بۆ هەر ئینباوەندێک، هەر کڕیارێک و هەر دەرچوونێک، لەگەڵ کۆنترۆڵی ڕیسێتکردن.
- **پشتگیریی فرە-نۆد** — بەڕێوەبردن و فراوانکردن بەسەر چەندین سێرڤەردا لە یەک پانێلەوە.
- **دەرچوون و ڕێنیشاندن** — WARP، NordVPN، یاسای ڕێنیشاندنی دیاریکراو، باڵانسکەری بار و زنجیرەکردنی پڕۆکسی دەرچوون.
- **سێرڤەری بەشداریکردنی سەرەکی** لەگەڵ چەند فۆرماتی دەرچوون و [داڕێژەی لاپەڕەی دیاریکراو](docs/custom-subscription-templates.md).
- **بۆتی تلگرام** بۆ چاودێری و بەڕێوەبردنی دوورەوە.
- **RESTful API** لەگەڵ بەڵگەنامەی Swagger لەناو پانێلدا.
- **هەڵگرتنی نەرم** — SQLite (بنەڕەت) یان PostgreSQL.
- **١٤ زمانی ڕووکار** لەگەڵ ڕووکاری تاریک و ڕووناک.
- **یەکخستن لەگەڵ Fail2ban** بۆ جێبەجێکردنی سنووری IP بۆ هەر کڕیارێک.

## وێنەگرتنەکان

<details>
<summary>کلیک بکە بۆ کردنەوە</summary>

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

## دەستپێکردنی خێرا

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

بۆ دامەزراندنی وەشانێکی دیاریکراو، تاگەکەی زیاد بکە (بۆ نموونە `v3.4.0`):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) v3.4.0
```

بۆ دامەزراندنی وەشانی گەشەپێدانی **dev** (دوایین پێش-بڵاوکراوەی هەر کۆمیتێک لە `main`، نەک بڵاوکراوەیەکی جێگیر)، `dev-latest` بنێرە:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh) dev-latest
```

لە کاتی دامەزراندندا ناوی بەکارهێنەر، وشەی نهێنی و ڕێچکەی گەیشتنێکی هەڕەمەکی دروست دەکرێت. دوای دامەزراندن، فەرمانی `x-ui` جێبەجێ بکە بۆ کردنەوەی مینیوی بەڕێوەبردن، کە لەوێدا دەتوانیت خزمەتگوزاریەکە دەستپێبکەیت/ڕایبگریت، زانیاری چوونەژوورەوەت ببینیت یان ڕیسێتی بکەیتەوە، بڕوانامەکانی SSL بەڕێوە ببەیت و شتی زیاتر.

بۆ بەڵگەنامەی تەواو، تکایە سەردانی [Wiki ی پڕۆژەکە](https://github.com/MHSanaei/3x-ui/wiki) بکە.

### دامەزراندنی بێ-چاودێری

دامەزرێنەرەکە بە شێوەی **ناتێکەڵاو** ـیش بۆ cloud-init کاردەکات.
`XUI_NONINTERACTIVE=1` دابنێ (یان بەبێ TTY لە ڕێگەی pipe ـەوە جێبەجێی بکە) بۆ ئەوەی دامەزراندن بەبێ
هیچ پرسیارێک تەواو بێت، زانیاری چوونەژوورەوەی هەڕەمەکی دروست دەکات و لە
`/etc/x-ui/install-result.env` دەینووسێت. بۆ ئەمانە سەردانی [`deploy/`](deploy/) بکە:

- [user-data ی Cloud-init](deploy/cloud-init/) — دامەزراندنی بێ-چاودێری لەسەر هەر هەورێک (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [تێبینیەکانی Hetzner Cloud](deploy/marketplace/hetzner/) — بەکارخستنی cloud-init لەسەر Hetzner

## پلاتفۆرمە پشتگیریکراوەکان

**سیستەمی کارپێکردن:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap)، Alpine و Windows.

**پێکهاتەکان:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## بژاردەکانی داتابەیس

3X-UI پشتگیری لە دوو بەشی پشتەوە دەکات، کە لە کاتی دامەزراندندا هەڵدەبژێردرێت:

- **SQLite** (بنەڕەت) — تاکە فایلێک لە `/etc/x-ui/x-ui.db`. بێ ڕێکخستن، گونجاو بۆ بەکارخستنی بچووک و مامناوەند.
- **PostgreSQL** — پێشنیارکراو بۆ ژمارەی زۆری کڕیار یان بەکارخستنی فرە-نۆد. دامەزرێنەرەکە دەتوانێت PostgreSQL بە شێوەی ناوخۆیی بۆت دابمەزرێنێت، یان DSN ـێک بۆ سێرڤەرێکی بوونیاندوو وەربگرێت.

لە کاتی کارپێکردندا، بەشی پشتەوە لە ڕێگەی گۆڕاوی ژینگەوە هەڵدەبژێردرێت (دامەزرێنەرەکە ئەمانە بۆت لە `/etc/default/x-ui` دەنووسێت):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### گواستنەوەی دامەزراندنێکی SQLite ی بوونیاندوو بۆ PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# پاشان XUI_DB_TYPE و XUI_DB_DSN لە /etc/default/x-ui دابنێ و دووبارە دەستپێبکەرەوە:
systemctl restart x-ui
```

فایلی سەرچاوەی SQLite دەست نەخراو دەمێنێتەوە؛ دوای دڵنیابوون لە بەشی پشتەوەی نوێ، بە دەستی بیسڕەوە.

### Docker

فەرمانی بنەڕەتی `docker compose up -d` هەروا بەردەوامە لەگەڵ SQLite. بۆ کارپێکردن لەگەڵ خزمەتگوزاری PostgreSQL ی هاوپێچکراو، دوو دێڕی گۆڕاوی ژینگەی `XUI_DB_*` لە `docker-compose.yml` لە کۆمێنت دەربهێنە و بە پرۆفایلەکە دەستپێبکە:

```bash
docker compose --profile postgres up -d
```

ئیمەیجەکە Fail2ban ـی هاوپێچکراو هەیە (بە بنەڕەت چالاکە) بۆ جێبەجێکردنی **سنووری IP** بۆ هەر کڕیارێک. Fail2ban تاوانباران بە `iptables` بان دەکات، کە پێویستی بە توانای `NET_ADMIN` هەیە. `docker-compose.yml` پێشتر ئەمە لە ڕێگەی `cap_add` دەدات؛ ئەگەر لەبری ئەوە کۆنتەینەرەکە بە `docker run` دەستپێدەکەیت، خۆت تواناکان زیاد بکە، بەپێچەوانەوە بانکردنەکان تەنها تۆمار دەکرێن بەڵام هەرگیز جێبەجێ ناکرێن:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## گۆڕاوەکانی ژینگە

| گۆڕاو | ڕوونکردنەوە | بنەڕەت |
| --- | --- | --- |
| `XUI_DB_TYPE` | بەشی پشتەوەی داتابەیس: `sqlite` یان `postgres` | `sqlite` |
| `XUI_DB_DSN` | دەقی پەیوەندیی PostgreSQL (کاتێک `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | بوخچە بۆ فایلی داتابەیسی SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | زۆرترین پەیوەندیی کراوە (کۆگای PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | زۆرترین پەیوەندیی بێکار (کۆگای PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | ڕێچکەی سەرەتای URI بۆ پانێلی وێب | `/` |
| `XUI_ENABLE_FAIL2BAN` | چالاککردنی جێبەجێکردنی سنووری IP بەپێی Fail2ban | `true` |
| `XUI_LOG_LEVEL` | ئاستی لۆگ (`debug`، `info`، `warning`، `error`) | `info` |
| `XUI_DEBUG` | چالاککردنی دۆخی debug | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | چالاککردنی چاودێری تەندروستیی تونێل (URL ـێک پشکنین دەکات و دوای هەڵەی دووبارە xray دەستپێدەکاتەوە؛ دەستپێکردنەوەیەک هەموو کڕیاران دەبڕێتەوە) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | پڕۆکسیەک کە پشکنینەکە لە ڕێگەیدا دەنێردرێت؛ بیبەستەوە بە ئینباوەندێکی ناوخۆیی xray بۆ ئەوەی پشکنینەکە خودی تونێلەکە تاقی بکاتەوە (بۆ نموونە `socks5://127.0.0.1:1080`). بەتاڵ بوونی واتای ئەوەیە پشکنینەکە تەنها پەیوەندیی هۆست دەپشکنێت | — |
| `XUI_TUNNEL_HEALTH_URL` | URL ـی پشکنراو بۆ تەندروستیی تونێل | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | ماوە لەنێوان پشکنینەکان | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | کاتی چاوەڕوانی هەر پشکنینێک | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | هەڵەی بەردەوام پێش ئەوەی دەستپێکردنەوەیەک چالاک بێت | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | کەمترین دواکەوتن لەنێوان دەستپێکردنەوە بەردەوامەکان | `5m` |

## زمانە پشتگیریکراوەکان

ڕووکاری پانێل بە ١٤ زمان بەردەستە:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil) · کوردی

## بەشداریکردن

بەشداریکردن بەخێربێن. تکایە پێش کردنەوەی issue یان pull request، [ڕێبەری بەشداریکردن](/CONTRIBUTING.md) بخوێنەرەوە.

## سوپاسێکی تایبەت بۆ

- [alireza0](https://github.com/alireza0/)

## متمانەدارێتی

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (مۆڵەت: **GPL-3.0**): _یاساکانی ڕێنیشاندنی باشترکراوی v2ray/xray و v2ray/xray-clients لەگەڵ دۆمەینی ئێرانی هاوپێچکراو و تیشکخستنە لەسەر ئاسایش و ڕاگرتنی ڕیکلام._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (مۆڵەت: **GPL-3.0**): _ئەم کۆگایە یاساکانی ڕێنیشاندنی V2Ray ی بەخۆکاری نوێکراوەتەوە لەخۆدەگرێت لەسەر بنەمای داتای دۆمەین و ناونیشانی ڕاگیراو لە ڕووسیا._

## ئامرازەکانی کۆمەڵگا

ئامراز و یەکخستنەکان کە لەلایەن کۆمەڵگاوە دەوروبەری 3x-ui دروستکراون.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (مۆڵەت: **MIT**): _بەڕێوەبردنی ئینباوەند، کڕیار، ڕێکخستنی پانێل و شێوەپێدانی Xray وەک کۆد بە Terraform / OpenTofu._

## پشتگیریی پڕۆژە

**ئەگەر ئەم پڕۆژەیە بەسوودە بۆت، دەتوانیت یەک** :star2: **بیدەیتێ**

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Stargazer لە درێژایی کاتدا

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
