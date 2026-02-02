# AI Feature Migration Guide

## For Existing 3X-UI Users

This guide helps you safely upgrade your existing 3X-UI installation to include the new AI-powered Telegram bot feature.

## 📋 Pre-Migration Checklist

### 1. Check Current Version
```bash
/usr/local/x-ui/x-ui -v
```

### 2. Backup Your Database
```bash
# Create backup
cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.backup.$(date +%Y%m%d_%H%M%S)

# Verify backup
ls -lh /etc/x-ui/x-ui.db*
```

### 3. Stop Telegram Bot (Critical!)
```bash
# This prevents 409 bot conflicts
systemctl stop x-ui
```

## 🔄 Migration Steps

### Option 1: Automatic Update (Recommended)

```bash
# 1. Stop service
systemctl stop x-ui

# 2. Run update script (when merged to main branch)
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/update.sh)

# 3. Start service
systemctl start x-ui

# 4. Check logs
tail -f /var/log/x-ui/3xipl.log
```

### Option 2: Manual Update (From Source)

```bash
# 1. Stop service
systemctl stop x-ui

# 2. Backup current installation
cp -r /usr/local/x-ui /usr/local/x-ui.backup

# 3. Pull latest code
cd /usr/local/x-ui/source  # Or your source directory
git fetch origin
git checkout main
git pull origin main

# 4. Build
go mod download
go build -o /usr/local/x-ui/x-ui ./main.go

# 5. Start service
systemctl start x-ui

# 6. Verify
/usr/local/x-ui/x-ui -v
```

### Option 3: From Feature Branch (Testing)

```bash
# For testing the feature before it's merged
cd /usr/local/x-ui/source
git fetch origin
git checkout feat/ai-integration  # Or the branch name
git pull origin feat/ai-integration

go mod download
go build -o /usr/local/x-ui/x-ui ./main.go

systemctl restart x-ui
```

## ⚙️ Post-Migration Configuration

### Enable AI Feature

#### Method 1: Database (Fastest)
```bash
sqlite3 /etc/x-ui/x-ui.db <<'EOF'
-- Enable AI
INSERT OR REPLACE INTO setting (key, value) VALUES ('aiEnabled', 'true');

-- Add your API key (get from https://makersuite.google.com/app/apikey)
INSERT OR REPLACE INTO setting (key, value) VALUES ('aiApiKey', 'YOUR_GEMINI_API_KEY_HERE');

-- Optional: Set max tokens (default: 1024)
INSERT OR REPLACE INTO setting (key, value) VALUES ('aiMaxTokens', '1024');

-- Optional: Set temperature (default: 0.7, range: 0.0-1.0)
INSERT OR REPLACE INTO setting (key, value) VALUES ('aiTemperature', '0.7');
EOF

# Restart to apply
systemctl restart x-ui
```

#### Method 2: Web Panel (Recommended for non-technical users)
1. Login to panel: `http://your-server:2053`
2. Navigate to: **Settings** → **Telegram Bot**
3. Scroll to: **AI Integration** section
4. Toggle: **Enable AI Features** → ON
5. Paste: Your Gemini API key
6. Click: **Save Settings**
7. Click: **Restart Panel** (or restart Telegram bot)

#### Method 3: API Call
```bash
# Get your session token first by logging in
SESSION_TOKEN="your_session_token_here"

# Update AI settings
curl -X POST http://localhost:2053/panel/api/setting/ai/update \
  -H "Content-Type: application/json" \
  -H "Cookie: session=$SESSION_TOKEN" \
  -d '{
    "enabled": true,
    "apiKey": "YOUR_GEMINI_API_KEY",
    "maxTokens": 1024,
    "temperature": 0.7
  }'
```

## ✅ Verification

### Test AI Integration

1. **Check Logs**
```bash
tail -f /var/log/x-ui/3xipl.log | grep -i "ai"

# Expected output:
# [INFO] Telegram Bot: AI service initialized - Enabled: true
# [INFO] AI Service: Gemini client initialized successfully
```

2. **Test in Telegram**
Open your bot and send:
```
show server status
```

Expected response: Server metrics (CPU, memory, traffic, etc.)

3. **Verify Settings**
```bash
sqlite3 /etc/x-ui/x-ui.db "SELECT key, value FROM setting WHERE key LIKE 'ai%';"

# Expected output:
# aiEnabled|true
# aiApiKey|AIza...
# aiMaxTokens|1024
# aiTemperature|0.7
```

## 🔍 Troubleshooting

### Issue: "AI service not enabled"

