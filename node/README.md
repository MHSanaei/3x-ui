# 3x-ui Node Service

Node service (worker) для multi-node архитектуры 3x-ui.

## Описание

Этот сервис запускается на отдельных серверах и управляет XRAY Core инстансами. Панель 3x-ui (master) отправляет конфигурации на ноды через REST API.

## Функциональность

- REST API для управления XRAY Core
- Применение конфигураций от панели
- Перезагрузка XRAY без остановки контейнера
- Проверка статуса и здоровья

## API Endpoints

### `GET /health`
Проверка здоровья сервиса (без аутентификации)

### `POST /api/v1/apply-config`
Применить новую конфигурацию XRAY
- **Headers**: `Authorization: Bearer <api-key>`
- **Body**: JSON конфигурация XRAY

### `POST /api/v1/reload`
Перезагрузить XRAY
- **Headers**: `Authorization: Bearer <api-key>`

### `GET /api/v1/status`
Получить статус XRAY
- **Headers**: `Authorization: Bearer <api-key>`

## Запуск

### Docker Compose

```bash
cd node
NODE_API_KEY=your-secure-api-key docker-compose up -d --build
```

**Примечание:** XRAY Core автоматически скачивается во время сборки Docker-образа для вашей архитектуры. Docker BuildKit автоматически определяет архитектуру хоста. Для явного указания архитектуры используйте:

```bash
DOCKER_BUILDKIT=1 docker build --build-arg TARGETARCH=arm64 -t 3x-ui-node -f node/Dockerfile ..
```

### Вручную

```bash
go run node/main.go -port 8080 -api-key your-secure-api-key
```

## Переменные окружения

- `NODE_API_KEY` - API ключ для аутентификации (обязательно)

## Структура

```
node/
├── main.go           # Точка входа
├── api/
│   └── server.go     # REST API сервер
├── xray/
│   └── manager.go    # Управление XRAY процессом
├── Dockerfile        # Docker образ
└── docker-compose.yml
```
