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

**DUNE**, [3X-UI](https://github.com/MHSanaei/3x-ui)'nin hafif bir çatalıdır — [Xray-core](https://github.com/XTLS/Xray-core) sunucularını yönetmek için açık kaynaklı bir web kontrol paneli. 3X-UI'nin tanıdık iş akışlarını ve protokol desteğini korurken çok daha az CPU ve RAM kullanır; küçük VPS'ler ve düşük kaynaklı sunucular için idealdir.

3X-UI'den verimlilik odaklı ayrılan DUNE; arka plan işlerini azaltır, bellek kullanımını sıkılaştırır ve yığını sadeleştirir — panel sunucuyu yormadan akıcı kalır.

> [!IMPORTANT]
> Bu proje yalnızca kişisel kullanım için tasarlanmıştır. Lütfen yasadışı amaçlar için veya üretim (production) ortamında kullanmayın.

## Özellikler

- **Çoklu protokol destekli gelen bağlantılar (Inbounds)** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Karma), Dokodemo-door / Tunnel ve TUN.
- **Modern aktarımlar (transports) ve güvenlik** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade ve XHTTP; TLS, XTLS ve REALITY ile güvene alınmıştır.
- **Geri Dönüş (Fallbacks)** — Xray'in fallback desteğini kullanarak tek bir port üzerinde birden fazla protokole (ör. 443 üzerinde hem VLESS hem Trojan) hizmet verin.
- **Kullanıcı başına yönetim** — Trafik kotaları, bitiş tarihleri, IP sınırları, canlı çevrimiçi (online) durumu ve tek tıkla paylaşım bağlantıları, QR kodları ve abonelikler.
- **Trafik istatistikleri** — Gelen bağlantı (Inbound), istemci ve giden bağlantı (Outbound) bazında istatistikler ve sıfırlama kontrolleri.
- **Çoklu düğüm (Multi-node) desteği** — Tek bir panel üzerinden birden fazla sunucuyu yönetin ve ölçeklendirin.
- **Giden bağlantı (Outbound) ve yönlendirme** — WARP, NordVPN, özel yönlendirme kuralları, yük dengeleyiciler (load balancers) ve giden bağlantı proxy zincirleme (proxy chaining).
- **Dahili abonelik sunucusu** (Birden fazla çıktı formatı ile).
- Uzaktan izleme ve yönetim için **Telegram botu**.
- Panel içi Swagger dokümantasyonuna sahip **RESTful API**.
- **Esnek depolama** — SQLite (varsayılan) veya PostgreSQL.
- Koyu ve açık tema seçenekleriyle **13 farklı UI dili**.
- Kullanıcı başına IP limitlerini zorunlu kılmak için **Fail2ban entegrasyonu**.

## Ekran Görüntüleri

