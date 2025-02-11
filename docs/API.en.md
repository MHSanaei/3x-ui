# API Documentation

To make requests to REST API v2, you need to include the Authorization header with the Bearer type and the token in each request.

```
Authorization: Bearer {token}
```

## Inbounds

### Get All Inbounds
- **Method**: `GET`
- **Endpoint**: `/inbounds/`
- **Description**: Retrieve a list of all inbounds.

### Reset All Client Traffic
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/traffic`
- **Description**: Reset the traffic of all clients.

---

## Inbound

### Add Inbound
- **Method**: `POST`
- **Endpoint**: `/inbounds/`
- **Description**: Add a new inbound.

### Get Inbound by ID
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id`
- **Description**: Retrieve information about a specific inbound by its ID.

### Delete Inbound by ID
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id`
- **Description**: Delete an inbound by its ID.

### Update Inbound by ID
- **Method**: `PUT`
- **Endpoint**: `/inbounds/:id`
- **Description**: Update information about an inbound by its ID.

### Delete Inbound Traffic
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/traffic`
- **Description**: Delete traffic for an inbound by its ID.

### Delete Depleted Clients
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/depleted-clients`
- **Description**: Remove clients with exhausted traffic for a specific inbound.

---

## Inbound Clients

### Get Inbound Clients
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/`
- **Description**: Retrieve a list of clients for a specific inbound.

---

## Inbound Client

### Add Inbound Client
- **Method**: `POST`
- **Endpoint**: `/inbounds/:id/clients`
- **Description**: Add a new client to an inbound.

### Get Client by ID
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Description**: Retrieve information about a client by its ID.

### Update Inbound Client
- **Method**: `PUT`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Description**: Update client information by its ID.

### Delete Inbound Client
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Description**: Delete a client by its ID.

### Get Client Traffic by ID
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/:clientId/traffic`
- **Description**: Retrieve traffic statistics for a client by its ID.

---

## Inbound Client by Email

### Get Client by Email
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email`
- **Description**: Retrieve client information by email.

### Get Client IPs
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email/ips`
- **Description**: Retrieve a list of client IP addresses by email.

### Clear Client IPs
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/email/:email/ips`
- **Description**: Clear the list of client IP addresses by email.

### Get Client Traffic by Email
- **Method**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email/traffic`
- **Description**: Retrieve traffic statistics for a client by email.

### Reset Client Traffic by Email
- **Method**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/email/:email/traffic`
- **Description**: Reset a client's traffic by email.

---

## Other

### Create Backup
- **Method**: `GET`
- **Endpoint**: `/inbounds/create-backup`
- **Description**: Create a data backup.

### Get Online Clients
- **Method**: `GET`
- **Endpoint**: `/inbounds/online`
- **Description**: Retrieve a list of online clients.

---

## Server

### Get Server Status
- **Method**: `GET`
- **Endpoint**: `/server/status`
- **Description**: Retrieve the server status.