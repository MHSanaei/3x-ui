# 03. API and CLI Reference

## API auth

- Session cookie required.
- API group is mounted under `/panel/api`.
- Unauthenticated API calls are hidden (404 behavior).

## Inbounds API (main set)

Examples:
- `GET /panel/api/inbounds/list`
- `GET /panel/api/inbounds/get/:id`
- `POST /panel/api/inbounds/add`
- `POST /panel/api/inbounds/update/:id`
- `POST /panel/api/inbounds/del/:id`
- `POST /panel/api/inbounds/addClient`
- `POST /panel/api/inbounds/updateClient/:clientId`
- `POST /panel/api/inbounds/:id/delClient/:clientId`
- `GET /panel/api/inbounds/getClientTraffics/:email`

Controller file:
- `web/controller/inbound.go`

## Server API

Examples:
- `GET /panel/api/server/status`
- `POST /panel/api/server/restartXrayService`
- `POST /panel/api/server/stopXrayService`
- `POST /panel/api/server/logs/:count`

Controller file:
- `web/controller/server.go`

## Custom Clients API (implemented)

- `GET /panel/api/clients/list`
- `GET /panel/api/clients/inbounds`
- `POST /panel/api/clients/add`
- `POST /panel/api/clients/update/:id`
- `POST /panel/api/clients/del/:id`

Files:
- `web/controller/client_center.go`
- `web/service/client_center.go`

## Panel and Xray settings APIs

- `/panel/setting/*`
- `/panel/xray/*`

Files:
- `web/controller/setting.go`
- `web/controller/xray_setting.go`

## CLI usage

Binary supports:
- `run`
- `setting`
- `migrate`

Examples:
```bash
go run . setting -port 2099 -username admin -password admin
go run . run
```

Admin shell scripts:
- `x-ui.sh`
- `install.sh`
- `update.sh`
