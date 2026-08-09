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

**این یک فورک شخصی از [3X-UI](https://github.com/MHSanaei/3x-ui) است** — پنل کنترل وب پیشرفته و متن‌باز برای [Xray-core](https://github.com/XTLS/Xray-core) — با یک افزوده‌ی اصلی: **پشتیبانی نیتیو از AmneziaWG**، به‌عنوان یک پروتکل کاملاً درجه‌یک در کنار VLESS، VMess، Trojan و بقیه. هر چیز دیگری که 3X-UI از قبل انجام می‌داد (اینباندهای چندپروتکلی، حسابداری ترافیک به‌ازای هر کلاینت، سابسکریپشن‌ها، چند نود، ربات تلگرام) بدون تغییر باقی مانده و دقیقاً مثل نسخه‌ی اصلی کار می‌کند.

این فورک برای اجرا روی روترها و سرورهای شخصی نویسنده ساخته شده است؛ قصد جایگزینی یا رقابت با پروژه‌ی اصلی را ندارد. اگر به‌دنبال پنل همه‌منظوره هستید، به [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) مراجعه کنید — همه‌ی مطالب زیر فقط تفاوت‌های این فورک را مستند می‌کند.

> [!IMPORTANT]
> این پروژه فقط برای استفاده‌ی شخصی در نظر گرفته شده است. لطفاً از آن برای اهداف غیرقانونی یا در محیط تولید (production) استفاده نکنید.

## تفاوت این فورک: AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) نسخه‌ای از WireGuard است با یک لایه‌ی مبهم‌سازی اضافه (بسته‌های زباله، پدینگ تصادفی، بازنویسی سرآیندهای جادویی) که برای شکست دادن اثرانگشت‌گیری پروتکل مبتنی بر DPI طراحی شده — همان تونل، اما تونلی که روی سیم شبیه تونل به نظر نمی‌رسد.

- **جاسازی‌شده (embedded)، نه یک ماژول کرنل.** AmneziaWG کاملاً درون خود پروسه‌ی پنل اجرا می‌شود ([amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go) روی یک پشته‌ی شبکه در فضای کاربر) — بدون نیاز به بیلد DKMS، بدون تداخل با Secure Boot، بدون کانتینر سایدکار ممتاز، و اصلاً چیزی برای نصب روی هاست وجود ندارد.
- **یک پروتکل درجه‌یک.** یک اینباند AmneziaWG در همان جدول `Inbound` بقیه‌ی موارد زندگی می‌کند، پس bulk-operations، مودال QR/دانلود کانفیگ و لینک‌های سابسکریپشن را رایگان دریافت می‌کند — چیز جدیدی برای یادگیری نیست.
- **مبهم‌سازی کامل AmneziaWG 2.0** — ‏Jc/Jmin/Jmax (بسته‌های زباله)، S1–S4 (پدینگ بسته)، H1–H4 (سرآیندهای جادویی) و هر ۵ اسلات بسته‌ی امضا (I1–I5) — هرکدام با یک کلیک به‌صورت بسته‌ی شبیه‌سازی‌شده‌ی DNS/STUN/SIP/QUIC، یک ClientHello واقعیِ TLS مرورگر Chrome/Firefox/Safari یا کاملاً تصادفی قابل تولیدند — همگی به‌ازای هر اینباند قابل ویرایش‌اند، به‌علاوه‌ی یک حالت سازگار با 1.x برای کلاینت‌های قدیمی‌تر.
- **ترافیک هر کلاینت همین حالا از Xray عبور می‌کند.** بدون TPROXY، بدون پلی که لازم باشد جداگانه فعال شود: هر اینباند AmneziaWG مستقیماً به اینباند SOCKS5 مخصوص به خودش در Xray از طریق loopback ریلی می‌شود، پس آمار هر کلاینت، وضعیت آنلاین، sniffing و قوانین صفحه‌ی «مسیریابی» موجود در پنل، همگی دقیقاً مثل هر پروتکل دیگری به‌طور خودکار کار می‌کنند — بدون هیچ پیکربندی اضافه‌ای.
- **لینک‌های اشتراک‌گذاری واقعی `vpn://`** — لینک کپی/QR هر کلاینت و endpoint سابسکریپشن همان طرح واقعی `vpn://` را که اپ رسمی AmneziaVPN انتظار دارد تولید می‌کنند (base64url یک فایل `.conf` ساده)، نه یک فرمت URI ساختگی که آن اپ نمی‌توانست وارد کند.
- **موقتاً پشتیبانی نمی‌شود** پس از این بازنویسی: یک آدرس IPv6 عمومی مجزا به‌ازای هر کلاینت و پروبرت پورت به‌ازای هر کلاینت، هر دو به قوانین iptables در سطح هاست ماژول کرنل قدیمی متکی بودند که هنوز معادلی در معماری جاسازی‌شده ندارند. هر دو به‌عنوان نسخه‌های بعدی و نزدیک برنامه‌ریزی شده‌اند؛ تنظیمات ذخیره‌شده‌ی موجود برای هرکدام از بین نرفته، فقط تا آن زمان غیرفعال است.

## سایر تغییرات این فورک

بهبودهای کوچک‌تر مخصوص این فورک، فراتر از AmneziaWG، هر بار که اضافه شوند اینجا ثبت می‌شوند:

- **تکمیل خودکار قوانین مسیریابی** — فیلدهای Domain/IP در ویرایشگر قوانین Routing ایکس‌ری اکنون دسته‌بندی‌های geosite/geoip را پیشنهاد می‌دهند (مثلاً تایپ «you» پیشنهاد `geosite:youtube` را می‌دهد) که به‌صورت زنده از فایل‌های `.dat` که واقعاً در پوشه bin ایکس‌ری نصب شده‌اند ساخته می‌شوند — از جمله فایل‌های سفارشی اضافه‌شده از طریق ویژگی به‌روزرسانی خودکار Geodata (مثلاً `geosite_roscom.dat`). ورود متن آزاد هنوز دقیقاً مثل قبل کار می‌کند.
- **سرعت زنده برای AmneziaWG و MTProto** — ستون Speed برای اینباند/کلاینت‌های AmneziaWG و MTProto (`mtg`) «--» نشان می‌داد، با اینکه مجموع ترافیک تجمعی درست بود — چون هیچ‌کدام داخل رانتایم خود Xray-core اجرا نمی‌شوند و به همین دلیل برای API آمار آن نامرئی‌اند. اکنون هر دو، سرعت زنده را دقیقاً مثل بقیه‌ی پروتکل‌ها پخش می‌کنند.
- **فایل‌های سفارشی در `bin/` از آپدیت جان سالم به در می‌برند** — نصب مجدد/آپدیت قبلاً کل پوشه‌ی `bin/` را قبل از استخراج نسخه‌ی جدید کاملاً پاک می‌کرد و هر چیزی که دستی آنجا گذاشته شده بود (معمولاً یک فایل سفارشی geoip/geosite که یک قانون مسیریابی از طریق `ext:<file>:<code>` به آن ارجاع می‌دهد) را بی‌سروصدا حذف می‌کرد و همه‌ی اینباندها را در اجرای بعدی خراب می‌کرد. اکنون نصب‌کننده ابتدا از `bin/` بکاپ می‌گیرد و فقط چیزهایی را که نسخه‌ی جدید ارائه نمی‌دهد بازمی‌گرداند.

## ویژگی‌ها

- **اینباندهای چندپروتکلی** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، **AmneziaWG**، Hysteria2، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **ترنسپورت‌ها و امنیت مدرن** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، ایمن‌شده با TLS، XTLS و REALITY.
- **فال‌بک (Fallback)** — ارائه‌ی چند پروتکل روی یک پورت واحد (مثلاً VLESS و Trojan روی پورت 443) با استفاده از قابلیت fallback در Xray.
- **مدیریت به‌ازای هر کلاینت** — سهمیه‌ی ترافیک، تاریخ انقضا، محدودیت IP، وضعیت آنلاینِ زنده و لینک‌های اشتراک‌گذاری، کدهای QR و سابسکریپشن‌ها با یک کلیک.
- **آمار ترافیک** — به‌ازای هر اینباند، هر کلاینت و هر اوتباند، همراه با کنترل بازنشانی (reset).
- **پشتیبانی از چند نود** — مدیریت و مقیاس‌دهی روی چندین سرور از یک پنل واحد.
- **اوتباند و مسیریابی** — WARP، NordVPN، قوانین مسیریابی سفارشی، متعادل‌کننده‌های بار (load balancer) و زنجیره‌کردن پراکسی اوتباند.
- **سرور سابسکریپشن داخلی** با چندین فرمت خروجی و [قالب‌های صفحه‌ی سفارشی](docs/custom-subscription-templates.md).
- **ربات تلگرام** برای نظارت و مدیریت از راه دور.
- **‏RESTful API** همراه با مستندات Swagger درون‌پنل.
- **ذخیره‌سازی منعطف** — SQLite (پیش‌فرض) یا PostgreSQL.
- **‏۱۳ زبان رابط کاربری** با تم‌های تیره و روشن.
- **یکپارچگی با Fail2ban** برای اعمال محدودیت IP به‌ازای هر کلاینت.

## اسکرین‌شات‌ها

<details>
<summary>برای باز شدن کلیک کنید</summary>

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

## شروع سریع

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash
```

برای نصب یک نسخه‌ی مشخص، تگ آن را اضافه کنید (مثلاً `v3.5.0-awg.1`):

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s v3.5.0-awg.1
```

برای نصب بیلد غلتانِ **dev** (آخرین پیش‌انتشار بر اساس هر commit از `main`، نه یک نسخه‌ی پایدار)، مقدار `dev` را پاس بدهید:

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

انتشارهای پایدار خودِ این فورک با تگ `<نسخه‌ی پایه‌ی آپستریم>-awg.N` مشخص می‌شوند (مثلاً `v3.5.0-awg.1`، ساخته‌شده روی چیزی که آپستریم آن را `v3.5.0` می‌نامد) — هرگز یک `vX.Y.Z` ساده نیست — تا هرگز با یک انتشار واقعی از MHSanaei/3x-ui با همان شماره اشتباه گرفته نشود.

در حین نصب، یک نام کاربری، رمز عبور و مسیر دسترسی تصادفی تولید می‌شود. پس از نصب، دستور `x-ui` را اجرا کنید تا منوی مدیریت باز شود؛ در آنجا می‌توانید سرویس را شروع/متوقف کنید، اطلاعات ورود خود را ببینید یا بازنشانی کنید، گواهی‌های SSL را مدیریت کنید و کارهای دیگری انجام دهید.

برای مستندات کامل پنل فراتر از آنچه در این README آمده، به [ویکی پروژه‌ی اصلی](https://github.com/MHSanaei/3x-ui/wiki) مراجعه کنید — هیچ‌کدام مختص این فورک نیست، پس همچنان کاربرد دارد.

### نصب بدون نظارت

نصب‌کننده به‌صورت **غیرتعاملی** نیز برای cloud-init اجرا می‌شود.
‏`XUI_NONINTERACTIVE=1` را تنظیم کنید (یا بدون TTY از طریق pipe اجرا کنید) تا نصب به‌صورت سرتاسری و بدون
هیچ پرسشی انجام شود، اطلاعات ورود تصادفی تولید کرده و آن‌ها را در
`/etc/x-ui/install-result.env` می‌نویسد. برای موارد زیر به [`deploy/`](deploy/) مراجعه کنید:

- [user-data مربوط به Cloud-init](deploy/cloud-init/) — نصب بدون نظارت روی هر ابری (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [یادداشت‌های Hetzner Cloud](deploy/marketplace/hetzner/) — استقرار مبتنی بر cloud-init روی Hetzner

## پلتفرم‌های پشتیبانی‌شده

**سیستم‌عامل‌ها:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap) و Alpine. (پروژه‌ی اصلی یک نسخه‌ی Windows نیز منتشر می‌کند؛ CI این فورک این کار را نمی‌کند — همه‌چیز اینجا سرورها/روترهای لینوکسی را هدف قرار می‌دهد.)

**معماری‌ها:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

‏AmneziaWG مستقیماً درون خود باینری پنل جاسازی شده است (به بخش [تفاوت این فورک](#تفاوت-این-فورک-amneziawg) مراجعه کنید) — بدون ماژول کرنل، بدون مرحله‌ی نصب جداگانه، بدون تنظیمات مخصوص هر توزیع.

## گزینه‌های پایگاه‌داده

‏3X-UI از دو بک‌اند پشتیبانی می‌کند که در حین نصب انتخاب می‌شوند:

- **SQLite** (پیش‌فرض) — یک فایل واحد در مسیر `/etc/x-ui/x-ui.db`. بدون نیاز به تنظیمات، ایده‌آل برای استقرارهای کوچک و متوسط.
- **PostgreSQL** — برای تعداد کلاینت بالا یا راه‌اندازی‌های چندنودی توصیه می‌شود. نصب‌کننده می‌تواند PostgreSQL را به‌صورت محلی برایتان نصب کند، یا یک DSN به یک سرور موجود را بپذیرد.

در زمان اجرا، بک‌اند از طریق متغیرهای محیطی انتخاب می‌شود (نصب‌کننده این موارد را برای شما در `/etc/default/x-ui` می‌نویسد):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### انتقال یک نصب موجود SQLite به PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# سپس XUI_DB_TYPE و XUI_DB_DSN را در /etc/default/x-ui تنظیم کرده و ری‌استارت کنید:
systemctl restart x-ui
```

فایل اصلی SQLite دست‌نخورده باقی می‌ماند؛ پس از اطمینان از صحت بک‌اند جدید، آن را به‌صورت دستی حذف کنید.

## متغیرهای محیطی

| متغیر | توضیحات | پیش‌فرض |
| --- | --- | --- |
| `XUI_DB_TYPE` | بک‌اند پایگاه‌داده: `sqlite` یا `postgres` | `sqlite` |
| `XUI_DB_DSN` | رشته‌ی اتصال PostgreSQL (وقتی `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | پوشه‌ی فایل پایگاه‌داده‌ی SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | حداکثر اتصالات باز (استخر PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | حداکثر اتصالات بی‌کار (استخر PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | مسیر URI اولیه برای پنل وب | `/` |
| `XUI_ENABLE_FAIL2BAN` | فعال‌سازی اعمال محدودیت IP مبتنی بر Fail2ban | `true` |
| `XUI_LOG_LEVEL` | سطح گزارش‌گیری (`debug`، `info`، `warning`، `error`) | `info` |
| `XUI_DEBUG` | فعال‌سازی حالت دیباگ | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | فعال‌سازی پایشگر سلامت تونل (یک URL را پروب می‌کند و پس از خطاهای مکرر، xray را ری‌استارت می‌کند؛ یک ری‌استارت همه‌ی کلاینت‌ها را قطع می‌کند) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | پراکسی‌ای که پروب از طریق آن ارسال می‌شود؛ آن را به یک اینباند محلی xray اشاره دهید تا پروب خودِ تونل را آزمایش کند (مثلاً `socks5://127.0.0.1:1080`). خالی بودن یعنی پروب فقط اتصال به هاست را بررسی می‌کند | — |
| `XUI_TUNNEL_HEALTH_URL` | URL ای که برای سلامت تونل پروب می‌شود | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | فاصله‌ی زمانی بین پروب‌ها | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | مهلت زمانی هر پروب | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | تعداد خطاهای متوالی پیش از آن‌که یک ری‌استارت فعال شود | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | حداقل تأخیر بین ری‌استارت‌های متوالی | `5m` |

## زبان‌های پشتیبانی‌شده

رابط کاربری پنل به ۱۳ زبان در دسترس است:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## یادداشت‌های توسعه‌دهنده

این یک فورک شخصی است و به‌دنبال مشارکت‌کننده‌ی بیرونی نیست، اما [CONTRIBUTING.md](/CONTRIBUTING.md) همچنان دستورالعمل‌های دقیق و مفیدی برای راه‌اندازی محیط توسعه‌ی محلی (نسخه‌های Go/Node، کامپایلر C مورد نیاز CGo، دستورات build/lint/test) دارد، اگر خودتان روی این کدبیس کار می‌کنید.

## قدردانی

این فورک به‌طور کامل بر پایه‌ی [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) ساخته شده — تمام پنل، پشتیبانی چندپروتکلی و معماری زیرین کار خودشان است؛ **پشتیبانی AmneziaWG تنها چیزی است که اینجا اضافه شده.** اگر پروژه‌ی اصلی را مفید یافتید، لینک‌های حمایتی نویسنده‌ی اصلی همچنان مکان درستی برای آن است:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

پیاده‌سازی AmneziaWG در این فورک برگرفته از/الهام‌گرفته از موارد زیر است:

- [amnezia-vpn/amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go) — پیاده‌سازی AmneziaWG در فضای کاربر که این فورک مستقیماً درون پروسه‌ی پنل، روی پشته‌ی شبکه‌ی [gVisor](https://gvisor.dev/)، جاسازی می‌کند و جایگزین بک‌اند قدیمی مبتنی بر ماژول کرنل در زیر می‌شود.
- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — پول‌ریکوئست اصلی AmneziaWG علیه پروژه‌ی اصلی (رویکرد Docker-sidecar)؛ این فورک ساختار schema/UI فرانت‌اند آن را دوباره استفاده می‌کند.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — یک فورک مستقل که از قبل AmneziaWG نیتیو را در محیط تولید اجرا می‌کند؛ مدیر قدیمی این فورک مبتنی بر ماژول کرنل (`awg-quick`) و تولیدکننده‌ی پارامتر مبهم‌سازی AmneziaWG 2.0 پیش از این بازنویسی از پکیج `awg/` آن برگرفته شده بودند.

## تشکر ویژه از

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (مجوز: **GPL-3.0**): _قوانین مسیریابی بهبود یافته v2ray/xray و v2ray/xray-clients با دامنه‌های ایرانی داخلی و تمرکز بر امنیت و مسدود کردن تبلیغات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (مجوز: **GPL-3.0**): _این مخزن شامل قوانین مسیریابی V2Ray به‌روزرسانی شده خودکار بر اساس داده‌های دامنه‌ها و آدرس‌های مسدود شده در روسیه است._

## ابزارهای جامعه

ابزارها و یکپارچه‌سازی‌هایی که توسط جامعه پیرامون 3x-ui ساخته شده‌اند.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (مجوز: **MIT**): _مدیریت اینباندها، کلاینت‌ها، تنظیمات پنل و پیکربندی Xray به‌صورت کد با Terraform / OpenTofu._