<details>
<summary>Genişletmek için tıklayın</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Genel Bakış" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Gelen Bağlantılar (Inbounds)" src="./media/02-add-inbound-light.png">
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
bash <(curl -Ls https://raw.githubusercontent.com/leto217/DUNE/main/install.sh)
```

Kurulum sırasında rastgele bir kullanıcı adı, şifre ve erişim yolu oluşturulur. Kurulumdan sonra, hizmeti başlatabileceğiniz/durdurabileceğiniz, giriş bilgilerinizi görüntüleyebileceğiniz veya sıfırlayabileceğiniz, SSL sertifikalarını yönetebileceğiniz ve çok daha fazlasını yapabileceğiniz yönetim menüsünü açmak için terminalde `dune` komutunu çalıştırın.

Tam dokümantasyon için lütfen [proje Wiki sayfasını](https://github.com/leto217/DUNE/wiki) ziyaret edin.

## Desteklenen Platformlar

**İşletim sistemleri:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine ve Windows.

**Mimariler:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Veritabanı Seçenekleri

Dune kurulum sırasında seçilebilecek iki arka uç (backend) destekler:

- **SQLite** (varsayılan) — `/etc/dune/dune.db` konumunda tek bir dosya. Kurulum gerektirmez, küçük ve orta ölçekli dağıtımlar için idealdir.
- **PostgreSQL** — Yüksek kullanıcı sayıları veya çoklu düğüm (multi-node) kurulumları için önerilir. Yükleyici sizin için yerel olarak PostgreSQL kurabilir veya mevcut bir sunucuya DSN bağlantısı kabul edebilir.

Çalışma anında veritabanı türü ortam değişkenleri (environment variables) ile seçilir (yükleyici bunları sizin için `/etc/default/dune` dosyasına yazar):

```
DUNE_DB_TYPE=postgres
DUNE_DB_DSN=postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable
```

### Mevcut bir SQLite Kurulumunu PostgreSQL'e Taşıma

```bash
dune migrate-db --dsn "postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable"
# ardından /etc/default/dune içindeki DUNE_DB_TYPE ve DUNE_DB_DSN değerlerini ayarlayıp yeniden başlatın:
systemctl restart dune
```

Kaynak SQLite dosyasına dokunulmaz; yeni veritabanının düzgün çalıştığını doğruladıktan sonra eski SQLite dosyasını manuel olarak silebilirsiniz.

### Docker

Varsayılan `docker compose up -d` komutu SQLite kullanmaya devam eder. Birlikte paketlenmiş PostgreSQL servisi ile çalıştırmak için, `docker-compose.yml` dosyasındaki iki `DUNE_DB_*` değişken satırının yorumunu kaldırın ve profille başlatın:

```bash
docker compose --profile postgres up -d
```

Docker imajı, kullanıcı başına **IP limitlerini** zorunlu kılmak için Fail2ban ile (varsayılan olarak etkindir) paketlenmiştir. Fail2ban, ihlalcileri `iptables` ile engeller ve bunun için `NET_ADMIN` yetkisine ihtiyaç duyar. `docker-compose.yml` bunu zaten `cap_add` üzerinden vermektedir; ancak konteyneri bunun yerine `docker run` ile başlatırsanız bu yetkileri kendiniz eklemelisiniz, aksi takdirde yasaklamalar günlüğe kaydedilir ancak uygulanmaz:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/leto217/DUNE
```

## Ortam Değişkenleri (Environment Variables)

| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `DUNE_DB_TYPE` | Veritabanı türü: `sqlite` veya `postgres` | `sqlite` |
| `DUNE_DB_DSN` | PostgreSQL bağlantı dizesi (eğer `DUNE_DB_TYPE=postgres` ise) | — |
| `DUNE_DB_FOLDER` | SQLite veritabanı dizini | `/etc/dune` |
| `DUNE_DB_MAX_OPEN_CONNS` | Maksimum açık bağlantı sayısı (PostgreSQL havuzu) | — |
| `DUNE_DB_MAX_IDLE_CONNS` | Maksimum boşta bekleme bağlantısı (PostgreSQL havuzu) | — |
| `DUNE_INIT_WEB_BASE_PATH` | Web paneli için başlangıç URI yolu | `/` |
| `DUNE_ENABLE_FAIL2BAN` | Fail2ban tabanlı IP limit uygulamasını etkinleştir | `true` |
| `DUNE_LOG_LEVEL` | Günlük (Log) ayrıntı seviyesi (`debug`, `info`, `warning`, `error`) | `info` |
| `DUNE_DEBUG` | Hata ayıklama (debug) modunu etkinleştir | `false` |

## Desteklenen Diller

Panel arayüzü 13 farklı dilde mevcuttur:

İngilizce · Farsça · Arapça · Çince (Basitleştirilmiş) · Çince (Geleneksel) · İspanyolca · Rusça · Ukraynaca · Türkçe · Vietnamca · Japonca · Endonezce · Portekizce (Brezilya)

## Katkıda Bulunma

Katkılarınızı her zaman bekliyoruz. Bir sorun (issue) açmadan veya pull request (PR) göndermeden önce lütfen [Katkıda Bulunma Kılavuzunu](/CONTRIBUTING.md) okuyun.

## Özel Teşekkürler

- [alireza0](https://github.com/alireza0/)

## Teşekkür & Atıf

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Lisans: **GPL-3.0**): _Geliştirilmiş v2ray/xray ve v2ray/xray-clients yönlendirme (routing) kuralları; yerleşik İran alan adları ile güvenlik ve reklam engelleme odaklıdır._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Lisans: **GPL-3.0**): _Bu depo, Rusya'daki engellenen alan adları ve adreslere dayalı otomatik olarak güncellenen V2Ray yönlendirme kurallarını içerir._

## Topluluk Araçları

dune çevresindeki topluluk tarafından oluşturulmuş araçlar ve entegrasyonlar.

- [terraform-provider-dune](https://github.com/batonogov/terraform-provider-threexui) (Lisans: **MIT**): _Gelen bağlantılarnı, kullanıcıları, panel ayarlarını ve Xray yapılandırmasını Terraform / OpenTofu ile kod olarak (as code) yönetin._

## Projeyi Destekleyin

**Eğer bu proje size faydalı olduysa, bir yıldız verebilirsiniz**:star2:

| Ağ | Adres |
| --- | --- |
| TON | `UQAa5FpNlK8Gp7tO8luJXHD-Sf0pPjJbNHGo8hdkyuUBhWEa` |
| TRON | `TLqtTfYSzPLFm8mtFDkSnXvzucxx7DS5VL` |
| ERC20 and BEP20 | `0x2fe632d70f4612b87670f8a28b4587ea2641452d` |

## Yıldız Tablosu

[![Zaman içerisindeki yıldız sayısı](https://starchart.cc/leto217/DUNE.svg?variant=adaptive)](https://starchart.cc/leto217/DUNE)
