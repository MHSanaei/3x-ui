[Русский](/README.md) | [English](/README.en_US.md)

<p align="center">
  <img alt="3x-ui-awg" src="./media/3x-ui-awg-logo.png">
</p>

<p align="center">
  <a href="https://github.com/kuzzrus/3x-ui-awg/actions"><img src="https://img.shields.io/github/actions/workflow/status/kuzzrus/3x-ui-awg/release.yml.svg" alt="Build"></a>
  <a href="#"><img src="https://img.shields.io/github/go-mod/go-version/kuzzrus/3x-ui-awg.svg" alt="GO Version"></a>
  <a href="https://www.gnu.org/licenses/gpl-3.0.en.html"><img src="https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true" alt="License"></a>
</p>

<p align="center">
  <a href="https://www.tbank.ru/cf/2qxNvGa3fSX"><img src="https://img.shields.io/badge/%E2%9D%A4%EF%B8%8F_%D0%9F%D0%BE%D0%B4%D0%B4%D0%B5%D1%80%D0%B6%D0%B0%D1%82%D1%8C_%D1%84%D0%BE%D1%80%D0%BA-Donate_%D1%87%D0%B5%D1%80%D0%B5%D0%B7_T--Bank-FFDD2D?style=for-the-badge&labelColor=1a1a1a" alt="Донат через T-Bank"></a>
</p>

