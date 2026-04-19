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

**3X-UI** — продвинутая панель управления с открытым исходным кодом на основе веб-интерфейса, разработанная для управления сервером Xray-core. Предоставляет удобный интерфейс для настройки и мониторинга различных VPN и прокси-протоколов.

> [!IMPORTANT]
> Этот проект предназначен только для личного использования, пожалуйста, не используйте его в незаконных целях и в производственной среде.

Как улучшенная версия оригинального проекта X-UI, 3X-UI обеспечивает повышенную стабильность, более широкую поддержку протоколов и дополнительные функции.

## Пользовательские GeoSite / GeoIP (DAT)

В панели можно задать свои источники `.dat` по URL (тот же сценарий, что и для встроенных геофайлов). Файлы сохраняются в каталоге с бинарником Xray (`XUI_BIN_FOLDER`, по умолчанию `bin/`) как `geosite_&lt;alias&gt;.dat` и `geoip_&lt;alias&gt;.dat`.

**Маршрутизация:** в правилах используйте форму `ext:имя_файла.dat:тег`, например `ext:geosite_myalias.dat:tag` (как у региональных списков `ext:geoip_IR.dat:ir`).

**Зарезервированные псевдонимы:** только для проверки на резерв используется нормализованная форма (`strings.ToLower`, `-` → `_`). Введённые пользователем псевдонимы и имена файлов в БД не переписываются и должны соответствовать `^[a-z0-9_-]+$`. Например, `geoip-ir` и `geoip_ir` попадают под одну и ту же зарезервированную запись.

## Быстрый старт

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

Полную документацию смотрите в [вики проекта](https://github.com/MHSanaei/3x-ui/wiki).

## Базы данных

3X-UI поддерживает `SQLite` и `PostgreSQL` как взаимозаменяемые бэкенды. Вся логика приложения написана через [GORM](https://gorm.io/) — ORM для Go с поддержкой множества СУБД, — поэтому запросы работают одинаково на любом движке. Переключиться между бэкендами можно в любой момент через UI панели без потери данных.

### Выбор бэкенда

| | SQLite | PostgreSQL |
|---|---|---|
| Настройка | Ноль конфигурации, файловая БД | Требует работающий PG-сервер |
| Подходит для | Один узел, малый трафик | Несколько узлов, высокая нагрузка |
| Резервные копии | Портативный + нативный файл | Только портативный |

### Переключение бэкенда через UI

1. Откройте **Настройки → Панель → База данных**.
2. Выберите бэкенд (`SQLite` или `PostgreSQL`) и заполните параметры подключения.
3. Нажмите **Проверить подключение**.
4. Нажмите **Переключить базу данных** — панель автоматически:
   - Сохранит портативную резервную копию текущих данных.
   - Мигрирует все данные в новый бэкенд.
   - Перезапустится.

> Целевая база данных должна быть пустой перед переключением. Всегда используйте **Проверить подключение** перед переключением.

### Локальный PostgreSQL (управляется панелью)

Режим **Локальный (управляется панелью)** — панель устанавливает и настраивает PostgreSQL автоматически (только Linux, root):

```bash
# Панель использует postgres-manager.sh внутри себя.
# Ручная настройка PostgreSQL не требуется.
```

### Внешний PostgreSQL

Подключение к существующему серверу PostgreSQL 13+:

1. Создайте отдельную БД и пользователя.
2. Введите параметры подключения в Настройки → База данных.
3. Нажмите **Проверить подключение**, затем **Переключить базу данных**.

### Переопределение через переменные окружения

Для Docker и IaC-деплоев можно управлять бэкендом через переменные окружения:

```bash
XUI_DB_DRIVER=postgres        # или: sqlite
XUI_DB_HOST=127.0.0.1
XUI_DB_PORT=5432
XUI_DB_NAME=x-ui
XUI_DB_USER=x-ui
XUI_DB_PASSWORD=change-me
XUI_DB_SSLMODE=disable        # или: require, verify-ca, verify-full
XUI_DB_MODE=external          # или: local
XUI_DB_PATH=/etc/x-ui/db/x-ui.db   # только для SQLite
```

Если задана любая переменная `XUI_DB_*`, раздел «База данных» в UI становится read-only.

### Резервное копирование и восстановление

| Формат | Совместим с | Когда использовать |
|---|---|---|
| **Портативный** (`.xui-backup`) | SQLite + PostgreSQL | Переключение бэкендов, резервные копии через Telegram-бот, долгосрочное хранение |
| **Нативный SQLite** (`.db`) | Только SQLite | Быстрый файловый бэкап при активном SQLite |

- Telegram-бот отправляет **портативную резервную копию** автоматически — это работает независимо от активного бэкенда.
- Портативные копии можно импортировать на SQLite и на PostgreSQL.
- Устаревшие `.db`-файлы от старых версий 3x-ui можно импортировать даже при активном PostgreSQL.

## Особая благодарность

- [alireza0](https://github.com/alireza0/)

## Благодарности

- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (Лицензия: **GPL-3.0**): _Улучшенные правила маршрутизации для v2ray/xray и v2ray/xray-clients со встроенными иранскими доменами и фокусом на безопасность и блокировку рекламы._
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (Лицензия: **GPL-3.0**): _Этот репозиторий содержит автоматически обновляемые правила маршрутизации V2Ray на основе данных о заблокированных доменах и адресах в России._

## Поддержка проекта

**Если этот проект полезен для вас, вы можете поставить ему**:star2:

<a href="https://www.buymeacoffee.com/MHSanaei" target="_blank">
<img src="./media/default-yellow.png" alt="Buy Me A Coffee" style="height: 70px !important;width: 277px !important;" >
</a>

</br>
<a href="https://nowpayments.io/donation/hsanaei" target="_blank" rel="noreferrer noopener">
   <img src="./media/donation-button-black.svg" alt="Crypto donation button by NOWPayments">
</a>

## Звезды с течением времени

[![Stargazers over time](https://starchart.cc/MHSanaei/3x-ui.svg?variant=adaptive)](https://starchart.cc/MHSanaei/3x-ui) 
