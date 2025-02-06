# API 文档

## Inbounds

### 获取所有 Inbounds
- **方法**: `GET`
- **端点**: `/inbounds/`
- **描述**: 获取所有 inbounds 的列表。

### 重置所有客户流量
- **方法**: `DELETE`
- **端点**: `/inbounds/traffic`
- **描述**: 重置所有客户的流量。

---

## Inbound

### 添加 Inbound
- **方法**: `POST`
- **端点**: `/inbounds/`
- **描述**: 添加新的 inbound。

### 通过 ID 获取 Inbound
- **方法**: `GET`
- **端点**: `/inbounds/:id`
- **描述**: 通过 ID 获取特定 inbound 的信息。

### 通过 ID 删除 Inbound
- **方法**: `DELETE`
- **端点**: `/inbounds/:id`
- **描述**: 通过 ID 删除 inbound。

### 通过 ID 更新 Inbound
- **方法**: `PUT`
- **端点**: `/inbounds/:id`
- **描述**: 通过 ID 更新 inbound 的信息。

### 删除 Inbound 流量
- **方法**: `DELETE`
- **端点**: `/inbounds/:id/traffic`
- **描述**: 通过 ID 删除 inbound 流量。

### 删除流量耗尽的客户
- **方法**: `DELETE`
- **端点**: `/inbounds/:id/depleted-clients`
- **描述**: 删除特定 inbound 下流量已耗尽的客户。

---

## Inbound 客户

### 获取 Inbound 客户
- **方法**: `GET`
- **端点**: `/inbounds/:id/clients/`
- **描述**: 获取特定 inbound 的客户列表。

---

## Inbound 客户管理

### 添加 Inbound 客户
- **方法**: `POST`
- **端点**: `/inbounds/:id/clients`
- **描述**: 向 inbound 添加新的客户。

### 通过 ID 获取客户
- **方法**: `GET`
- **端点**: `/inbounds/:id/clients/:clientId`
- **描述**: 通过 ID 获取客户信息。

### 更新 Inbound 客户
- **方法**: `PUT`
- **端点**: `/inbounds/:id/clients/:clientId`
- **描述**: 通过 ID 更新客户信息。

### 删除 Inbound 客户
- **方法**: `DELETE`
- **端点**: `/inbounds/:id/clients/:clientId`
- **描述**: 通过 ID 删除客户。

### 通过 ID 获取客户流量
- **方法**: `GET`
- **端点**: `/inbounds/:id/clients/:clientId/traffic`
- **描述**: 通过 ID 获取客户流量统计信息。

---

## 通过电子邮件管理客户

### 通过电子邮件获取客户
- **方法**: `GET`
- **端点**: `/inbounds/:id/clients/email/:email`
- **描述**: 通过电子邮件获取客户信息。

### 获取客户 IP 地址
- **方法**: `GET`
- **端点**: `/inbounds/:id/clients/email/:email/ips`
- **描述**: 通过电子邮件获取客户的 IP 地址列表。

### 清除客户 IP 地址
- **方法**: `DELETE`
- **端点**: `/inbounds/:id/clients/email/:email/ips`
- **描述**: 清除客户的 IP 地址列表。

### 通过电子邮件获取客户流量
- **方法**: `GET`
- **端点**: `/inbounds/:id/clients/email/:email/traffic`
- **描述**: 通过电子邮件获取客户流量统计信息。

### 通过电子邮件重置客户流量
- **方法**: `DELETE`
- **端点**: `/inbounds/:id/clients/email/:email/traffic`
- **描述**: 通过电子邮件重置客户流量。

---

## 其他功能

### 创建备份
- **方法**: `GET`
- **端点**: `/inbounds/create-backup`
- **描述**: 创建数据备份。

### 获取在线客户
- **方法**: `GET`
- **端点**: `/inbounds/online`
- **描述**: 获取在线客户列表。

---

## 服务器

### 获取服务器状态
- **方法**: `GET`
- **端点**: `/server/status`
- **描述**: 获取服务器状态。