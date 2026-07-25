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

**Это личный форк [3X-UI](https://github.com/MHSanaei/3x-ui)** — продвинутой веб-панели с открытым исходным кодом для управления серверами [Xray-core](https://github.com/XTLS/Xray-core) — с одним крупным дополнением: **нативной поддержкой AmneziaWG**, добавленной как полноценный протокол наравне с VLESS, VMess, Trojan и остальными. Всё остальное, что умеет 3X-UI (многопротокольные входящие, учёт трафика по клиентам, подписки, несколько узлов, Telegram-бот), не изменено и работает точно так же, как в оригинале.

Этот форк существует, чтобы обслуживать собственные роутеры и серверы автора; он не пытается заменить или конкурировать с оригинальным проектом. Если вам нужна универсальная панель — обращайтесь к [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui); всё, что написано ниже, документирует только отличия этого форка.

> [!IMPORTANT]
> Этот проект предназначен только для личного использования. Пожалуйста, не используйте его в незаконных целях или в производственной среде.

## Чем этот форк отличается: AmneziaWG

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) — это WireGuard с добавленным слоем обфускации (мусорные пакеты, случайный паддинг, переписывание магических заголовков), который призван обмануть DPI-фингерпринтинг протокола: тот же туннель, но не выглядящий как туннель в трафике.

- **Нативно, без Docker.** AmneziaWG работает как настоящий интерфейс ядра на хосте, поднимается и опускается через `awg-quick`/`awg` — тот же подход через DKMS-модуль ядра, что и у обычного интерфейса `wg0`. Никаких привилегированных сайдкар-контейнеров.
- **Полноценный протокол.** AmneziaWG-инбаунд живёт в той же таблице `Inbound`, что и всё остальное, поэтому бесплатно получает bulk-операции, модалку QR/скачивания конфига и ссылки подписки — учить ничего отдельного не нужно.
- **Полная обфускация AmneziaWG 2.0** — Jc/Jmin/Jmax (мусорные пакеты), S1–S4 (паддинг пакетов), H1–H4 (магические заголовки) и сигнатурный пакет I1, всё редактируется по каждому инбаунду с кнопкой генерации одним кликом, плюс совместимый с 1.x режим для старых клиентов.
- **Нативный IPv6** с NDP-прокси по каждому клиенту — у каждого пира напрямую доступный IPv6-адрес, без NAT66.
- **Проброс портов по клиентам** — DNAT конкретных портов/диапазонов прямо на туннельный адрес одного пира.
- **Маршрутизация трафика клиента через Xray** — каждый AmneziaWG-инбаунд автоматически получает свой loopback-мост в Xray (без переключателей), а куда направить трафик конкретного клиента — решается через уже существующее меню «Маршрутизация» панели, точно так же, как для любого другого протокола.
- **`install.sh` сам ставит модуль ядра** на Ubuntu/Debian/Armbian (`ppa:amnezia/ppa`), с fallback для других дистрибутивов. Одно он сделать не может: **выключить Secure Boot** на вашем VPS/VM заранее — DKMS-модуль не подписан, и ядро откажется его загружать, пока Secure Boot включён.
- Реконсиляция устроена так же, как [`internal/mtproto`](internal/mtproto) управляет своим сайдкаром `mtg`: фоновая джоба держит запущенный интерфейс синхронизированным с тем, что сохранено в базе, применяя изменения пиров через `awg syncconf` вместо полного рестарта интерфейса там, где это возможно.

## Возможности

- **Многопротокольные входящие подключения** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, **AmneziaWG**, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel и TUN.
- **Современные транспорты и безопасность** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade и XHTTP, защищённые с помощью TLS, XTLS и REALITY.
- **Fallback** — обслуживание нескольких протоколов на одном порту (например, VLESS и Trojan на 443) с помощью функции fallback в Xray.
- **Управление по каждому клиенту** — квоты трафика, даты истечения, лимиты IP, статус «онлайн» в реальном времени, а также ссылки для общего доступа, QR-коды и подписки в один клик.
- **Статистика трафика** — по каждому входящему, по каждому клиенту и по каждому исходящему, с возможностью сброса.
- **Поддержка нескольких узлов** — управление и масштабирование на несколько серверов из одной панели.
- **Исходящие подключения и маршрутизация** — WARP, NordVPN, пользовательские правила маршрутизации, балансировщики нагрузки и цепочки исходящих прокси.
- **Встроенный сервер подписок** с несколькими форматами вывода и [пользовательскими шаблонами страниц](docs/custom-subscription-templates.md).
- **Telegram-бот** для удалённого мониторинга и управления.
- **RESTful API** с документацией Swagger внутри панели.
- **Гибкое хранилище** — SQLite (по умолчанию) или PostgreSQL.
- **13 языков интерфейса** с тёмной и светлой темами.
- **Интеграция с Fail2ban** для применения лимитов IP по каждому клиенту.

