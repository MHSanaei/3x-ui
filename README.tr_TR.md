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

**Bu, [3X-UI](https://github.com/MHSanaei/3x-ui)'nin kişisel bir çatallamasıdır (fork)** — [Xray-core](https://github.com/XTLS/Xray-core) için geliştirilmiş, açık kaynaklı bir web kontrol panelidir — ve tek bir büyük ekleme içerir: VLESS, VMess, Trojan ve diğerleriyle aynı seviyede birinci sınıf bir protokol olarak **yerel (native) AmneziaWG desteği**. 3X-UI'nin zaten yaptığı her şey (çoklu protokol destekli gelen bağlantılar, kullanıcı başına trafik hesaplama, abonelikler, çoklu düğüm, Telegram botu) değişmeden kalır ve orijinal projedeki gibi çalışmaya devam eder.

Bu fork, yazarının kendi yönlendiricileri ve kişisel sunucuları üzerinde çalışması için oluşturulmuştur; orijinal projenin yerini almayı veya onunla rekabet etmeyi amaçlamaz. Genel amaçlı paneli arıyorsanız [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) adresine gidin — aşağıdaki her şey yalnızca bu fork'un farklarını belgelemektedir.

> [!IMPORTANT]
> Bu proje yalnızca kişisel kullanım için tasarlanmıştır. Lütfen yasadışı amaçlar için veya üretim (production) ortamında kullanmayın.

## Bu fork'ta ne farklı: AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module), DPI tabanlı protokol parmak izi çıkarmayı yenmek için tasarlanmış bir gizleme (obfuscation) katmanı (çöp paketler, rastgele dolgu, sihirli başlıkların yeniden yazılması) ekleyen bir WireGuard varyantıdır — aynı tünel, ama artık hat üzerinde bir tünel gibi görünmeyen bir tünel.

- **Yerel (native), Docker değil.** AmneziaWG, host üzerinde gerçek bir çekirdek arabirimi olarak çalışır; `awg-quick`/`awg` ile açılıp kapatılır — size yerel bir `wg0` arabirimi kazandıran aynı DKMS çekirdek modülü yaklaşımı. Ayrıcalıklı bir sidecar konteynerine gerek yoktur.
- **Birinci sınıf bir protokol.** Bir AmneziaWG gelen bağlantısı, diğerleriyle aynı `Inbound` tablosunda yaşar; bu sayede toplu işlemleri (bulk operations), QR kodu/yapılandırma indirme modalını ve abonelik bağlantılarını hiçbir ek çaba olmadan kazanır — öğrenilecek yeni bir şey yoktur.
- **Tam AmneziaWG 2.0 gizlemesi** — Jc/Jmin/Jmax (çöp paketler), S1–S4 (paket dolgusu), H1–H4 (sihirli başlıklar) ve I1 imza paketi; her biri gelen bağlantı başına düzenlenebilir ve tek tıkla rastgeleleştirme düğmesine sahiptir, ayrıca eski istemciler için 1.x uyumlu bir mod da bulunur.
- **Yerel IPv6**, kullanıcı başına NDP proxy desteğiyle; böylece her eş (peer) doğrudan erişilebilir bir IPv6 adresi alır — NAT66 gerekmez.
- **Kullanıcı başına port yönlendirme** — belirli portları/aralıkları doğrudan bir eşin tünel adresine DNAT edin.
- **Bir istemcinin trafiğini Xray üzerinden yönlendirme** — her AmneziaWG gelen bağlantısı otomatik olarak kendi loopback Xray köprüsünü alır (hiçbir anahtar/switch olmadan); herhangi bir istemcinin trafiğini, panelde zaten mevcut olan "Yönlendirme" sayfası üzerinden yapılandırılmış herhangi bir Xray giden bağlantısına, tıpkı başka herhangi bir protokolü yönlendirir gibi yönlendirin.
- **`install.sh` çekirdek modülünü sizin için kurar** — Ubuntu/Debian/Armbian üzerinde (`ppa:amnezia/ppa`), diğer dağıtımlar için bir yedek (fallback) ile birlikte. Sizin için yapamayacağı tek şey: VPS/VM'inizde **Secure Boot'u önceden devre dışı bırakmak** — DKMS ile derlenmiş bir modül imzasızdır ve Secure Boot etkin olduğu sürece çekirdek onu yüklemeyi reddeder.
- Uzlaştırma (reconcile), [`internal/mtproto`](internal/mtproto)'nun `mtg` sidecar'ını yönetme biçimiyle tamamen aynı şekilde yapılır: arka planda çalışan bir görev, çalışan arabirimi veritabanında saklanan durumla senkronize tutar ve mümkün olduğunda eş (peer) değişikliklerini tam bir arabirim yeniden başlatması yerine `awg syncconf` üzerinden uygular.

## Özellikler

- **Çoklu protokol destekli gelen bağlantılar (Inbounds)** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, **AmneziaWG**, Hysteria2, HTTP, SOCKS (Karma), Dokodemo-door / Tunnel ve TUN.
- **Modern aktarımlar (transports) ve güvenlik** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade ve XHTTP; TLS, XTLS ve REALITY ile güvene alınmıştır.
- **Geri Dönüş (Fallbacks)** — Xray'in fallback desteğini kullanarak tek bir port üzerinde birden fazla protokole (ör. 443 üzerinde hem VLESS hem Trojan) hizmet verin.
- **Kullanıcı başına yönetim** — Trafik kotaları, bitiş tarihleri, IP sınırları, canlı çevrimiçi (online) durumu ve tek tıkla paylaşım bağlantıları, QR kodları ve abonelikler.
- **Trafik istatistikleri** — Gelen bağlantı (Inbound), istemci ve giden bağlantı (Outbound) bazında istatistikler ve sıfırlama kontrolleri.
- **Çoklu düğüm (Multi-node) desteği** — Tek bir panel üzerinden birden fazla sunucuyu yönetin ve ölçeklendirin.
- **Giden bağlantı (Outbound) ve yönlendirme** — WARP, NordVPN, özel yönlendirme kuralları, yük dengeleyiciler (load balancers) ve giden bağlantı proxy zincirleme (proxy chaining).
- **Dahili abonelik sunucusu** (Birden fazla çıktı formatı ve [özel sayfa şablonları](docs/custom-subscription-templates.md) ile).
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
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

Bu fork yalnızca sürekli güncellenen **`dev-latest`** ön sürümünü yayınlar (her `main` push'unda otomatik olarak yeniden derlenir) — henüz etiketlenmiş kararlı bir sürüm yoktur, bu yüzden şu anda gerçekten bir şeye karşılık gelen tek kanal `dev`'dir.

Kurulum sırasında rastgele bir kullanıcı adı, şifre ve erişim yolu oluşturulur. Kurulumdan sonra, hizmeti başlatabileceğiniz/durdurabileceğiniz, giriş bilgilerinizi görüntüleyebileceğiniz veya sıfırlayabileceğiniz, SSL sertifikalarını yönetebileceğiniz ve çok daha fazlasını yapabileceğiniz yönetim menüsünü açmak için terminalde `x-ui` komutunu çalıştırın.

Bu README'nin kapsadığından daha fazlası için tam panel dokümantasyonuna [orijinal projenin Wiki sayfasından](https://github.com/MHSanaei/3x-ui/wiki) ulaşabilirsiniz — hiçbiri bu fork'a özgü değildir, dolayısıyla tamamen geçerliliğini korur.

### Etkileşimsiz kurulum

Yükleyici, cloud-init için **etkileşimsiz** olarak da çalışır.
`XUI_NONINTERACTIVE=1` ayarlayın (veya TTY olmadan boru hattına aktarın); kurulum baştan
sona hiçbir soru sormadan tamamlanır, rastgele kimlik bilgileri oluşturup bunları
`/etc/x-ui/install-result.env` dosyasına yazar. Şunlar için [`deploy/`](deploy/) klasörüne bakın:

- [Cloud-init user-data](deploy/cloud-init/) — herhangi bir bulutta etkileşimsiz kurulum (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Hetzner Cloud notları](deploy/marketplace/hetzner/) — Hetzner üzerinde cloud-init tabanlı dağıtım

## Desteklenen Platformlar

**İşletim sistemleri:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap) ve Alpine. (Orijinal proje ayrıca bir Windows sürümü de yayınlar; bu fork'un CI'ı bunu yapmaz — buradaki her şey Linux çalıştıran sunucuları/yönlendiricileri hedefler ve AmneziaWG zaten her durumda bir Linux çekirdek modülüne ihtiyaç duyar.)

**Mimariler:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

AmneziaWG özellikle gerçek bir Linux çekirdeği ve AmneziaWG'ye özgü DKMS çekirdek modülüne ihtiyaç duyar — Windows üzerinde çalışmaz ve `install_amneziawg` bugün yalnızca Ubuntu/Debian/Armbian üzerinde çekirdek modülü kurulumunu otomatikleştirir ([Bu fork'ta ne farklı](#bu-forkta-ne-farklı-amneziawg) bölümüne bakın).

## Veritabanı Seçenekleri

3X-UI kurulum sırasında seçilebilecek iki arka uç (backend) destekler:

- **SQLite** (varsayılan) — `/etc/x-ui/x-ui.db` konumunda tek bir dosya. Kurulum gerektirmez, küçük ve orta ölçekli dağıtımlar için idealdir.
- **PostgreSQL** — Yüksek kullanıcı sayıları veya çoklu düğüm (multi-node) kurulumları için önerilir. Yükleyici sizin için yerel olarak PostgreSQL kurabilir veya mevcut bir sunucuya DSN bağlantısı kabul edebilir.

Çalışma anında veritabanı türü ortam değişkenleri (environment variables) ile seçilir (yükleyici bunları sizin için `/etc/default/x-ui` dosyasına yazar):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Mevcut bir SQLite Kurulumunu PostgreSQL'e Taşıma

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# ardından /etc/default/x-ui içindeki XUI_DB_TYPE ve XUI_DB_DSN değerlerini ayarlayıp yeniden başlatın:
systemctl restart x-ui
```

Kaynak SQLite dosyasına dokunulmaz; yeni veritabanının düzgün çalıştığını doğruladıktan sonra eski SQLite dosyasını manuel olarak silebilirsiniz.

### Docker

Varsayılan `docker compose up -d` komutu SQLite kullanmaya devam eder. Birlikte paketlenmiş PostgreSQL servisi ile çalıştırmak için, `docker-compose.yml` dosyasındaki iki `XUI_DB_*` değişken satırının yorumunu kaldırın ve profille başlatın:

```bash
docker compose --profile postgres up -d
```

> [!NOTE]
> AmneziaWG gelen bağlantıları, **host** üzerinde `awg-quick`/`awg` ve AmneziaWG çekirdek moduline ihtiyaç duyar — bu tam olarak [Bu fork'ta ne farklı](#bu-forkta-ne-farklı-amneziawg) bölümünde açıklanan Docker'sız tasarım amacının yansımasıdır. Panelin kendisini Docker içinde çalıştırmak diğer tüm protokoller için işlemeye devam eder, ancak konteynerize edilmiş bir panelden oluşturulan bir AmneziaWG gelen bağlantısının, konteyner host düzeyinde ağ/çekirdek erişimi almadıkça arabirimini açacak bir yeri yoktur ki bu da temel amacı geçersiz kılar. AmneziaWG kullanmayı planlıyorsanız, host üzerinde yerel (native) olarak çalıştırın.

Docker imajı, kullanıcı başına **IP limitlerini** zorunlu kılmak için Fail2ban ile (varsayılan olarak etkindir) paketlenmiştir. Fail2ban, ihlalcileri `iptables` ile engeller ve bunun için `NET_ADMIN` yetkisine ihtiyaç duyar. `docker-compose.yml` bunu zaten `cap_add` üzerinden vermektedir; ancak konteyneri bunun yerine `docker run` ile başlatırsanız bu yetkileri kendiniz eklemelisiniz, aksi takdirde yasaklamalar günlüğe kaydedilir ancak uygulanmaz:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Ortam Değişkenleri (Environment Variables)

| Değişken | Açıklama | Varsayılan |
| --- | --- | --- |
| `XUI_DB_TYPE` | Veritabanı türü: `sqlite` veya `postgres` | `sqlite` |
| `XUI_DB_DSN` | PostgreSQL bağlantı dizesi (eğer `XUI_DB_TYPE=postgres` ise) | — |
| `XUI_DB_FOLDER` | SQLite veritabanı dizini | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Maksimum açık bağlantı sayısı (PostgreSQL havuzu) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Maksimum boşta bekleme bağlantısı (PostgreSQL havuzu) | — |
| `XUI_INIT_WEB_BASE_PATH` | Web paneli için başlangıç URI yolu | `/` |
| `XUI_ENABLE_FAIL2BAN` | Fail2ban tabanlı IP limit uygulamasını etkinleştir | `true` |
| `XUI_LOG_LEVEL` | Günlük (Log) ayrıntı seviyesi (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Hata ayıklama (debug) modunu etkinleştir | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Tünel sağlık izleyicisini etkinleştir (bir URL'yi yoklar ve tekrarlanan başarısızlıklardan sonra xray'i yeniden başlatır; yeniden başlatma tüm istemcilerin bağlantısını düşürür) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Yoklamanın gönderildiği proxy; yoklamanın tüneli test etmesi için bunu yerel bir xray gelen bağlantısına yönlendirin (ör. `socks5://127.0.0.1:1080`). Boş bırakılırsa yoklama yalnızca ana makine bağlantısını kontrol eder | — |
| `XUI_TUNNEL_HEALTH_URL` | Tünel sağlığı için yoklanan URL | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Yoklamalar arasındaki aralık | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Yoklama başına zaman aşımı | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Yeniden başlatma tetiklenmeden önceki ardışık başarısızlık sayısı | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Ardışık yeniden başlatmalar arasındaki minimum gecikme | `5m` |

## Desteklenen Diller

Panel arayüzü 13 farklı dilde mevcuttur:

İngilizce · Farsça · Arapça · Çince (Basitleştirilmiş) · Çince (Geleneksel) · İspanyolca · Rusça · Ukraynaca · Türkçe · Vietnamca · Japonca · Endonezce · Portekizce (Brezilya)

## Geliştirici Notları

Bu kişisel bir fork'tur ve dış katkıda bulunanlar aramamaktadır; ancak siz kendiniz bu kod tabanı üzerinde çalışıyorsanız [CONTRIBUTING.md](/CONTRIBUTING.md) hâlâ yerel bir geliştirme ortamı kurmak için (Go/Node sürümleri, CGo için gereken C derleyicisi, build/lint/test komutları) ayrıntılı ve yararlı talimatlar içermektedir.

## Atıf

Bu fork tamamen [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) üzerine inşa edilmiştir — panelin tamamı, çoklu protokol desteği ve altta yatan mimari onların eseridir; **AmneziaWG desteği burada eklenen tek şeydir.** Orijinal projeyi faydalı bulduysanız, orijinal yazarın destek bağlantıları bunun için hâlâ doğru yerdir:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Bana Bir Kahve Ismarla" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="NOWPayments üzerinden Kripto Bağış Butonu">
</a>

Bu fork'taki yerel AmneziaWG uygulaması şunlardan alınmış/ilham almıştır:

- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — orijinal projeye karşı açılan orijinal AmneziaWG pull request'i (Docker sidecar yaklaşımı); bu fork onun şema/ön yüz (frontend) yapısını yeniden kullanır ancak arka ucu (backend) yerel, Docker'sız bir yöneticiyle değiştirir.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — üretimde zaten yerel AmneziaWG çalıştıran bağımsız bir fork; bu fork'un `awg-quick` süreç yönetimi, yapılandırma üretimi ve AmneziaWG 2.0 gizleme parametresi üreticisi onun `awg/` paketinden alınmıştır.

## Özel Teşekkürler

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Lisans: **GPL-3.0**): _Geliştirilmiş v2ray/xray ve v2ray/xray-clients yönlendirme (routing) kuralları; yerleşik İran alan adları ile güvenlik ve reklam engelleme odaklıdır._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Lisans: **GPL-3.0**): _Bu depo, Rusya'daki engellenen alan adları ve adreslere dayalı otomatik olarak güncellenen V2Ray yönlendirme kurallarını içerir._

## Topluluk Araçları

3x-ui çevresindeki topluluk tarafından oluşturulmuş araçlar ve entegrasyonlar.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Lisans: **MIT**): _Gelen bağlantılarnı, kullanıcıları, panel ayarlarını ve Xray yapılandırmasını Terraform / OpenTofu ile kod olarak (as code) yönetin._
