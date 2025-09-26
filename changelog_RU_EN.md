# 📌 Changelog

---

## 🇬🇧 English

### 🔐 LDAP Integration for Client Access Management
- **Scheduled sync job** reads LDAP and:
  - Enables/disables clients by `client.email` (local status storage, no LDAP lookups on connect).
  - *Optionally* auto-creates clients in selected inbounds if they don’t exist yet.
  - *Optionally* auto-deletes clients from selected inbounds if they disappear in LDAP or access flag = `deny`.

### ⚙️ Access Flag Evaluation
- Uses:
  - `ldapFlagField` (fallback: `ldapVlessField` if empty),
  - `ldapTruthyValues` (default: `true, 1, yes, on`),
  - `ldapInvertFlag` (useful for `shadowInactive`).
- **Example:**  
  `shadowInactive=1` + `ldapTruthyValues=1` + `ldapInvertFlag=true` → client **disabled**.
- **Matching:** `ldapUserAttr` (e.g., `mail`) ↔ `client.email`.

### 🆕 New Settings  
**Panel → Settings → General → LDAP**  
- **Connection:** `ldapHost`, `ldapPort`, `ldapUseTLS`, `ldapBindDN`, `ldapPassword`, `ldapBaseDN`.  
- **User Search:** `ldapUserFilter`, `ldapUserAttr`.  
- **Access Flag:** `ldapFlagField` (or `ldapVlessField`), `ldapTruthyValues`, `ldapInvertFlag`.  
- **Scheduler:** `ldapEnable`, `ldapSyncCron` (e.g., `@every 1m`).  
- **Inbound Management (multi-select):** `ldapInboundTags`.  
- **Auto Actions:** `ldapAutoCreate`, `ldapAutoDelete`.  
- **Defaults for Auto-Create:**  
  - `ldapDefaultTotalGB`  
  - `ldapDefaultExpiryDays`  
  - `ldapDefaultLimitIP` (0 = unlimited).  

👉 UI: `ldapInboundTags` options are fetched from `/panel/inbound/list`; a hint is shown if none exist.

### 🛠️ Auto-Create Rules
- **VLESS/VMESS:** generate UUID, use `security/flow` from inbound.  
- **Trojan:** generate password, email = LDAP mail.  
- **Shadowsocks:** take method from inbound, generate password.  
- **All protocols:** `enable` from LDAP flag, limits/expiry from defaults.  

### 🔧 Implementation
- **Sync job:** `web/job/ldap_sync_job.go`  
- **LDAP client:** `util/ldap/ldap.go` (go-ldap/ldap/v3)  
- **Backend:** `web/service/setting.go`, `web/entity/entity.go`  
- **Frontend:**  
  - `web/assets/js/model/setting.js`  
  - `web/html/settings/panel/general.html`  
  - `web/html/settings.html`  
- **Xray restart:** handled by scheduler when changes occur.

### 🗄️ External MySQL Database
- Support added for **external MySQL** instead of local DB.  
- Docker env supports external DB parameters.  
- Panel starts as soon as DB is reachable.

### 🔄 Upgrade Notes
- Frontend assets are version-busted (`config/version`) → **hard refresh browser** after update (`Ctrl+F5`).  
- If you previously used only `vless_enabled`, you can now use any LDAP attribute via `ldapFlagField + ldapTruthyValues + ldapInvertFlag`.  
- To enable auto-create, select inbounds in `ldapInboundTags` and enable `ldapAutoCreate`.  

---

## 🇷🇺 Русский

### 🔐 Интеграция с LDAP для управления доступом клиентов
- **Периодическая синхронизация (cron)**:
  - Включает/выключает клиентов по `client.email` (локально, без онлайн-запросов).  
  - *Опционально* автосоздаёт клиентов в выбранных инбаундах.  
  - *Опционально* автоудаляет клиентов, если их нет в LDAP или флаг запрещает доступ.  

### ⚙️ Логика проверки флага доступа
- Используется:  
  - `ldapFlagField` (если пусто → `ldapVlessField`),  
  - `ldapTruthyValues` (по умолчанию: `true,1,yes,on`),  
  - `ldapInvertFlag` (например, для `shadowInactive`).  
- **Пример:**  
  `shadowInactive=1` + `ldapTruthyValues=1` + `ldapInvertFlag=true` → клиент **отключён**.  
- **Сопоставление:** `ldapUserAttr` (обычно `mail`) ↔ `client.email`.

### 🆕 Новые параметры  
**Панель → Settings → General → LDAP**  
- **Подключение:** `ldapHost`, `ldapPort`, `ldapUseTLS`, `ldapBindDN`, `ldapPassword`, `ldapBaseDN`.  
- **Поиск пользователей:** `ldapUserFilter`, `ldapUserAttr`.  
- **Флаг доступа:** `ldapFlagField` (или `ldapVlessField`), `ldapTruthyValues`, `ldapInvertFlag`.  
- **Планировщик:** `ldapEnable`, `ldapSyncCron` (например, `@every 1m`).  
- **Управление инбаундами:** `ldapInboundTags`.  
- **Автодействия:** `ldapAutoCreate`, `ldapAutoDelete`.  
- **Дефолты для автосоздания:**  
  - `ldapDefaultTotalGB`  
  - `ldapDefaultExpiryDays`  
  - `ldapDefaultLimitIP` (0 = безлимит).  

👉 UI: `ldapInboundTags` выбираются из `/panel/inbound/list`. Если инбаундов нет — показывается подсказка.  

### 🛠️ Правила автосоздания
- **VLESS/VMESS:** генерируется UUID, параметры `security/flow` из инбаунда.  
- **Trojan:** генерируется пароль, email = mail из LDAP.  
- **Shadowsocks:** берёт `method` из инбаунда, генерирует пароль.  
- **Все протоколы:** статус включён/выключен по флагу, лимиты/срок из дефолтов.  

### 🔧 Реализация
- **Job:** `web/job/ldap_sync_job.go`  
- **LDAP-клиент:** `util/ldap/ldap.go` (go-ldap/ldap/v3)  
- **Backend:** `web/service/setting.go`, `web/entity/entity.go`  
- **Frontend:**  
  - `web/assets/js/model/setting.js`  
  - `web/html/settings/panel/general.html`  
  - `web/html/settings.html`  
- **Перезапуск Xray:** флаг на изменения → обрабатывается планировщиком.  

### 🗄️ Внешняя база MySQL
- Поддержка подключения к **внешней MySQL** вместо локальной.  
- Docker-окружение принимает параметры внешней БД.  
- Панель стартует при доступности БД.  

### 🔄 Замечания по обновлению
- Версия фронтенда контролируется через `config/version` → после обновления сделайте **жёсткое обновление страницы** (`Ctrl+F5`).  
- Если раньше использовался только `vless_enabled`, теперь можно использовать произвольный атрибут (`ldapFlagField + ldapTruthyValues + ldapInvertFlag`).  
- Для автосоздания заранее выберите инбаунды в `ldapInboundTags` и включите `ldapAutoCreate`.  