## Скриншоты

<details>
<summary>Нажмите, чтобы развернуть</summary>

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

## Быстрый старт

```bash
curl -fsSL https://raw.githubusercontent.com/Kuzz007/3x-ui/main/install.sh | bash -s dev
```

Этот форк пока публикует только скользящий пре-релиз **`dev-latest`** (пересобирается автоматически при каждом push в `main`) — стабильного релиза с тегом ещё нет, поэтому `dev` — единственный канал, который сейчас на что-то указывает.

Во время установки генерируются случайные имя пользователя, пароль и путь доступа. После установки выполните `x-ui`, чтобы открыть меню управления, где можно запускать/останавливать сервис, просматривать или сбрасывать учётные данные для входа, управлять SSL-сертификатами и многое другое.

Общую документацию по панели, помимо этого README, смотрите в оригинальной [вики проекта](https://github.com/MHSanaei/3x-ui/wiki) — там ничего специфичного для этого форка, так что всё актуально.

### Автоматическая установка

Установщик также работает в **неинтерактивном** режиме для cloud-init.
Задайте `XUI_NONINTERACTIVE=1` (или передайте по конвейеру без TTY), и установка пройдёт от начала до конца
без единого запроса: будут сгенерированы случайные учётные данные и записаны в
`/etc/x-ui/install-result.env`. Смотрите [`deploy/`](deploy/) для:

- [Cloud-init user-data](deploy/cloud-init/) — автоматическая установка в любом облаке (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Заметки по Hetzner Cloud](deploy/marketplace/hetzner/) — развёртывание на Hetzner на базе cloud-init

## Поддерживаемые платформы

**Операционные системы:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap) и Alpine. (В апстриме также есть сборка под Windows; в CI этого форка её нет — здесь всё нацелено на Linux-серверы/роутеры, да и AmneziaWG в любом случае требует модуль ядра Linux.)

**Архитектуры:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

