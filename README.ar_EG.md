[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/mhsanaei/3x-ui.svg)](https://github.com/MHSanaei/3x-ui/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/mhsanaei/3x-ui/release.yml.svg)](https://github.com/MHSanaei/3x-ui/actions)
[![GO Version](https://img.shields.io/github/go-mod/go-version/mhsanaei/3x-ui.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/mhsanaei/3x-ui/total.svg)](https://github.com/MHSanaei/3x-ui/releases/latest)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)
[![Go Reference](https://pkg.go.dev/badge/github.com/mhsanaei/3x-ui/v2.svg)](https://pkg.go.dev/github.com/mhsanaei/3x-ui/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/mhsanaei/3x-ui/v2)](https://goreportcard.com/report/github.com/mhsanaei/3x-ui/v2)

**3X-UI** — لوحة تحكم متقدمة مفتوحة المصدر تعتمد على الويب مصممة لإدارة خادم Xray-core. توفر واجهة سهلة الاستخدام لتكوين ومراقبة بروتوكولات VPN والوكيل المختلفة.

> [!IMPORTANT]
> هذا المشروع مخصص للاستخدام الشخصي والاتصال فقط، يرجى عدم استخدامه لأغراض غير قانونية، يرجى عدم استخدامه في بيئة الإنتاج.

كمشروع محسن من مشروع X-UI الأصلي، يوفر 3X-UI استقرارًا محسنًا ودعمًا أوسع للبروتوكولات وميزات إضافية.

## مصادر DAT مخصصة GeoSite / GeoIP

يمكن للمسؤولين إضافة ملفات `.dat` لـ GeoSite وGeoIP من عناوين URL في اللوحة (نفس أسلوب تحديث ملفات الجيو المدمجة). تُحفظ الملفات بجانب ثنائي Xray (`XUI_BIN_FOLDER`، الافتراضي `bin/`) بأسماء ثابتة: `geosite_&lt;alias&gt;.dat` و`geoip_&lt;alias&gt;.dat`.

**التوجيه:** استخدم الصيغة `ext:`، مثل `ext:geosite_myalias.dat:tag` أو `ext:geoip_myalias.dat:tag`، حيث `tag` اسم قائمة داخل ملف DAT (كما في `ext:geoip_IR.dat:ir`).

**الأسماء المحجوزة:** يُقارَن شكل مُطبَّع فقط لمعرفة التحفظ (`strings.ToLower`، `-` → `_`). لا تُعاد كتابة الأسماء التي يدخلها المستخدم أو سجلات قاعدة البيانات؛ يجب أن تطابق `^[a-z0-9_-]+$`. مثلاً `geoip-ir` و`geoip_ir` يصطدمان بنفس الحجز.

## البدء السريع

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

للحصول على الوثائق الكاملة، يرجى زيارة [ويكي المشروع](https://github.com/MHSanaei/3x-ui/wiki).

## شكر خاص إلى

- [alireza0](https://github.com/alireza0/)

## الاعتراف

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (الترخيص: **GPL-3.0**): _قواعد توجيه v2ray/xray و v2ray/xray-clients المحسنة مع النطاقات الإيرانية المدمجة وتركيز على الأمان وحظر الإعلانات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (الترخيص: **GPL-3.0**): _يحتوي هذا المستودع على قواعد توجيه V2Ray محدثة تلقائيًا بناءً على بيانات النطاقات والعناوين المحظورة في روسيا._

## دعم المشروع

**إذا كان هذا المشروع مفيدًا لك، فقد ترغب في إعطائه**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>
</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## النجوم عبر الزمن

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui) 
