# IP Limit Live Integration Guide

## Overview

The IP Limit feature provides IP-based access control. Clients can connect from multiple IPs up to a configured limit while maintaining security.

## Architecture

### Database Schema

```sql
CREATE TABLE inbound_client_ips (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    client_email TEXT UNIQUE NOT NULL,
    ips TEXT NOT NULL DEFAULT '',
    created_at INTEGER DEFAULT 0,
    updated_at INTEGER DEFAULT 0
);
```

### IP Storage Format

IPs are stored as comma-separated values:
```
client_email: user@example.com
ips: 192.168.1.100,203.0.113.45,198.51.100.200
```

## Components

### 1. IPLimitService (`web/service/ip_limit_service.go`)

Core IP limit functionality:

- **CheckIPLimit(email, limit, newIP)** - Validates if a new IP exceeds the limit
- **RecordIPAccess(email, ip)** - Records IP access
- **GetClientIPs(email)** - Retrieves all IPs for a client
- **RemoveIP(email, ipToRemove)** - Removes a specific IP
- **ClearAllIPs(email)** - Clears all IPs for a client

### 2. Database Model (`database/model/ip_limit_model.go`)

Defines the `InboundClientIPs` model.

### 3. API Endpoints (`web/controller/ip_limit_api.go`)

#### GET `/api/client/ips/:email`
Returns all IPs for a client.

```bash
curl http://localhost:2053/api/client/ips/user@example.com
```

Response:
```json
{
  "msg": "success",
  "ips": [
    "192.168.1.100",
    "203.0.113.45"
  ]
}
```

#### DELETE `/api/client/ips/:email/:ip`
Removes a specific IP.

```bash
curl -X DELETE http://localhost:2053/api/client/ips/user@example.com/192.168.1.100
```

#### DELETE `/api/client/ips/:email`
Clears all IPs for a client.

```bash
curl -X DELETE http://localhost:2053/api/client/ips/user@example.com
```

### 4. Access Service (`web/service/inbound_client_access_service.go`)

Integrated access control:

- **CheckClientAccess(email, ip, limitIP)** - Live IP validation and logging
- **ValidateClientIP(email, ip, limitIP)** - Pure validation
- **GetClientIPList(email)** - Get all registered IPs
- **RemoveClientIP(email, ip)** - Remove specific IP

## Usage Example

### In Login Handler

```go
package controller

import (
    "github.com/gin-gonic/gin"
    "github.com/mhsanaei/3x-ui/v3/web/service"
)

func (a *APIController) Login(ctx *gin.Context) {
    var req LoginRequest
    if err := ctx.ShouldBindJSON(&req); err != nil {
        ctx.AbortWithStatusJSON(400, gin.H{"msg": "Invalid request"})
        return
    }

    // Validate credentials
    client, err := a.getClientByEmail(req.Email)
    if err != nil {
        ctx.AbortWithStatusJSON(401, gin.H{"msg": "Invalid credentials"})
        return
    }

    // Get client IP
    clientIP := ctx.ClientIP()

    // Create inbound service
    inboundSvc := &service.InboundService{}

    // Check IP limit (LIVE INTEGRATION)
    allowed, message, err := inboundSvc.CheckClientAccess(
        client.Email,
        clientIP,
        client.LimitIP, // IP limit from client config
    )

    if !allowed {
        ctx.AbortWithStatusJSON(403, gin.H{
            "msg": message,
            "error": "ip_limit_exceeded",
        })
        return
    }

    // Issue login token
    token := generateToken(client)
    ctx.JSON(200, gin.H{
        "msg": "login_success",
        "token": token,
    })
}
```

### In Middleware

```go
func IPLimitMiddleware() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        clientEmail := ctx.GetString("client_email")
        clientIP := ctx.ClientIP()
        limitIP := ctx.GetInt("limit_ip")

        inboundSvc := &service.InboundService{}
        allowed, _, err := inboundSvc.CheckClientAccess(
            clientEmail,
            clientIP,
            limitIP,
        )

        if !allowed || err != nil {
            ctx.AbortWithStatusJSON(403, gin.H{
                "msg": "Access denied",
            })
            return
        }

        ctx.Next()
    }
}
```

## Configuration

### Client Model Field

In `database/model/client.go`, add:

```go
type Client struct {
    // ... other fields ...
    LimitIP int `json:"limitIP"` // Number of allowed IPs (0 = unlimited)
}
```

## API Routes Registration

Register these routes in your router setup:

```go
package router

import (
    "github.com/gin-gonic/gin"
    "github.com/mhsanaei/3x-ui/v3/web/controller"
)

func SetupRoutes(r *gin.Engine, apiController *controller.APIController) {
    api := r.Group("/api")
    {
        client := api.Group("/client")
        {
            // IP management endpoints
            client.GET("/ips/:email", apiController.GetClientIPs)
            client.DELETE("/ips/:email/:ip", apiController.ClearClientIP)
            client.DELETE("/ips/:email", apiController.ClearAllClientIPs)
        }
    }
}
```

## Database Migration

Call migration in your startup code:

```go
package main

import (
    "github.com/mhsanaei/3x-ui/v3/database"
)

func init() {
    db := database.GetDB()
    database.MigrateIPLimit(db)
}
```

## Live Integration Workflow

```
Client Login Request
    ↓
[Extract] Client IP from request
    ↓
[Validate] Credentials
    ↓
[Check] IP Limit via CheckClientAccess()
    ├─ Database lookup for existing IPs
    ├─ Count unique IPs from last 30 days
    ├─ Compare with limit threshold
    └─ If exceeded: Return 403 error
    ↓
[Record] IP access in database
    ↓
[Issue] Login token
```

## Testing

### Get Client IPs

```bash
curl -X GET http://localhost:2053/api/client/ips/user@example.com
```

### Remove Specific IP

```bash
curl -X DELETE http://localhost:2053/api/client/ips/user@example.com/192.168.1.100
```

### Clear All IPs

```bash
curl -X DELETE http://localhost:2053/api/client/ips/user@example.com
```

### Login Test with IP Check

```bash
curl -X POST http://localhost:2053/api/client/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "secret": "client-secret"
  }'
```

## Security Considerations

1. **IP Spoofing Prevention** - Use `X-Forwarded-For` header carefully in production
2. **Stale IP Cleanup** - Automatically clean up old IPs after 30 days
3. **Rate Limiting** - Implement rate limiting on IP registration
4. **Logging** - Log all IP registration and access attempts
5. **Load Balancer** - Ensure consistent IP detection behind load balancers

## Troubleshooting

### IP Not Being Recorded
- Check if `RecordIPAccess()` is being called
- Verify database permissions
- Check logs for SQL errors

### Limit Not Enforced
- Verify `LimitIP` value in client configuration
- Check if `CheckIPLimit()` is called before granting access
- Ensure database migration ran successfully

### Wrong IP Detected
- Check client IP extraction logic
- Verify load balancer configuration
- Check `X-Forwarded-For` header handling

## Future Enhancements

- IP geolocation tracking
- VPN/Proxy detection
- IP reputation scoring
- Automatic IP whitelisting
- IP-based access policies
- IP anomaly detection
