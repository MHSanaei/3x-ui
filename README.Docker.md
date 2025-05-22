# 🐳 3X-UI Docker Setup with PostgreSQL Support

This guide explains how to run 3X-UI with Docker, supporting both SQLite (default) and PostgreSQL databases.

## 📋 Quick Start

### Option 1: SQLite (Default, Recommended for most users)

```bash
# Clone the repository
git clone https://github.com/MHSanaei/3x-ui.git
cd 3x-ui

# Start with SQLite (default)
docker-compose up -d
```

### Option 2: PostgreSQL (Production setup)

```bash
# Clone the repository
git clone https://github.com/MHSanaei/3x-ui.git
cd 3x-ui

# Copy environment file and configure
cp env.example .env
# Edit .env file with your PostgreSQL settings

# Start with PostgreSQL
docker-compose -f docker-compose.postgresql.yml up -d
```

## 🔧 Configuration

### Environment Variables

Copy `env.example` to `.env` and modify according to your needs:

```bash
cp env.example .env
```

Key variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_TYPE` | `sqlite` | Database type: `sqlite` or `postgres` |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `x_ui` | Database name |
| `DB_USER` | `x_ui` | Database user |
| `DB_PASSWORD` | - | Database password (required for PostgreSQL) |
| `XUI_PORT` | `2053` | 3X-UI web interface port |
| `XUI_SUB_PORT` | `2096` | Subscription port |

### Database Types

#### SQLite (Default)
- **Pros**: Simple setup, no additional containers, good for small-medium loads
- **Cons**: Limited concurrent connections, single file storage
- **Use case**: Personal use, small teams, development

#### PostgreSQL
- **Pros**: High performance, concurrent connections, ACID compliance, scalability
- **Cons**: More complex setup, additional container
- **Use case**: Production environments, high load, multiple users

## 🚀 Deployment Options

### 1. SQLite Deployment

```bash
# Basic SQLite setup
docker-compose up -d

# Check logs
docker-compose logs -f 3x-ui
```

### 2. PostgreSQL Deployment

```bash
# Set environment variables
export DB_PASSWORD="your_secure_password_here"

# Start PostgreSQL setup
docker-compose -f docker-compose.postgresql.yml up -d

# Check all services
docker-compose -f docker-compose.postgresql.yml ps
```

### 3. PostgreSQL with Admin Interface

```bash
# Start with PgAdmin for database management
docker-compose -f docker-compose.postgresql.yml --profile admin up -d

# Access PgAdmin at http://localhost:5050
# Default login: admin@example.com / admin_password
```

### 4. External PostgreSQL

```bash
# Configure .env for external database
cat > .env << EOF
DB_TYPE=postgres
DB_HOST=your-postgres-server.com
DB_PORT=5432
DB_NAME=x_ui_production
DB_USER=x_ui_user
DB_PASSWORD=your_external_db_password
DB_SSLMODE=require
EOF

# Start only 3X-UI container
docker-compose up -d
```

## 🔍 Monitoring and Maintenance

### Health Checks

All services include health checks:

```bash
# Check service health
docker-compose ps

# View health check logs
docker inspect --format='{{json .State.Health}}' 3x-ui
```

### Logs

```bash
# View 3X-UI logs
docker-compose logs -f 3x-ui

# View PostgreSQL logs
docker-compose -f docker-compose.postgresql.yml logs -f postgres

# View all logs
docker-compose -f docker-compose.postgresql.yml logs -f
```

### Database Management

#### Backup PostgreSQL

```bash
# Create backup
docker-compose -f docker-compose.postgresql.yml exec postgres pg_dump -U x_ui x_ui > backup.sql

# Restore backup
docker-compose -f docker-compose.postgresql.yml exec -T postgres psql -U x_ui x_ui < backup.sql
```

#### Backup SQLite

```bash
# SQLite backup
docker-compose exec 3x-ui cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.backup
docker cp 3x-ui:/etc/x-ui/x-ui.db.backup ./x-ui-backup.db
```

## 🔧 Troubleshooting

### Common Issues

#### PostgreSQL Connection Failed
```bash
# Check PostgreSQL is running
docker-compose -f docker-compose.postgresql.yml ps postgres

