# Documentación de la API

## Inbounds

### Obtener todos los Inbounds
- **Método**: `GET`
- **Endpoint**: `/inbounds/`
- **Descripción**: Obtener una lista de todos los inbounds.

### Restablecer todo el tráfico de clientes
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/traffic`
- **Descripción**: Restablecer el tráfico de todos los clientes.

---

## Inbound

### Agregar Inbound
- **Método**: `POST`
- **Endpoint**: `/inbounds/`
- **Descripción**: Agregar un nuevo inbound.

### Obtener Inbound por ID
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id`
- **Descripción**: Obtener información sobre un inbound específico por su ID.

### Eliminar Inbound por ID
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/:id`
- **Descripción**: Eliminar un inbound por su ID.

### Actualizar Inbound por ID
- **Método**: `PUT`
- **Endpoint**: `/inbounds/:id`
- **Descripción**: Actualizar la información de un inbound por su ID.

### Eliminar tráfico de Inbound
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/:id/traffic`
- **Descripción**: Eliminar el tráfico de un inbound por su ID.

### Eliminar clientes con tráfico agotado
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/:id/depleted-clients`
- **Descripción**: Eliminar clientes con tráfico agotado para un inbound específico.

---

## Clientes de Inbound

### Obtener clientes de Inbound
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id/clients/`
- **Descripción**: Obtener una lista de clientes para un inbound específico.

---

## Cliente de Inbound

### Agregar cliente de Inbound
- **Método**: `POST`
- **Endpoint**: `/inbounds/:id/clients`
- **Descripción**: Agregar un nuevo cliente a un inbound.

### Obtener cliente por ID
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Descripción**: Obtener información sobre un cliente por su ID.

### Actualizar cliente de Inbound
- **Método**: `PUT`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Descripción**: Actualizar la información del cliente por su ID.

### Eliminar cliente de Inbound
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/:clientId`
- **Descripción**: Eliminar un cliente por su ID.

### Obtener tráfico del cliente por ID
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id/clients/:clientId/traffic`
- **Descripción**: Obtener estadísticas de tráfico para un cliente por su ID.

---

## Cliente de Inbound por correo electrónico

### Obtener cliente por correo electrónico
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email`
- **Descripción**: Obtener información del cliente por correo electrónico.

### Obtener IPs del cliente
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email/ips`
- **Descripción**: Obtener una lista de direcciones IP del cliente por correo electrónico.

### Limpiar IPs del cliente
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/email/:email/ips`
- **Descripción**: Limpiar la lista de direcciones IP del cliente por correo electrónico.

### Obtener tráfico del cliente por correo electrónico
- **Método**: `GET`
- **Endpoint**: `/inbounds/:id/clients/email/:email/traffic`
- **Descripción**: Obtener estadísticas de tráfico para un cliente por correo electrónico.

### Restablecer tráfico del cliente por correo electrónico
- **Método**: `DELETE`
- **Endpoint**: `/inbounds/:id/clients/email/:email/traffic`
- **Descripción**: Restablecer el tráfico de un cliente por correo electrónico.

---

## Otros

### Crear copia de seguridad
- **Método**: `GET`
- **Endpoint**: `/inbounds/create-backup`
- **Descripción**: Crear una copia de seguridad de los datos.

### Obtener clientes en línea
- **Método**: `GET`
- **Endpoint**: `/inbounds/online`
- **Descripción**: Obtener una lista de clientes en línea.

---

## Servidor

### Obtener estado del servidor
- **Método**: `GET`
- **Endpoint**: `/server/status`
- **Descripción**: Obtener el estado del servidor.