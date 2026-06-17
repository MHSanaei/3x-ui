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

**DUNE** — облегчённый форк [3X-UI](https://github.com/MHSanaei/3x-ui): веб-панель с открытым исходным кодом для управления серверами [Xray-core](https://github.com/XTLS/Xray-core). Сохраняет привычные сценарии работы и поддержку протоколов 3X-UI, но потребляет значительно меньше CPU и RAM — идеально для небольших VPS и слабых серверов.

Ответвлён от 3X-UI с упором на эффективность: меньше фоновых задач, экономнее память и более лёгкий стек, чтобы панель оставалась отзывчивой, не перегружая сервер.

> [!IMPORTANT]
> Этот проект предназначен только для личного использования. Пожалуйста, не используйте его в незаконных целях или в производственной среде.

## Возможности

- **Многопротокольные входящие подключения** — VLESS, VMess, Trojan, Shadowsocks, WireGuard, Hysteria2, HTTP, SOCKS (Mixed), Dokodemo-door / Tunnel и TUN.
- **Современные транспорты и безопасность** — TCP (Raw), mKCP, WebSocket, gRPC, HTTPUpgrade и XHTTP, защищённые с помощью TLS, XTLS и REALITY.
- **Fallback** — обслуживание нескольких протоколов на одном порту (например, VLESS и Trojan на 443) с помощью функции fallback в Xray.
- **Управление по каждому клиенту** — квоты трафика, даты истечения, лимиты IP, статус «онлайн» в реальном времени, а также ссылки для общего доступа, QR-коды и подписки в один клик.
- **Статистика трафика** — по каждому входящему, по каждому клиенту и по каждому исходящему, с возможностью сброса.
- **Поддержка нескольких узлов** — управление и масштабирование на несколько серверов из одной панели.
- **Исходящие подключения и маршрутизация** — WARP, NordVPN, пользовательские правила маршрутизации, балансировщики нагрузки и цепочки исходящих прокси.
- **Встроенный сервер подписок** с несколькими форматами вывода.
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
bash <(curl -Ls https://raw.githubusercontent.com/leto217/DUNE/main/install.sh)
```

Во время установки генерируются случайные имя пользователя, пароль и путь доступа. После установки выполните `dune`, чтобы открыть меню управления, где можно запускать/останавливать сервис, просматривать или сбрасывать учётные данные для входа, управлять SSL-сертификатами и многое другое.

Полную документацию смотрите в [вики проекта](https://github.com/leto217/DUNE/wiki).

## Поддерживаемые платформы

**Операционные системы:** Ubuntu, Debian, Armbian, Fedora, CentOS, RHEL, AlmaLinux, Rocky Linux, Oracle Linux, Amazon Linux, Virtuozzo, Arch, Manjaro, Parch, openSUSE (Tumbleweed / Leap), Alpine и Windows.

**Архитектуры:** `amd64` · `386` · `arm64` (aarch64) · `armv7` · `armv6` · `armv5` · `s390x`.

## Варианты базы данных

Dune поддерживает два бэкенда, выбираемых при установке:

- **SQLite** (по умолчанию) — единый файл по пути `/etc/dune/dune.db`. Без настройки, идеально для небольших и средних развёртываний.
- **PostgreSQL** — рекомендуется при большом числе клиентов или конфигурациях с несколькими узлами. Установщик может установить PostgreSQL локально за вас или принять DSN к существующему серверу.

Во время выполнения бэкенд выбирается через переменные окружения (установщик записывает их за вас в `/etc/default/dune`):

```
DUNE_DB_TYPE=postgres
DUNE_DB_DSN=postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable
```

### Перенос существующей установки SQLite в PostgreSQL

```bash
dune migrate-db --dsn "postgres://dune:password@127.0.0.1:5432/dune?sslmode=disable"
# затем задайте DUNE_DB_TYPE и DUNE_DB_DSN в /etc/default/dune и перезапустите:
systemctl restart dune
```

Исходный файл SQLite остаётся нетронутым; удалите его вручную после проверки нового бэкенда.

### Docker

Команда по умолчанию `docker compose up -d` продолжает использовать SQLite. Чтобы запустить со встроенным сервисом PostgreSQL, раскомментируйте две строки переменных окружения `DUNE_DB_*` в `docker-compose.yml` и запустите с профилем:

```bash
docker compose --profile postgres up -d
```

Образ включает Fail2ban (включён по умолчанию) для применения **лимитов IP** по каждому клиенту. Fail2ban блокирует нарушителей с помощью `iptables`, что требует возможности `NET_ADMIN`. `docker-compose.yml` уже предоставляет её через `cap_add`; если вы вместо этого запускаете контейнер через `docker run`, добавьте возможности самостоятельно, иначе блокировки будут регистрироваться, но никогда не применяться:

```bash
docker run -d --cap-add=NET_ADMIN --cap-add=NET_RAW ... ghcr.io/leto217/DUNE
```

## Переменные окружения

| Переменная | Описание | По умолчанию |
| --- | --- | --- |
| `DUNE_DB_TYPE` | Бэкенд базы данных: `sqlite` или `postgres` | `sqlite` |
| `DUNE_DB_DSN` | Строка подключения PostgreSQL (когда `DUNE_DB_TYPE=postgres`) | — |
| `DUNE_DB_FOLDER` | Каталог для файла базы данных SQLite | `/etc/dune` |
| `DUNE_DB_MAX_OPEN_CONNS` | Максимум открытых соединений (пул PostgreSQL) | — |
| `DUNE_DB_MAX_IDLE_CONNS` | Максимум простаивающих соединений (пул PostgreSQL) | — |
| `DUNE_INIT_WEB_BASE_PATH` | Начальный URI-путь для веб-панели | `/` |
| `DUNE_ENABLE_FAIL2BAN` | Включить применение лимитов IP на основе Fail2ban | `true` |
| `DUNE_LOG_LEVEL` | Уровень логирования (`debug`, `info`, `warning`, `error`) | `info` |
| `DUNE_DEBUG` | Включить режим отладки | `false` |

## Поддерживаемые языки

Интерфейс панели доступен на 13 языках:

English · فارسی · العربية · 中文（简体） · 中文（繁體） · Español · Русский · Українська · Türkçe · Tiếng Việt · 日本語 · Bahasa Indonesia · Português (Brasil)

## Участие в разработке

Вклад приветствуется. Пожалуйста, прочитайте [руководство по участию](/CONTRIBUTING.md), прежде чем открывать issue или pull request.

## Особая благодарность

- [alireza0](https://github.com/alireza0/)

## Благодарности

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Лицензия: **GPL-3.0**): _Улучшенные правила маршрутизации для v2ray/xray и v2ray/xray-clients со встроенными иранскими доменами и фокусом на безопасность и блокировку рекламы._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Лицензия: **GPL-3.0**): _Этот репозиторий содержит автоматически обновляемые правила маршрутизации V2Ray на основе данных о заблокированных доменах и адресах в России._

## Инструменты сообщества

Инструменты и интеграции, созданные сообществом вокруг dune.

- [terraform-provider-dune](https://github.com/batonogov/terraform-provider-threexui) (Лицензия: **MIT**): _Управление входящими, клиентами, настройками панели и конфигурацией Xray через код с помощью Terraform / OpenTofu._

## Поддержка проекта

**Если этот проект полезен для вас, вы можете поставить ему**:star2:

| Сеть | Адрес |
| --- | --- |
| TON | `UQAa5FpNlK8Gp7tO8luJXHD-Sf0pPjJbNHGo8hdkyuUBhWEa` |
| TRON | `TLqtTfYSzPLFm8mtFDkSnXvzucxx7DS5VL` |
| ERC20 and BEP20 | `0x2fe632d70f4612b87670f8a28b4587ea2641452d` |

## Звезды с течением времени

[![Stargazers over time](https://starchart.cc/leto217/DUNE.svg?variant=adaptive)](https://starchart.cc/leto217/DUNE)