**Diagnosis:**
```bash
# Check if enabled
sqlite3 /etc/x-ui/x-ui.db "SELECT value FROM setting WHERE key = 'aiEnabled';"

# Check if API key exists
sqlite3 /etc/x-ui/x-ui.db "SELECT length(value) FROM setting WHERE key = 'aiApiKey';"
```

**Solution:**
```bash
# Enable and add API key
sqlite3 /etc/x-ui/x-ui.db <<EOF
UPDATE setting SET value = 'true' WHERE key = 'aiEnabled';
UPDATE setting SET value = 'YOUR_API_KEY' WHERE key = 'aiApiKey';
EOF

systemctl restart x-ui
```

### Issue: "Telegram bot 409 conflict"

**Cause:** Previous bot instance still running

**Solution:**
```bash
# Force stop all instances
pkill -f "x-ui"
sleep 2
systemctl start x-ui
```

### Issue: "Module not found: generative-ai-go"

**Cause:** Dependencies not installed

**Solution:**
```bash
cd /usr/local/x-ui/source
go mod download
go build -o /usr/local/x-ui/x-ui ./main.go
systemctl restart x-ui
```

### Issue: Bot doesn't respond to natural language

**Diagnosis:**
```bash
# Check if AI is actually initialized
tail -f /var/log/x-ui/3xipl.log | grep "AI Service"

# Test with explicit command first
# In Telegram: /status (should work)
# Then: show status (AI should work)
```

**Solution:**
1. Verify you're an admin user in bot settings
2. Check API key validity at [Google AI Studio](https://makersuite.google.com/)
3. Restart bot: `systemctl restart x-ui`

## 🔙 Rollback Procedure

If you encounter issues and want to revert:

### Complete Rollback
```bash
# 1. Stop service
systemctl stop x-ui

# 2. Restore backup
cp /usr/local/x-ui.backup/x-ui /usr/local/x-ui/x-ui

# 3. Restore database (if needed)
cp /etc/x-ui/x-ui.db.backup.* /etc/x-ui/x-ui.db

# 4. Start service
systemctl start x-ui
```

### Disable AI Only (Keep Feature Code)
```bash
# Just disable AI, keep everything else
sqlite3 /etc/x-ui/x-ui.db "UPDATE setting SET value = 'false' WHERE key = 'aiEnabled';"
systemctl restart x-ui
```

## 📊 Database Schema Changes

The migration adds these new settings:

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `aiEnabled` | boolean | `false` | Enable/disable AI |
| `aiApiKey` | string | `""` | Gemini API key |
| `aiMaxTokens` | int | `1024` | Max response tokens |
| `aiTemperature` | float | `0.7` | Response creativity |

**No structural changes** to existing tables - backward compatible!

## 🔐 Security Notes

### API Key Storage
- Stored in SQLite database: `/etc/x-ui/x-ui.db`
- File permissions: `600` (owner read/write only)
- Never logged in plain text
- Not exposed in API responses

### Access Control
- Only users in `tgBotChatId` setting can use AI
- Non-admin messages are ignored
- Rate limited: 20 requests/minute per user

### Data Privacy
- Messages sent to Google Gemini API for processing
- No conversation history stored (except 5-min cache)
- Cache cleared on bot restart
- Consider GDPR/data residency requirements

## 📞 Support

### Get Help
- **GitHub Issues**: [3x-ui/issues](https://github.com/mhsanaei/3x-ui/issues)
- **Telegram Group**: [3X-UI Community](https://t.me/threexui)
- **Documentation**: [AI_INTEGRATION.md](./AI_INTEGRATION.md)

### Report Bugs
When reporting issues, include:
```bash
# System info
uname -a
/usr/local/x-ui/x-ui -v

# Logs
tail -n 100 /var/log/x-ui/3xipl.log

# Configuration
sqlite3 /etc/x-ui/x-ui.db "SELECT key, CASE WHEN key='aiApiKey' THEN '***REDACTED***' ELSE value END FROM setting WHERE key LIKE 'ai%';"
```

## 🎉 Success Indicators

You've successfully migrated when:
- ✅ `systemctl status x-ui` shows "active (running)"
- ✅ Logs show "AI service initialized - Enabled: true"
- ✅ Traditional commands work: `/status`, `/usage`
- ✅ Natural language works: "show server status"
- ✅ Bot responds intelligently with server info

## 📚 Next Steps

After successful migration:
1. Read [AI_QUICKSTART.md](./AI_QUICKSTART.md) for usage examples
2. Explore [AI_INTEGRATION.md](./AI_INTEGRATION.md) for advanced features
3. Join community to share feedback
4. Consider contributing improvements!

---

**Migration Guide Version**: 1.0
**Last Updated**: February 2, 2026
**Status**: Production-Ready ✅
