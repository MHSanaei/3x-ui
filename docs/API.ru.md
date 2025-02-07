# API Documentation

## Inbounds

### Get All Inbounds
- **Method**: `GET`
- **Endpoint**: `/inbounds/`
- **Description**: Получить список всех inbounds.

### Reset All Client Traffics
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/traffic`
- **Description**: Сбросить трафик всех клиентов.

---

## Inbound

### Add Inbound
- **Method**: `POST`
- **Endpoint**: `/inbounds/`
- **Description**: Добавить новый inbound.

### Get Inbound by ID
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id`
- **Description**: Получить информацию о конкретном inbound по его ID.

### Delete Inbound by ID
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id`
- **Description**: Удалить inbound по его ID.

### Update Inbound by ID
- **Method**: `PUT`
- **Endpoint**: `/inbounds/:id`
- **Description**: Обновить информацию о inbound по его ID.

### Delete Inbound Traffic
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/traffic`
- **Description**: Удалить трафик inbound по его ID.

### Delete Depleted Clients
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/depleted-clients`
- **Description**: Удалить клиентов с исчерпанным трафиком для конкретного inbound.

---

## Inbound Clients

### Get Inbound Clients
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/`
- **Description**: Получить список клиентов для конкретного inbound.

---

## Inbound Client

### Add Inbound Client
- **Method**: `POST`
- **Endpoint**: `/inbounds/:id/clients`
- **Description**: Добавить нового клиента к inbound.

### Get Client by ID
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Description**: Получить информацию о клиенте по его ID.

### Update Inbound Client
- **Method**: `PUT`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Description**: Обновить информацию о клиенте по его ID.

### Delete Inbound Client
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Description**: Удалить клиента по его ID.

### Get Client Traffics by ID
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/:clientId/traffic`
- **Description**: Получить статистику трафика клиента по его ID.

---

## Inbound Client by Email

### Get Client by Email
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email`
- **Description**: Получить информацию о клиенте по его email.

### Get Client IPs
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email/ips`
- **Description**: Получить список IP-адресов клиента по его email.

### Clear Client IPs
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/email/:email/ips`
- **Description**: Очистить список IP-адресов клиента по его email.

### Get Client Traffics by Email
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email/traffic`
- **Description**: Получить статистику трафика клиента по его email.

### Reset Client Traffic by Email
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/email/:email/traffic`
- **Description**: Сбросить трафик клиента по его email.

---

## Other

### Create Backup
- **Method**: `GET`
- **Endpoint**: `/inbounds/create-backup`
- **Description**: Создать резервную копию данных.

### Get Online Clients
- **Method**: `GET`
- **Endpoint**: `/inbounds/online`
- **Description**: Получить список онлайн-клиентов.

---

## Server

### Get Server Status
- **Method**: `GET`
- **Endpoint**: `/server/status`
- **Description**: Получить статус сервера.