# Check PostgreSQL logs
docker-compose -f docker-compose.postgresql.yml logs postgres

# Test connection manually
docker-compose -f docker-compose.postgresql.yml exec postgres psql -U x_ui -d x_ui -c "SELECT 1;"
```

#### 3X-UI Won't Start
```bash
# Check 3X-UI logs
docker-compose logs 3x-ui

# Check database environment
docker-compose exec 3x-ui cat /etc/x-ui/db.env

# Restart services
docker-compose restart
```

#### Port Conflicts
```bash
# Change ports in .env file
echo "XUI_PORT=3053" >> .env
echo "XUI_SUB_PORT=3096" >> .env

# Restart with new ports
docker-compose up -d
```

### Performance Tuning

#### PostgreSQL Optimization

The PostgreSQL container is pre-configured with optimized settings for 3X-UI workload:

- `max_connections=200`
- `shared_buffers=256MB`
- `effective_cache_size=1GB`
- `work_mem=4MB`

For high-load environments, consider:

1. **Increase resources**:
   ```yaml
   deploy:
     resources:
       limits:
         memory: 2G
         cpus: '1.0'
   ```

2. **Use external PostgreSQL** with dedicated hardware

3. **Enable connection pooling** (PgBouncer)

## 🔒 Security Considerations

### Production Checklist

- [ ] Change default passwords in `.env`
- [ ] Use strong, unique passwords
- [ ] Enable SSL for PostgreSQL (`DB_SSLMODE=require`)
- [ ] Restrict network access (firewall rules)
- [ ] Regular backups
- [ ] Monitor logs for suspicious activity
- [ ] Keep containers updated

### Network Security

```yaml
# Example: Restrict PostgreSQL access
services:
  postgres:
    ports:
      - "127.0.0.1:5432:5432"  # Only localhost access
```

## 📊 Monitoring

### Prometheus Metrics (Optional)

Add monitoring stack:

```yaml
# Add to docker-compose.postgresql.yml
  prometheus:
    image: prom/prometheus
    ports:
      - "9090:9090"
    
  grafana:
    image: grafana/grafana
    ports:
      - "3000:3000"
```

## 🔄 Migration

### SQLite to PostgreSQL

1. **Backup SQLite data**:
   ```bash
   docker-compose exec 3x-ui cp /etc/x-ui/x-ui.db /etc/x-ui/backup.db
   ```

2. **Export data** (manual process - depends on your data structure)

3. **Switch to PostgreSQL**:
   ```bash
   # Update .env
   echo "DB_TYPE=postgres" >> .env
   
   # Start PostgreSQL
   docker-compose -f docker-compose.postgresql.yml up -d
   ```

4. **Import data** (application will create tables automatically)

### PostgreSQL to SQLite

1. **Export PostgreSQL data**
2. **Switch to SQLite** in `.env`
3. **Import data**

## 📞 Support

- **Issues**: [GitHub Issues](https://github.com/MHSanaei/3x-ui/issues)
- **Documentation**: [Main README](README.md)
- **Community**: [Telegram](https://t.me/x_ui_channel)

## 📝 Examples

### Development Setup

```bash
# Quick development setup with SQLite
git clone https://github.com/MHSanaei/3x-ui.git
cd 3x-ui
docker-compose up -d
```

### Production Setup

```bash
# Production setup with PostgreSQL
git clone https://github.com/MHSanaei/3x-ui.git
cd 3x-ui

# Configure environment
cat > .env << EOF
DB_TYPE=postgres
DB_PASSWORD=$(openssl rand -base64 32)
XUI_PORT=2053
XUI_SUB_PORT=2096
HOSTNAME=$(hostname)
EOF

# Deploy
docker-compose -f docker-compose.postgresql.yml up -d

# Verify
docker-compose -f docker-compose.postgresql.yml ps
```

### High Availability Setup

For HA setups, consider:
- External PostgreSQL cluster
- Load balancer for 3X-UI instances
- Shared storage for certificates
- Monitoring and alerting 