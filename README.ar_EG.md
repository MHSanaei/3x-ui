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

**هذه نسخة محسّنة (fork) شخصية من [3X-UI](https://github.com/MHSanaei/3x-ui)** — لوحة تحكم ويب متقدمة ومفتوحة المصدر لـ [Xray-core](https://github.com/XTLS/Xray-core) — بإضافة رئيسية واحدة: **دعم نيتيف لـ AmneziaWG** كبروتوكول من الدرجة الأولى إلى جانب VLESS وVMess وTrojan وغيرها. كل ما كان 3X-UI يقوم به سابقًا (اتصالات واردة متعددة البروتوكولات، محاسبة الترافيك لكل عميل، الاشتراكات، تعدد العقد، روبوت تيليجرام) يبقى دون تغيير ويعمل تمامًا كما في المشروع الأصلي.

بُنيت هذه النسخة لتعمل على أجهزة التوجيه والخوادم الشخصية لصاحبها؛ وليست بديلاً أو منافسًا للمشروع الأصلي. إذا كنت تبحث عن اللوحة متعددة الأغراض، توجّه إلى [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — كل ما يلي هنا يوثّق فقط الفروقات في هذه النسخة.

> [!IMPORTANT]
> هذا المشروع مخصص للاستخدام الشخصي فقط. يرجى عدم استخدامه لأغراض غير قانونية أو في بيئة إنتاجية.

## ما الذي يختلف في هذه النسخة: AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) هو نسخة من WireGuard تضيف طبقة تمويه (حزم مهملة، حشو عشوائي، إعادة كتابة رؤوس سحرية) مصممة لهزيمة أخذ البصمات القائم على DPI للبروتوكول — نفس النفق، لكنه لا يبدو كنفق على السلك.

- **نيتيف، وليس Docker.** يعمل AmneziaWG كواجهة نواة حقيقية على المضيف، تُرفَع وتُخفَض عبر `awg-quick`/`awg` — نفس نهج وحدة النواة DKMS التي تمنحك واجهة `wg0` نيتيف. لا حاجة لحاوية جانبية ذات صلاحيات مرتفعة.
- **بروتوكول من الدرجة الأولى.** يعيش اتصال AmneziaWG الوارد في نفس جدول `Inbound` مثل البقية، لذا يحصل مجانًا على العمليات الجماعية (bulk operations)، ونافذة رمز QR/تنزيل التكوين، وروابط الاشتراك — لا يوجد شيء جديد لتعلّمه.
- **تمويه AmneziaWG 2.0 الكامل** — Jc/Jmin/Jmax (الحزم المهملة)، وS1–S4 (حشو الحزم)، وH1–H4 (الرؤوس السحرية)، وحزمة التوقيع I1، جميعها قابلة للتعديل لكل اتصال وارد مع زر عشوائي بنقرة واحدة، بالإضافة إلى وضع متوافق مع 1.x للعملاء القدامى.
- **دعم IPv6 نيتيف**، مع وكيل NDP لكل عميل بحيث يحصل كل نظير على عنوان IPv6 يمكن الوصول إليه مباشرة — بدون NAT66.
- **إعادة توجيه المنافذ لكل عميل** — DNAT لمنافذ/نطاقات محددة مباشرة إلى عنوان نفق نظير واحد.
- **توجيه ترافيك عميل عبر Xray** — يحصل كل اتصال AmneziaWG الوارد تلقائيًا على جسر Xray الخاص به عبر loopback (بدون أي مفتاح تبديل)؛ يتم توجيه ترافيك أي عميل إلى أي اتصال صادر مُهيّأ في Xray من خلال صفحة "التوجيه" الموجودة مسبقًا في اللوحة، تمامًا كما تُوجَّه أي بروتوكول آخر.
- **يقوم `install.sh` بتثبيت وحدة النواة نيابةً عنك** على Ubuntu/Debian/Armbian (`ppa:amnezia/ppa`)، مع بديل احتياطي للتوزيعات الأخرى. الشيء الوحيد الذي لا يمكنه فعله من أجلك: **تعطيل Secure Boot** على خادمك الافتراضي (VPS/VM) مسبقًا — وحدة نواة مبنية بواسطة DKMS غير موقّعة، ولن تحمّلها النواة طالما كان Secure Boot مفعّلاً.
- تتم المطابقة (reconcile) تمامًا كما تدير [`internal/mtproto`](internal/mtproto) الحاوية الجانبية `mtg`: تُبقي مهمة خلفية الواجهة قيد التشغيل متزامنة مع ما هو مخزَّن في قاعدة البيانات، وتطبّق تغييرات النظير عبر `awg syncconf` بدلاً من إعادة تشغيل الواجهة بالكامل كلما أمكن ذلك.

## تغييرات أخرى في هذه النسخة

تُضاف هنا التحسينات الأصغر الخاصة بهذه النسخة، بخلاف AmneziaWG، كلما تمت إضافتها:

- **الإكمال التلقائي لقواعد التوجيه (Routing)** — أصبحت حقول Domain/IP في محرر قواعد التوجيه الخاص بـ Xray تقترح فئات geosite/geoip (مثلاً كتابة "you" تقترح `geosite:youtube`) يتم إنشاؤها مباشرةً من ملفات `.dat` المثبَّتة فعليًا في مجلد bin الخاص بـ Xray — بما في ذلك الملفات المخصَّصة المُضافة عبر ميزة التحديث التلقائي لـ Geodata (مثل `geosite_roscom.dat`). لا يزال إدخال النص الحر يعمل تمامًا كما كان من قبل.

## الميزات

- **اتصالات واردة متعددة البروتوكولات** — VLESS، VMess، Trojan، Shadowsocks، WireGuard، **AmneziaWG**، Hysteria2، HTTP، SOCKS (Mixed)، Dokodemo-door / Tunnel و TUN.
- **وسائل نقل وأمان حديثة** — TCP (Raw)، mKCP، WebSocket، gRPC، HTTPUpgrade و XHTTP، مؤمَّنة بـ TLS و XTLS و REALITY.
- **Fallback** — تقديم عدة بروتوكولات على منفذ واحد (مثل VLESS و Trojan على المنفذ 443) باستخدام ميزة fallback في Xray.
- **إدارة لكل عميل** — حصص الترافيك، تواريخ انتهاء الصلاحية، حدود IP، حالة الاتصال المباشرة، وروابط مشاركة وأكواد QR واشتراكات بنقرة واحدة.
- **إحصائيات الترافيك** — لكل اتصال وارد، ولكل عميل، ولكل اتصال صادر، مع عناصر تحكم لإعادة التعيين.
- **دعم العقد المتعددة** — إدارة وتوسيع عبر عدة خوادم من لوحة واحدة.
- **الاتصالات الصادرة والتوجيه** — WARP، NordVPN، قواعد توجيه مخصصة، موازنات تحميل، وتسلسل الوكلاء الصادرة.
- **خادم اشتراك مدمج** بصيغ إخراج متعددة و[قوالب صفحات مخصصة](docs/custom-subscription-templates.md).
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
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash
```

لتثبيت إصدار محدد، أضِف وسمه (مثل `v3.5.0-awg.1`):

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s v3.5.0-awg.1
```

لتثبيت بنية **dev** المتجددة (أحدث إصدار أولي لكل التزام (commit) من `main`، وليس إصدارًا مستقرًا)، مرّر `dev`:

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

تُوسَم الإصدارات المستقرة الخاصة بهذه النسخة بـ `<إصدار الأساس في المشروع الأصلي>-awg.N` (مثل `v3.5.0-awg.1`، مبني فوق ما يسميه المشروع الأصلي `v3.5.0`) — أبدًا مجرد `vX.Y.Z` — حتى لا يُخلَط أبدًا بينها وبين إصدار حقيقي من MHSanaei/3x-ui بنفس الرقم.

أثناء التثبيت، يتم إنشاء اسم مستخدم وكلمة مرور ومسار وصول عشوائية. بعد التثبيت، شغّل `x-ui` لفتح قائمة الإدارة، حيث يمكنك بدء/إيقاف الخدمة، وعرض أو إعادة تعيين بيانات تسجيل الدخول، وإدارة شهادات SSL، والمزيد.

للحصول على وثائق اللوحة الكاملة إلى ما هو أبعد مما هو مذكور هنا، يرجى زيارة [ويكي المشروع الأصلي](https://github.com/MHSanaei/3x-ui/wiki) — لا شيء فيه خاص بهذه النسخة، لذا يبقى صالحًا بالكامل.

### التثبيت غير التفاعلي

يعمل المثبِّت أيضًا **بشكل غير تفاعلي** لـ cloud-init.
عيّن `XUI_NONINTERACTIVE=1` (أو مرّره عبر أنبوب دون TTY) وسيتولى التثبيت من البداية إلى النهاية
دون أي مطالبات، مُنشئًا بيانات اعتماد عشوائية وكاتبًا إياها في
`/etc/x-ui/install-result.env`. راجع [`deploy/`](deploy/) لـ:

- [بيانات مستخدم cloud-init](deploy/cloud-init/) — تثبيت غير تفاعلي على أي سحابة (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [ملاحظات Hetzner Cloud](deploy/marketplace/hetzner/) — نشر يعتمد على cloud-init على Hetzner

## المنصات المدعومة

**أنظمة التشغيل:** Ubuntu، Debian، Armbian، Fedora، CentOS، RHEL، AlmaLinux، Rocky Linux، Oracle Linux، Amazon Linux، Virtuozzo، Arch، Manjaro، Parch، openSUSE (Tumbleweed / Leap) و Alpine. (ينشر المشروع الأصلي أيضًا إصدارًا لـ Windows؛ لا تفعل CI هذه النسخة ذلك — كل شيء هنا موجَّه للخوادم/أجهزة التوجيه العاملة بلينكس، ويحتاج AmneziaWG على أي حال إلى وحدة نواة لينكس.)

**المعماريات:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

يحتاج AmneziaWG تحديدًا إلى نواة لينكس حقيقية مع وحدة نواة DKMS خاصة بـ AmneziaWG — لن يعمل على Windows، ويقوم `install_amneziawg` اليوم بأتمتة تثبيت وحدة النواة فقط على Ubuntu/Debian/Armbian (راجع قسم [ما الذي يختلف في هذه النسخة](#ما-الذي-يختلف-في-هذه-النسخة-amneziawg)).

## خيارات قاعدة البيانات

يدعم 3X-UI خلفيتين (backends) يتم اختيارهما أثناء التثبيت:

- **SQLite** (افتراضي) — ملف واحد في `/etc/x-ui/x-ui.db`. بدون إعداد، مثالي لعمليات النشر الصغيرة والمتوسطة.
- **PostgreSQL** — موصى به لأعداد العملاء الكبيرة أو الإعدادات متعددة العقد. يمكن للمثبِّت تثبيت PostgreSQL محليًا لك، أو قبول DSN لخادم موجود.

في وقت التشغيل، يتم اختيار الخلفية عبر متغيرات البيئة (يكتبها المثبِّت لك في `/etc/default/x-ui`):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### ترحيل تثبيت SQLite موجود إلى PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# ثم عيّن XUI_DB_TYPE و XUI_DB_DSN في /etc/default/x-ui وأعد التشغيل:
systemctl restart x-ui
```

يبقى ملف SQLite الأصلي دون تغيير؛ احذفه يدويًا بعد التحقق من الخلفية الجديدة.

## متغيرات البيئة

| المتغير | الوصف | الافتراضي |
| --- | --- | --- |
| `XUI_DB_TYPE` | خلفية قاعدة البيانات: `sqlite` أو `postgres` | `sqlite` |
| `XUI_DB_DSN` | سلسلة اتصال PostgreSQL (عندما `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | مجلد ملف قاعدة بيانات SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | الحد الأقصى للاتصالات المفتوحة (تجمّع PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | الحد الأقصى للاتصالات الخاملة (تجمّع PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | مسار URI الأولي للوحة الويب | `/` |
| `XUI_ENABLE_FAIL2BAN` | تفعيل فرض حدود IP المعتمد على Fail2ban | `true` |
| `XUI_LOG_LEVEL` | مستوى السجل (`debug`، `info`، `warning`، `error`) | `info` |
| `XUI_DEBUG` | تفعيل وضع التصحيح | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | تفعيل مراقب صحة النفق (يفحص عنوان URL ويعيد تشغيل xray بعد فشل متكرر؛ إعادة التشغيل تقطع جميع العملاء) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | الوكيل الذي يُرسَل عبره الفحص؛ وجّهه إلى اتصال xray وارد محلي ليختبر الفحص النفق (مثل `socks5://127.0.0.1:1080`). القيمة الفارغة تعني أن الفحص يتحقق فقط من اتصال المضيف | — |
| `XUI_TUNNEL_HEALTH_URL` | عنوان URL الذي يُفحَص لمعرفة صحة النفق | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | الفترة بين عمليات الفحص | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | مهلة كل عملية فحص | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | عدد حالات الفشل المتتالية قبل تشغيل إعادة التشغيل | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | الحد الأدنى للتأخير بين عمليات إعادة التشغيل المتتالية | `5m` |

## اللغات المدعومة

تتوفر واجهة اللوحة بـ 13 لغة:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## ملاحظات للمطورين

هذه نسخة محسّنة (fork) شخصية ولا تبحث عن مساهمين خارجيين، لكن [CONTRIBUTING.md](/CONTRIBUTING.md) لا يزال يحتوي على تعليمات دقيقة ومفيدة لإعداد بيئة تطوير محلية (إصدارات Go/Node، مُصرِّف C المطلوب لـ CGo، أوامر build/lint/test)، إذا كنت تعمل بنفسك على قاعدة الشيفرة هذه.

## الشكر

بُنيت هذه النسخة بالكامل فوق [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — اللوحة بأكملها ودعم البروتوكولات المتعددة والبنية الأساسية هي عملهم؛ **دعم AmneziaWG هو الشيء الوحيد المضاف هنا.** إذا وجدت المشروع الأصلي مفيدًا، فإن روابط دعم المؤلف الأصلي لا تزال المكان الصحيح لذلك:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

استندت عملية تنفيذ AmneziaWG النيتيف في هذه النسخة إلى/استُلهمت من:

- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — طلب السحب الأصلي لدعم AmneziaWG في المشروع الأصلي (نهج حاوية Docker الجانبية)؛ تعيد هذه النسخة استخدام بنية المخطط (schema) والواجهة الأمامية الخاصة به، لكنها تستبدل الواجهة الخلفية بمدير نيتيف بدون Docker.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — نسخة محسّنة مستقلة تُشغّل بالفعل AmneziaWG النيتيف في بيئة إنتاجية؛ استُمدت إدارة عملية `awg-quick`، وتوليد التكوين، ومولّد معاملات تمويه AmneziaWG 2.0 في هذه النسخة من حزمة `awg/` الخاصة بها.

## شكر خاص إلى

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (الترخيص: **GPL-3.0**): _قواعد توجيه v2ray/xray و v2ray/xray-clients المحسنة مع النطاقات الإيرانية المدمجة وتركيز على الأمان وحظر الإعلانات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (الترخيص: **GPL-3.0**): _يحتوي هذا المستودع على قواعد توجيه V2Ray محدثة تلقائيًا بناءً على بيانات النطاقات والعناوين المحظورة في روسيا._

## أدوات المجتمع

أدوات وتكاملات بناها المجتمع حول 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (الترخيص: **MIT**): _إدارة الاتصالات الواردة والعملاء وإعدادات اللوحة وتكوين Xray كرمز باستخدام Terraform / OpenTofu._
