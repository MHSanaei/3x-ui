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

**3X-UI** — یک پنل کنترل پیشرفته مبتنی بر وب با کد باز که برای مدیریت سرور Xray-core طراحی شده است. این پنل یک رابط کاربری آسان برای پیکربندی و نظارت بر پروتکل‌های مختلف VPN و پراکسی ارائه می‌دهد.

> [!IMPORTANT]
> این پروژه فقط برای استفاده شخصی و ارتباطات است، لطفاً از آن برای اهداف غیرقانونی استفاده نکنید، لطفاً از آن در محیط تولید استفاده نکنید.

به عنوان یک نسخه بهبود یافته از پروژه اصلی X-UI، 3X-UI پایداری بهتر، پشتیبانی گسترده‌تر از پروتکل‌ها و ویژگی‌های اضافی را ارائه می‌دهد.

## منابع DAT سفارشی GeoSite / GeoIP

سرپرستان می‌توانند از طریق پنل فایل‌های `.dat` GeoSite و GeoIP را از URL اضافه کنند (همان الگوی به‌روزرسانی ژئوفایل‌های داخلی). فایل‌ها در کنار باینری Xray (`XUI_BIN_FOLDER`، پیش‌فرض `bin/`) با نام‌های ثابت `geosite_&lt;alias&gt;.dat` و `geoip_&lt;alias&gt;.dat` ذخیره می‌شوند.

**مسیریابی:** از شکل `ext:` استفاده کنید، مثلاً `ext:geosite_myalias.dat:tag` یا `ext:geoip_myalias.dat:tag`؛ `tag` نام لیست داخل همان DAT است (مانند `ext:geoip_IR.dat:ir`).

**نام‌های رزرو:** فقط برای تشخیص رزرو بودن، نسخه نرمال‌شده (`strings.ToLower`، `-` → `_`) مقایسه می‌شود. نام‌های واردشده و رکورد پایگاه داده بازنویسی نمی‌شوند و باید با `^[a-z0-9_-]+$` سازگار باشند؛ مثلاً `geoip-ir` و `geoip_ir` به یک رزرو یکسان می‌خورند.

## شروع سریع

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

برای مستندات کامل، لطفاً به [ویکی پروژه](https://github.com/MHSanaei/3x-ui/wiki) مراجعه کنید.

## تشکر ویژه از

- [alireza0](https://github.com/alireza0/)

## قدردانی

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (مجوز: **GPL-3.0**): _قوانین مسیریابی بهبود یافته v2ray/xray و v2ray/xray-clients با دامنه‌های ایرانی داخلی و تمرکز بر امنیت و مسدود کردن تبلیغات._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (مجوز: **GPL-3.0**): _این مخزن شامل قوانین مسیریابی V2Ray به‌روزرسانی شده خودکار بر اساس داده‌های دامنه‌ها و آدرس‌های مسدود شده در روسیه است._

## پشتیبانی از پروژه

**اگر این پروژه برای شما مفید است، می‌توانید به آن یک**:star2: بدهید

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## ستاره‌ها در طول زمان

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui) 
