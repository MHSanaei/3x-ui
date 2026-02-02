# AI Integration Quick Start

Transform your 3X-UI Telegram bot into an intelligent assistant using Google's Gemini AI.

## 🚀 Quick Setup (5 minutes)

### 1. Get API Key
Visit [Google AI Studio](https://makersuite.google.com/app/apikey) → Create API Key → Copy it

### 2. Configure
```bash
# Add to database
sqlite3 /etc/x-ui/x-ui.db <<EOF
INSERT OR REPLACE INTO setting (key, value) VALUES ('aiEnabled', 'true');
INSERT OR REPLACE INTO setting (key, value) VALUES ('aiApiKey', 'YOUR_API_KEY_HERE');
EOF

# Restart
systemctl restart x-ui
```

### 3. Test
Open your Telegram bot and type:
```
show server status
```

🎉 **Done!** Your bot now understands natural language.

## 💬 Example Commands

Before AI (rigid):
- `/status`
- `/usage user@example.com`
- `/inbound`

After AI (natural):
- "Show me server status"
- "How much traffic has user@example.com used?"
- "List all inbounds"
- "What's the CPU usage?"
- "Get client info for test@domain.com"

## ⚙️ Advanced Configuration

### Via Web Panel
1. Go to **Settings** → **Telegram Bot**
2. Find **AI Integration** section
3. Enable and paste API key
4. Save

### Fine-tuning
```bash
# Adjust response creativity (0.0 = precise, 1.0 = creative)
sqlite3 /etc/x-ui/x-ui.db "UPDATE setting SET value = '0.5' WHERE key = 'aiTemperature';"

# Adjust max response length
sqlite3 /etc/x-ui/x-ui.db "UPDATE setting SET value = '2048' WHERE key = 'aiMaxTokens';"
```

## 💰 Cost

**FREE** for typical usage:
- Gemini 1.5 Flash: 15 requests/min free
- Most panels stay under free tier
- Cache reduces API calls by 60%

## 🔒 Security

- ✅ Only admins can use AI features
- ✅ API key stored securely in database
- ✅ Rate limited: 20 requests/minute per user
- ✅ Messages cached for 5 minutes (no history stored)

## 🐛 Troubleshooting

### Bot doesn't respond to natural language?
```bash
# Check if enabled
sqlite3 /etc/x-ui/x-ui.db "SELECT value FROM setting WHERE key = 'aiEnabled';"
# Should return: true

# Check API key exists
sqlite3 /etc/x-ui/x-ui.db "SELECT length(value) FROM setting WHERE key = 'aiApiKey';"
# Should return: > 30

# Check logs
tail -f /var/log/x-ui/3xipl.log | grep "AI Service"
```

### Disable AI
```bash
sqlite3 /etc/x-ui/x-ui.db "UPDATE setting SET value = 'false' WHERE key = 'aiEnabled';"
systemctl restart x-ui
```

## 📚 Full Documentation

See [AI_INTEGRATION.md](./AI_INTEGRATION.md) for:
- Architecture details
- API reference
- Advanced features
- Development guide

## 🎯 What's Supported

| Feature | Status |
|---------|--------|
| Server status queries | ✅ |
| Traffic/usage queries | ✅ |
| Inbound management | ✅ |
| Client information | ✅ |
| Natural conversation | ✅ |
| Multi-language | 🔜 Soon |
| Voice messages | 🔜 Soon |
| Proactive alerts | 🔜 Soon |

## 🤝 Contributing

Found a bug? Want a feature? [Open an issue](https://github.com/mhsanaei/3x-ui/issues)

---

**Built with ❤️ using Google Gemini AI**