AmneziaWG отдельно требует настоящее ядро Linux с DKMS-модулем AmneziaWG — на Windows он не поднимется, а `install_amneziawg` пока автоматизирует установку модуля только на Ubuntu/Debian/Armbian (см. [Чем этот форк отличается: AmneziaWG](#чем-этот-форк-отличается-amneziawg)).

## Варианты базы данных

3X-UI поддерживает два бэкенда, выбираемых при установке:

- **SQLite** (по умолчанию) — единый файл по пути `/etc/x-ui/x-ui.db`. Без настройки, идеально для небольших и средних развёртываний.
- **PostgreSQL** — рекомендуется при большом числе клиентов или конфигурациях с несколькими узлами. Установщик может установить PostgreSQL локально за вас или принять DSN к существующему серверу.

Во время выполнения бэкенд выбирается через переменные окружения (установщик записывает их за вас в `/etc/default/x-ui`):

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Перенос существующей установки SQLite в PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
# затем задайте XUI_DB_TYPE и XUI_DB_DSN в /etc/default/x-ui и перезапустите:
systemctl restart x-ui
```

Исходный файл SQLite остаётся нетронутым; удалите его вручную после проверки нового бэкенда.

### Docker

Команда по умолчанию `docker compose up -d` продолжает использовать SQLite. Чтобы запустить со встроенным сервисом PostgreSQL, раскомментируйте две строки переменных окружения `XUI_DB_*` в `docker-compose.yml` и запустите с профилем:

```bash
docker compose --profile postgres up -d
```

> [!NOTE]
> AmneziaWG-инбаундам нужны `awg-quick`/`awg` и модуль ядра AmneziaWG на **хосте** — в этом весь смысл отказа от Docker, описанного в разделе [Чем этот форк отличается: AmneziaWG](#чем-этот-форк-отличается-amneziawg). Сама панель в Docker по-прежнему прекрасно работает для всех остальных протоколов, но AmneziaWG-инбаунд, созданный из контейнеризированной панели, поднять интерфейс негде, если только у контейнера нет доступа к сети/ядру хоста — а это уже сводит на нет весь смысл. Если планируете использовать AmneziaWG, запускайте панель нативно на хосте.

Образ включает Fail2ban (включён по умолчанию) для применения **лимитов IP** по каждому клиенту. Fail2ban блокирует нарушителей с помощью `iptables`, что требует возможности `NET_ADMIN`. `docker-compose.yml` уже предоставляет её через `cap_add`; если вы вместо этого запускаете контейнер через `docker run`, добавьте возможности самостоятельно, иначе блокировки будут регистрироваться, но никогда не применяться:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/mhsanaei/3x-ui
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд базы данных: `sqlite` или `postgres` | `sqlite` |
| `XUI_DB_DSN` | Строка подключения PostgreSQL (когда `XUI_DB_TYPE=postgres`) | — |
| `XUI_DB_FOLDER` | Каталог для файла базы данных SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Максимум открытых соединений (пул PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Максимум простаивающих соединений (пул PostgreSQL) | — |
| `XUI_INIT_WEB_BASE_PATH` | Начальный URI-путь для веб-панели | `/` |
| `XUI_ENABLE_FAIL2BAN` | Включить применение лимитов IP на основе Fail2ban | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `warning`, `error`) | `info` |
| `XUI_DEBUG` | Включить режим отладки | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Включить монитор состояния туннеля (опрашивает URL и перезапускает xray после многократных сбоев; перезапуск отключает всех клиентов) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Прокси, через который отправляется проба; укажите локальный входящий xray, чтобы проба проверяла туннель (например, `socks5://127.0.0.1:1080`). Пустое значение означает, что проба проверяет только связь с хостом | — |
| `XUI_TUNNEL_HEALTH_URL` | URL, опрашиваемый для проверки состояния туннеля | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Интервал между пробами | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Таймаут на одну пробу | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Число последовательных сбоев до запуска перезапуска | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Минимальная задержка между последовательными перезапусками | `5m` |

## Поддерживаемые языки

Интерфейс панели доступен на 13 языках:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Заметки для разработчика

Это личный форк, и он не ищет сторонних контрибьюторов, но в [CONTRIBUTING.md](/CONTRIBUTING.md) по-прежнему актуальные и полезные инструкции по локальной настройке разработки (версии Go/Node, C-компилятор для CGo, команды сборки/линта/тестов) — пригодится, если сами будете работать с этим кодом.

## Благодарность

Этот форк полностью построен поверх [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — вся панель, поддержка множества протоколов и базовая архитектура — их работа; **единственное, что добавлено здесь — поддержка AmneziaWG.** Если оригинальный проект оказался вам полезен, ссылки на поддержку автора всё ещё актуальны:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

Нативная реализация AmneziaWG в этом форке портирована/вдохновлена:

- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — оригинальный PR с AmneziaWG в апстрим (подход через Docker-сайдкар); этот форк переиспользует его фронтенд-схему/структуру UI, но заменяет бэкенд на нативный менеджер без Docker.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — независимый форк, уже использующий нативный AmneziaWG в проде; управление процессом `awg-quick`, генерация конфига и генератор параметров обфускации AmneziaWG 2.0 в этом форке портированы из его пакета `awg/`.

## Благодарности

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Лицензия: **GPL-3.0**): _Улучшенные правила маршрутизации для v2ray/xray и v2ray/xray-clients со встроенными иранскими доменами и фокусом на безопасность и блокировку рекламы._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Лицензия: **GPL-3.0**): _Этот репозиторий содержит автоматически обновляемые правила маршрутизации V2Ray на основе данных о заблокированных доменах и адресах в России._

## Инструменты сообщества

Инструменты и интеграции, созданные сообществом вокруг 3x-ui.

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (Лицензия: **MIT**): _Управление входящими, клиентами, настройками панели и конфигурацией Xray через код с помощью Terraform / OpenTofu._
