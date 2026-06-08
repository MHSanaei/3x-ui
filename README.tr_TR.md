[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) | [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md) | [Türkçe](/README.tr_TR.md)

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
  <a href="https://goreportcard.com/report/github.com/mhsanaei/3x-ui/v3"><img src="https://goreportcard.com/badge/github.com/mhsanaei/3x-ui/v3" alt="Go Report Card"></a>
</p>

**3X-UI**, [Xray-core](https://github.com/XTLS/Xray-core) sunucularını yönetmek için geliştirilmiş, gelişmiş ve açık kaynaklı bir web kontrol panelidir. Tek bir VPS'den çok düğümlü (multi-node) kurulumlara kadar çok çeşitli proxy ve VPN protokollerini kurmak, yapılandırmak ve izlemek için temiz, çok dilli bir arayüz sağlar.

Orijinal X-UI projesinin geliştirilmiş bir çatalı (fork) olarak inşa edilen 3X-UI; daha geniş protokol desteği, iyileştirilmiş kararlılık, kullanıcı başına (per-client) trafik takibi ve birçok yaşam kalitesi (QoL) özelliği ekler.

> [!IMPORTANT]
> Bu proje yalnızca kişisel kullanım içindir. Lütfen yasa dışı amaçlarla veya üretim (production) ortamında kullanmayın.

## Özellikler

- **Çoklu protokol destekli bağlantı noktaları (Inbounds)** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel ve TUN.
- **Modern aktarım (Transport) & güvenlik** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade ve XHTTP; TLS, XTLS ve REALITY ile güvenli hale getirilmiştir.
- **Yedek bağlantılar (Fallbacks)** — Xray'in fallback desteğini kullanarak tek bir port (örn. 443) üzerinden birden fazla protokol (örn. VLESS ve Trojan) sunma.
- **Kullanıcı bazlı yönetim** — Trafik kotaları, son kullanma tarihleri, IP sınırları, canlı çevrimiçi (online) durumu ve tek tıklamayla paylaşım bağlantıları, QR kodları ve abonelikler.
- **Trafik istatistikleri** — Bağlantı noktası, kullanıcı ve çıkış noktası (outbound) bazında istatistikler ve sıfırlama kontrolleri.
- **Çoklu düğüm (Multi-node) desteği** — Tek bir panelden birden fazla sunucuyu yönetin ve ölçeklendirin.
- **Çıkış noktaları & yönlendirme (Outbound & Routing)** — WARP, NordVPN, özel yönlendirme kuralları, yük dengeleyiciler (load balancers) ve çıkış noktası proxy zincirleme.
- **Dahili abonelik sunucusu** (Birden fazla çıktı formatıyla).
- **Telegram botu** (Uzaktan izleme ve yönetim için).
- **RESTful API** (Panel içi Swagger dokümantasyonu ile).
- **Esnek veritabanı** — SQLite (varsayılan) veya PostgreSQL.
- **13 Kullanıcı Arayüzü (UI) dili** (Karanlık ve aydınlık tema destekli).
- **Fail2ban entegrasyonu** (Kullanıcı bazlı IP sınırlarını zorlamak için).

## Ekran Görüntüleri

<details>
<summary>Genişletmek için tıklayın</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Genel Bakış" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Bağlantı Noktaları" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Kullanıcı Ekle" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Yapılandırmalar" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Hızlı Başlangıç

```bash
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

Kurulum sırasında rastgele bir kullanıcı adı, şifre ve erişim yolu (path) oluşturulur. Kurulumdan sonra `x-ui` komutunu çalıştırarak yönetim menüsünü açabilir; buradan hizmeti başlatabilir/durdurabilir, giriş bilgilerinizi görüntüleyebilir veya sıfırlayabilir, SSL sertifikalarını yönetebilir ve daha fazlasını yapabilirsiniz.

Tam dokümantasyon için lütfen [proje Wiki sayfasını](https://github.com/MHSanaei/3x-ui/wiki) ziyaret edin.

## Desteklenen Platformlar

**İşletim sistemleri:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine ve Windows.

**Mimariler:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Veritabanı Seçenekleri

3X-UI, kurulum sırasında seçilebilen iki arka uç (backend) destekler:

- **SQLite** (varsayılan) — `/etc/x-ui/x-ui.db` konumunda tek bir dosya. Sıfır kurulum gerektirir, küçük ve orta ölçekli dağıtımlar için idealdir.
- **PostgreSQL** — Yüksek kullanıcı sayıları veya çok düğümlü (multi-node) kurulumlar için önerilir. Yükleyici sizin için PostgreSQL'i yerel olarak kurabilir veya mevcut bir sunucuya DSN ile bağlanabilir.

Çalışma anında arka uç, ortam değişkenleri (environment variables) aracılığıyla seçilir (yükleyici bunları sizin için `/etc/default/x-ui` dosyasına yazar):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Mevcut bir SQLite kurulumunu PostgreSQL'e taşıma

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# Ardından /etc/default/x-ui dosyasında XUI_DB_TYPE ve XUI_DB_DSN değerlerini ayarlayıp yeniden başlatın:
systemctl restart x-ui
```

Kaynak SQLite dosyasına dokunulmaz; yeni arka ucu doğruladıktan sonra eski dosyayı manuel olarak silebilirsiniz.

### Docker

Varsayılan `docker compose up -d` komutu SQLite kullanmaya devam eder. Dahili PostgreSQL hizmetiyle çalıştırmak için `docker-compose.yml` dosyasındaki iki `XUI_DB_*` ortam değişkeni satırının başındaki yorum işaretini kaldırın ve profille başlatın:

```bash
docker compose --profile postgres up -d
```

İmaj, kullanıcı bazlı **IP sınırlarını** zorlamak için Fail2ban'i (varsayılan olarak etkindir) içerir. Fail2ban, ihlalcileri `iptables` ile engeller ve bu işlem `NET_ADMIN` yetkisi gerektirir. `docker-compose.yml` bunu `cap_add` aracılığıyla zaten sağlar; eğer container'ı bunun yerine `docker run` ile başlatırsanız yetkileri kendiniz eklemelisiniz, aksi takdirde engellemeler sadece günlüğe (log) kaydedilir ancak asla uygulanmaz:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Ortam Değişkenleri

| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `XUI_DB_TYPE` | Veritabanı arka ucu: `sqlite` veya `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL bağlantı dizesi (`XUI_DB_TYPE=postgres` olduğunda) | — |
| `XUI_DB_FOLDER` | SQLite veritabanı dosyası için dizin | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maksimum açık bağlantı sayısı (PostgreSQL havuzu) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maksimum boşta bağlantı sayısı (PostgreSQL havuzu) | — |
| `XUI_ENABLE_FAIL2BAN` | Fail2ban tabanlı IP sınırı zorlamasını etkinleştir | `true` |
| `XUI_LOG_LEVEL` | Log detay seviyesi (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Hata ayıklama (debug) modunu etkinleştir | `false` |

## Desteklenen Diller

Panel kullanıcı arayüzü 13 dilde mevcuttur:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Katkıda Bulunma

Katkılara açığız. Lütfen bir sorun (issue) veya çekme isteği (pull request) açmadan önce [Katkıda Bulunma Rehberi](/CONTRIBUTING.md)'ni okuyun.

## Özel Teşekkürler

- [alireza0](https://github.com/alireza0/)

## Teşekkür

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Lisans: **GPL-3.0**): _Dahili İran alan adları ve güvenlik/reklam engelleme odaklı geliştirilmiş v2ray/xray ve v2ray/xray-clients yönlendirme kuralları._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Lisans: **GPL-3.0**): _Rusya'daki engellenmiş alan adları ve adreslere dayalı otomatik olarak güncellenen V2Ray yönlendirme kuralları içerir._

## Topluluk Araçları

3x-ui etrafında topluluk tarafından geliştirilen araçlar ve entegrasyonlar.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Lisans: **MIT**): _Bağlantı noktalarını, kullanıcıları, panel ayarlarını ve Xray yapılandırmasını Terraform / OpenTofu ile kod olarak yönetin._

## Projeyi Destekleyin

**Eğer bu proje sizin için faydalıysa, bir :star2: (yıldız) verebilirsiniz.**

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Zaman İçindeki Yıldız Sayısı

[![Zaman İçindeki Yıldız Sayısı](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui)
