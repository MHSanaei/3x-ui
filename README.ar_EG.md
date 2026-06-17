[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/dune-dark.png">
    <img alt="dune" src="./media/dune-light.png">
  </picture>
</p>

<p align="center">
  <a href="https://github.com/leto217/DUNE/releases"><img src="https://img.shields.io/github/v/release/leto217/DUNE" alt="Release"></a>
  <a href="https://github.com/leto217/DUNE/actions"><img src="https://img.shields.io/github/actions/workflow/status/leto217/DUNE/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/leto217/DUNE.svg" alt="GO Version"></a>
  <a href="https://github.com/leto217/DUNE/releases/latest"><img src="https://img.shields.io/github/downloads/leto217/DUNE/total.svg" alt="Downloads"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
  <a href="https://pkg.go.dev/github.com/leto217/DUNE"><img src="https://pkg.go.dev/badge/github.com/leto217/DUNE.svg" alt="Go Reference"></a>
  <a href="https://goreportcard.com/report/github.com/leto217/DUNE"><img src="https://goreportcard.com/badge/github.com/leto217/DUNE" alt="Go Report Card"></a>
</p>

**DUNE** هي نسخة خفيفة (fork) من [3X-UI](https://github.com/MHSanaei/3x-ui) — لوحة تحكم ويب مفتوحة المصدر لإدارة خوادم [Xray-core](https://github.com/XTLS/Xray-core). تحافظ على سير العمل المألوف وتغطية البروتوكولات في 3X-UI مع استهلاك أقل بكثير لموارد CPU وRAM، مما يجعلها مثالية لخوادم VPS الصغيرة والبيئات محدودة الموارد.

مشتقة من 3X-UI مع التركيز على الكفاءة: تقليل المهام الخلفية، وضبط استخدام الذاكرة، وتبسيط المكدس حتى تبقى اللوحة سريعة الاستجابة دون إرهاق الخادم.

> [!IMPORTANT]
> هذا المشروع مخصص للاستخدام الشخصي فقط. يرجى عدم استخدامه لأغراض غير قانونية أو في بيئة إنتاجية.

## الميزات

- **اتصالات واردة متعددة البروتوكولات** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، Hysteria2، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **وسائل نقل وأمان حديثة** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، مؤمَّنة بـ TLS و XTLS و REALITY.
- **Fallback** — تقديم عدة بروتوكولات على منفذ واحد (مثل VLESS و Trojan على المنفذ 443) باستخدام ميزة fallback في Xray.
- **إدارة لكل عميل** — حصص الترافيك، تواريخ انتهاء الصلاحية، حدود IP، حالة الاتصال المباشرة، وروابط مشاركة وأكواد QR واشتراكات بنقرة واحدة.
- **إحصائيات الترافيك** — لكل اتصال وارد، ولكل عميل، ولكل اتصال صادر، مع عناصر تحكم لإعادة التعيين.
- **دعم العقد المتعددة** — إدارة وتوسيع عبر عدة خوادم من لوحة واحدة.
- **الاتصالات الصادرة والتوجيه** — WARP، NordVPN، قواعد توجيه مخصصة، موازنات تحميل، وتسلسل الوكلاء الصادرة.
- **خادم اشتراك مدمج** بصيغ إخراج متعددة.
- **روبوت تيليجرام** للمراقبة والإدارة عن بُعد.
- **واجهة RESTful API** مع توثيق Swagger داخل اللوحة.
- **تخزين مرن** — SQLite (افتراضي) أو PostgreSQL.
- **13 لغة لواجهة المستخدم** مع سمات داكنة وفاتحة.
- **تكامل مع Fail2ban** لفرض حدود IP لكل عميل.

## لقطات الشاشة

<details>
<summary>انقر للتوسيع</summary>

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

## البدء السريع

```bash
bash <(curl -Ls https://raw.githubusercontent.com/leto217/DUNE/main/install.sh)
```

أثناء التثبيت، يتم إنشاء اسم مستخدم وكلمة مرور ومسار وصول عشوائية. بعد التثبيت، شغّل `dune` لفتح قائمة الإدارة، حيث يمكنك بدء/إيقاف الخدمة، وعرض أو إعادة تعيين بيانات تسجيل الدخول، وإدارة شهادات SSL، والمزيد.

للحصول على الوثائق الكاملة، يرجى زيارة [ويكي المشروع](https://github.com/leto217/DUNE/wiki).

## المنصات المدعومة

**أنظمة التشغيل:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap)، Alpine و Windows.

**المعماريات:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## خيارات قاعدة البيانات

يدعم Dune خلفيتين (backends) يتم اختيارهما أثناء التثبيت:

- **SQLite** (افتراضي) — ملف واحد في `/etc/dune/dune.db`. بدون إعداد، مثالي لعمليات النشر الصغيرة والمتوسطة.
- **PostgreSQL** — موصى به لأعداد العملاء الكبيرة أو الإعدادات متعددة العقد. يمكن للمثبِّت تثبيت PostgreSQL محليًا لك، أو قبول DSN لخادم موجود.

في وقت التشغيل، يتم اختيار الخلفية عبر متغيرات البيئة (يكتبها المثبِّت لك في `/etc/default/dune`):

```
DUNE_DB_TYPE=postgres
DUNE_DB_DSN=postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable
```

### ترحيل تثبيت SQLite موجود إلى PostgreSQL

```bash
dune migrate-db --dsn "postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable"
# ثم عيّن DUNE_DB_TYPE و DUNE_DB_DSN في /etc/default/dune وأعد التشغيل:
systemctl restart dune
```

يبقى ملف SQLite الأصلي دون تغيير؛ احذفه يدويًا بعد التحقق من الخلفية الجديدة.

### Docker

يستمر الأمر الافتراضي `docker compose up -d` في استخدام SQLite. للتشغيل مع خدمة PostgreSQL المرفقة، أزِل التعليق عن سطري متغيرات البيئة `DUNE_DB_*` في `docker-compose.yml` وشغّل باستخدام البروفايل:

```bash
docker compose --profile postgres up -d
```

تتضمن الصورة Fail2ban (مُفعَّل افتراضيًا) لفرض **حدود IP** لكل عميل. يحظر Fail2ban المخالفين باستخدام `iptables`، الذي يتطلب صلاحية `NET_ADMIN`. يمنح `docker-compose.yml` هذه الصلاحية مسبقًا عبر `cap_add`؛ إذا شغّلت الحاوية باستخدام `docker run` بدلاً من ذلك، فأضِف الصلاحيات بنفسك، وإلا فسيتم تسجيل عمليات الحظر دون تطبيقها أبدًا:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/leto217/DUNE
```

## متغيرات البيئة

| المتغير | الوصف | الافتراضي |
| --- | --- | --- |
| `DUNE_DB_TYPE` | خلفية قاعدة البيانات: `sqlite` أو `postgres` | `sqlite` |
| `DUNE_DB_DSN` | سلسلة اتصال PostgreSQL (عندما `DUNE_DB_TYPE=postgres`) | — |
| `DUNE_DB_FOLDER` | مجلد ملف قاعدة بيانات SQLite | `/etc/dune` |
| `DUNE_DB_MAX_OPEN_CONNS` | الحد الأقصى للاتصالات المفتوحة (تجمّع PostgreSQL) | — |
| `DUNE_DB_MAX_IDLE_CONNS` | الحد الأقصى للاتصالات الخاملة (تجمّع PostgreSQL) | — |
| `DUNE_INIT_WEB_BASE_PATH` | مسار URI الأولي للوحة الويب | `/` |
| `DUNE_ENABLE_FAIL2BAN` | تفعيل فرض حدود IP المعتمد على Fail2ban | `true` |
| `DUNE_LOG_LEVEL` | مستوى السجل (`debug`، `info`، `warning`، `error`) | `info` |
| `DUNE_DEBUG` | تفعيل وضع التصحيح | `false` |

## اللغات المدعومة

تتوفر واجهة اللوحة بـ 13 لغة:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## المساهمة

المساهمات مرحب بها. يرجى قراءة [دليل المساهمة](/CONTRIBUTING.md) قبل فتح مشكلة (issue) أو طلب سحب (pull request).

## شكر خاص إلى

- [alireza0](https://github.com/alireza0/)

## الاعتراف

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (الترخيص: **GPL-3.0**): _قواعد توجيه v2ray/xray و v2ray/xray-clients المحسنة مع النطاقات الإيرانية المدمجة وتركيز على الأمان وحظر الإعلانات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (الترخيص: **GPL-3.0**): _يحتوي هذا المستودع على قواعد توجيه V2Ray محدثة تلقائيًا بناءً على بيانات النطاقات والعناوين المحظورة في روسيا._

## أدوات المجتمع

أدوات وتكاملات بناها المجتمع حول dune.

- [terraform-provider-dune](https://github.com/batonogov/terraform-provider-threexui) (الترخيص: **MIT**): _إدارة الاتصالات الواردة والعملاء وإعدادات اللوحة وتكوين Xray كرمز باستخدام Terraform / OpenTofu._

## دعم المشروع

**إذا كان هذا المشروع مفيدًا لك، فقد ترغب في إعطائه**:star2:

| الشبكة | العنوان |
| --- | --- |
| TON | `UQAa5FpNlK8Gp7tO8luJXHD-Sf0pPjJbNHGo8hdkyuUBhWEa` |
| TRON | `TLqtTfYSzPLFm8mtFDkSnXvzucxx7DS5VL` |
| ERC20 and BEP20 | `0x2fe632d70f4612b87670f8a28b4587ea2641452d` |

## النجوم عبر الزمن

[![Stargazers over time](https://starchart.cc/leto217/DUNE.svg?variant=adaptive)](https://starchart.cc/leto217/DUNE)