**3x-ui-awg** — панель для управления собственными VPN/прокси-серверами, заточенная на то, чтобы трафик не палился под DPI и не выглядел как VPN вообще. Технически это форк [3X-UI](https://github.com/MHSanaei/3x-ui) — как и раньше, всё, что 3X-UI умел (многопротокольные входящие, учёт трафика, подписки, несколько узлов, Telegram-бот), никуда не делось и работает как в оригинале. Но за это время сюда добавился нативный протокол, две независимые системы маскировки трафика, ещё один способ выхода наружу, и куча правок, найденных не в теории, а на живых серверах — так что дальше это описывается как отдельный проект, а не как список патчей к чужому.

> [!IMPORTANT]
> Проект для личного использования. Не используйте его в противозаконных целях или в продакшене.

## AmneziaWG — нативно, а не сбоку

[AmneziaWG](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module) — это WireGuard с добавленным слоем обфускации (мусорные пакеты, случайный паддинг, переписанные магические заголовки), который должен обмануть DPI-фингерпринтинг протокола: тот же туннель, но не выглядящий как туннель в трафике.

- **Встроено в процесс панели, а не модуль ядра.** Работает поверх пользовательского сетевого стека ([amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go)) — никакой сборки DKMS, никакого конфликта с Secure Boot, никаких привилегированных сайдкар-контейнеров, ничего не нужно ставить на хост отдельно.
- **Полноценный протокол наравне с остальными.** Живёт в той же таблице `Inbound`, что и всё остальное — bulk-операции, модалка QR/скачивания конфига, ссылки подписки работают из коробки.
- **Полная обфускация 2.0** — Jc/Jmin/Jmax (мусорные пакеты), S1–S4 (паддинг), H1–H4 (магические заголовки) и все 5 слотов сигнатурных пакетов I1–I5, каждый одним кликом как имитация DNS/STUN/SIP/QUIC-пакета, настоящий TLS ClientHello Chrome/Firefox/Safari или чистая случайность — всё редактируется по инбаунду, плюс совместимость 1.x для старых клиентов.
- **Опционально 3.0 и 3.1** — `HeaderProtectionKey`/`ContentPaddingAddition` и `RandomTrailers`/`DisableCookies`, каждое включение явное и осознанное, само по себе ничего на проводе не меняет.
- **Трафик клиента уже идёт через Xray** — без TPROXY и лишних мостов: инбаунд напрямую передаёт трафик в свой loopback SOCKS5-инбаунд Xray, поэтому статистика, онлайн-статус, sniffing и правила маршрутизации работают как для любого другого протокола.
- **Настоящие ссылки `vpn://`**, которые реально импортирует официальное приложение AmneziaVPN, не выдуманный формат.
- **Отдельная IPv6-идентичность и проброс портов по клиенту** — тоже на встроенном движке.

Это не осталось внутренней доработкой форка — [PR #6105](https://github.com/MHSanaei/3x-ui/pull/6105) с этой реализацией смёржен в основную ветку MHSanaei/3x-ui.

## Маскировка: два независимых способа спрятать панель

Сканирующему ваш `:443` без правильного секрета/пути не должно быть видно, что там вообще есть панель — везде должен быть обычный работающий сайт.

- **Настоящий AdGuard Home как decoy.** Не имитация — панель сама ставит, настраивает и держит живой AdGuard Home и отдаёт его как содержимое decoy: у зашедшего без секрета — рабочий DNS-фильтр с настоящей админкой и DoH, а не заглушка. Логин/пароль AdGuard Home меняются прямо из панели.
- **7 интерактивных login-заглушек** — AdGuard Home, Portainer, Pi-hole, OMV, Jellyfin, Home Assistant, Uptime Kuma — с реальной блокировкой по попыткам входа, не просто картинки форм.
- **Встроенный реверс-прокси** (**Настройки → Реверс-прокси**) сам маршрутизирует по пути: путь панели — в панель, путь подписки — на сервер подписок, всё остальное — на decoy. Укажите его в fallback `target` REALITY-инбаунда. Сертификаты — свои файлы или автовыпуск через Let's Encrypt. Заменяет собой конфиг Nginx, который раньше приходилось писать руками; если что-то пойдёт не так — `x-ui setting -disableFrontProxy` выключает по SSH.

## Ещё один выход наружу: Tor

Однокликовый outbound через Tor — панель сама ставит и держит живым `tor`-процесс, трафик заворачивается через SOCKS5. Медленнее, чем WARP/NordVPN, зато максимум анонимности там, где это важнее скорости.

## Подписки и маршрутизация

- **Пресеты маршрутизации RoscomVPN** для Happ и Incy — DEFAULT/JSONSUB/WHITELIST/Custom одним селектором, набор правил обновляется на лету из внешнего источника.
- **Автокомплит правил маршрутизации** — поля IP/Домен подсказывают категории geosite/geoip прямо из тех `.dat`-файлов, что реально стоят в bin Xray, включая кастомные.
- **Стабильные теги подписок** — обновление подписки больше не может тихо переприсвоить чужой стабильный тег другому серверу.

## Собрано и обкатано на реальных серверах

Каждая правка ниже — не гипотетическая, а найденная и подтверждённая на живой инфраструктуре:

- **Keepalive для туннельных клиентов** — можно явно настроить `PersistentKeepalive`, а не только принимать значение по умолчанию.
- **Live-скорость для AmneziaWG и MTProto** — раньше колонка Speed показывала «--» для обоих, хотя суммарный трафик считался верно.
- **Кастомные файлы в `bin/` переживают обновления** — раньше апдейт стирал папку `bin/` целиком, теперь бэкапит и восстанавливает только то, чего нет в новом релизе.
- Плюс закрытые гонки и баги в TPROXY/firewall-правилах, привязке peer-адресов к конкретному инбаунду, MTU при большом паддинге S4 и IPv6-алиасинге — каждый найден и исправлен по факту, не в теории.

## Возможности (полный список)

- **Многопротокольные входящие** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, **AmneziaWG**, MTProto, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel и TUN.
- **Современные транспорты и безопасность** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade и XHTTP, с TLS, XTLS и REALITY.
- **Fallback** — несколько протоколов на одном порту (например, VLESS и Trojan на 443).
- **Исходящие** — WARP, NordVPN, **Tor**, пользовательские правила маршрутизации, балансировщики, цепочки прокси.
- **Маскировка** — реверс-прокси с decoy (реальный AdGuard Home или login-заглушки).
- **Управление по клиенту** — квоты трафика, даты истечения, лимиты IP, live-статус, ссылки/QR/подписки в один клик.
- **Статистика трафика** — по инбаунду, клиенту и исходящему, со сбросом.
- **Несколько узлов** — управление и масштабирование из одной панели.
- **Встроенный сервер подписок** с [кастомными шаблонами страниц](docs/custom-subscription-templates.md).
- **Telegram-бот** для мониторинга и управления.
- **RESTful API** со Swagger-документацией прямо в панели.
- **SQLite или PostgreSQL** на выбор.
- **Русский и английский интерфейс**, тёмная и светлая темы.
- **Fail2ban** для лимитов IP по клиенту.

## Скриншоты

<details>
<summary>Показать</summary>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/01-overview-dark.png">
  <img alt="Обзор" src="./media/01-overview-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/02-add-inbound-dark.png">
  <img alt="Входящие" src="./media/02-add-inbound-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/03-add-client-dark.png">
  <img alt="Добавление клиента" src="./media/03-add-client-light.png">
</picture>

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./media/05-add-nodes-dark.png">
  <img alt="Конфиги" src="./media/05-add-nodes-light.png">
</picture>

</details>

## Быстрый старт

```bash
curl -fsSL https://raw.githubusercontent.com/kuzzrus/3x-ui-awg/main/install.sh | bash
```

Чтобы установить конкретную версию, добавьте её тег (например, `v3.7.0-awg.18`):

```bash
curl -fsSL https://raw.githubusercontent.com/kuzzrus/3x-ui-awg/main/install.sh | bash -s v3.7.0-awg.18
```

Чтобы установить скользящую сборку **dev** (последний пре-релиз по коммитам из `main`, не стабильный релиз), передайте `dev`:

```bash
curl -fsSL https://raw.githubusercontent.com/kuzzrus/3x-ui-awg/main/install.sh | bash -s dev
```

Стабильные релизы этого форка помечаются тегом `<базовая версия апстрима>-awg.N` (например, `v3.7.0-awg.18`, поверх того, что апстрим называет `v3.7.0`) — никогда просто `vX.Y.Z`, чтобы не спутать с настоящим релизом MHSanaei/3x-ui того же номера.

Во время установки генерируются случайные имя пользователя, пароль и путь доступа. После установки выполните `x-ui`, чтобы открыть меню управления: запуск/остановка сервиса, просмотр или сброс учётных данных, управление SSL-сертификатами и остальное.

Общую документацию по панели, помимо этого README, смотрите в [вики апстрима](https://github.com/MHSanaei/3x-ui/wiki) — там ничего не специфично для этого форка, всё актуально.

### Автоматическая установка

Установщик работает и **неинтерактивно** — для cloud-init. Задайте `XUI_NONINTERACTIVE=1` (или передайте по конвейеру без TTY), и установка пройдёт без единого запроса, с генерацией случайных учётных данных в `/etc/x-ui/install-result.env`. Смотрите [`deploy/`](deploy/):

- [Cloud-init user-data](deploy/cloud-init/) — установка на любом облаке (Hetzner/AWS/DO/Vultr/GCP/Azure/Oracle)
- [Заметки по Hetzner Cloud](deploy/marketplace/hetzner/)

## Поддерживаемые платформы

**ОС:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine. (В апстриме есть ещё сборка под Windows — в CI этого форка её нет, здесь всё нацелено на Linux-серверы/роутеры.)

**Архитектуры:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

AmneziaWG встроен прямо в бинарник панели — никакого модуля ядра, никакого отдельного шага установки, никакой специфики под дистрибутив.

## Варианты базы данных

- **SQLite** (по умолчанию) — один файл `/etc/x-ui/x-ui.db`, без настройки, хватает для небольших и средних развёртываний.
- **PostgreSQL** — для большого числа клиентов или нескольких узлов. Установщик может поставить PostgreSQL локально сам или принять DSN к уже существующему серверу.

```
XUI_DB_TYPE=postgres
XUI_DB_DSN=postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable
```

### Перенос SQLite → PostgreSQL

```bash
x-ui migrate-db --dsn "postgres://xui:password@127.0.0.1:5432/xui?sslmode=disable"
systemctl restart x-ui
```

Исходный файл SQLite остаётся нетронутым — удалите вручную после проверки нового бэкенда.

## Переменные окружения

| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `XUI_DB_TYPE` | Бэкенд БД: `sqlite` или `postgres` | `sqlite` |
| `XUI_DB_DSN` | Строка подключения PostgreSQL | — |
| `XUI_DB_FOLDER` | Каталог файла SQLite | `/etc/x-ui` |
| `XUI_DB_MAX_OPEN_CONNS` | Максимум открытых соединений (пул PostgreSQL) | — |
| `XUI_DB_MAX_IDLE_CONNS` | Максимум простаивающих соединений | — |
| `XUI_INIT_WEB_BASE_PATH` | Начальный URI-путь панели | `/` |
| `XUI_ENABLE_FAIL2BAN` | Fail2ban-лимиты IP | `true` |
| `XUI_LOG_LEVEL` | Уровень логирования | `info` |
| `XUI_DEBUG` | Режим отладки | `false` |
| `XUI_TUNNEL_HEALTH_MONITOR` | Монитор состояния туннеля (перезапускает xray после сбоев — сбрасывает всех клиентов) | `false` |
| `XUI_TUNNEL_HEALTH_PROXY` | Прокси для пробы (например, `socks5://127.0.0.1:1080`) | — |
| `XUI_TUNNEL_HEALTH_URL` | URL для проверки | `https://www.cloudflare.com/cdn-cgi/trace` |
| `XUI_TUNNEL_HEALTH_INTERVAL` | Интервал проб | `30s` |
| `XUI_TUNNEL_HEALTH_TIMEOUT` | Таймаут пробы | `10s` |
| `XUI_TUNNEL_HEALTH_FAILURES` | Сбоев подряд до перезапуска | `3` |
| `XUI_TUNNEL_HEALTH_COOLDOWN` | Минимальная пауза между перезапусками | `5m` |

## Языки интерфейса

Русский и English, тёмная и светлая темы.

## Для разработчиков

Личный форк, сторонние контрибьюторы не ищутся, но [CONTRIBUTING.md](/CONTRIBUTING.md) содержит актуальные инструкции по локальной разработке (версии Go/Node, компилятор для CGo, команды сборки/линта/тестов), если разбираетесь в коде сами.

## Основа и благодарности

Построено поверх [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui) — вся базовая архитектура панели их работа. Реализация AmneziaWG опирается на:

- [amnezia-vpn/amneziawg-go](https://github.com/amnezia-vpn/amneziawg-go) — пользовательская реализация AmneziaWG поверх сетевого стека [gVisor](https://gvisor.dev/), встроенная прямо в процесс панели.
- [MHSanaei/3x-ui#6086](https://github.com/MHSanaei/3x-ui/pull/6086) — исходный PR с AmneziaWG в апстрим (через Docker-сайдкар); отсюда переиспользована фронтенд-схема/структура UI.
- [coinman-dev/3ax-ui](https://github.com/coinman-dev/3ax-ui) — независимый форк с нативным AmneziaWG в проде; ранний менеджер на модуле ядра и генератор параметров обфускации 2.0 были портированы из его пакета `awg/` до перехода на встроенный движок.

Если форк оказался полезен — [донат через T-Bank](https://www.tbank.ru/cf/2qxNvGa3fSX).

Также используются:

- [alireza0](https://github.com/alireza0/)
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (**GPL-3.0**): расширенные правила маршрутизации v2ray/xray с иранскими доменами, упор на безопасность и блокировку рекламы.
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (**GPL-3.0**): автообновляемые правила маршрутизации по заблокированным в России доменам и адресам.

## Инструменты сообщества

- [terraform-provider-3x-ui](https://github.com/batonogov/terraform-provider-threexui) (**MIT**) — управление инбаундами, клиентами, настройками панели и конфигурацией Xray как кодом через Terraform / OpenTofu.
