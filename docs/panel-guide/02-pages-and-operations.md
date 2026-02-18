# 02. Pages and Operations

## Overview page (`/panel/`)

Primary purpose:
- Server and Xray health view
- Quick operational actions

Typical actions:
- Restart/stop Xray
- View logs
- Check utilization and throughput

## Inbounds page (`/panel/inbounds`)

Primary purpose:
- Manage listeners and protocol endpoints
- Manage inbound-scoped clients

Core operations:
- Add/edit/delete inbound
- Enable/disable inbound
- Add/edit/delete client
- Bulk add clients
- Reset traffic (client/inbound/global)
- Export URLs/subscription data
- Clone inbound

Operational best practice:
1. Rename/label first.
2. Change one technical setting at a time.
3. Keep one known-good inbound untouched while testing.

## Clients page (`/panel/clients`) - custom extension

Primary purpose:
- Master client profile management
- Multi-inbound assignment orchestration

Core operations:
- Create master client (name, prefix, quota, expiry, ip limit, enabled, comment)
- Assign one or more inbounds
- Update profile and sync assigned inbounds
- Delete master client and remove assignments

## Panel Settings (`/panel/settings`)

Tabs:
- General
- Authentication
- Telegram Bot
- Subscription

Critical controls:
- Listen IP/domain/port
- Base path
- Session duration
- Admin credentials and 2FA

## Xray Configs (`/panel/xray`)

Tabs:
- Basics
- Routing Rules
- Outbounds
- Reverse
- Balancers
- DNS
- Advanced

Safety workflow:
1. Backup/export first.
2. Small changes only.
3. Save and restart Xray when needed.
4. Check logs immediately.

## Frequent operations checklist

- Add new inbound safely:
  1. Create inbound with clear remark and tested config
  2. Add one test client
  3. Verify connectivity
  4. Roll out to users

- Rotate admin credentials:
  1. Change in Authentication tab
  2. Re-login and confirm

- Recover from bad Xray config:
  1. Restore known-good template
  2. Restart Xray
  3. Verify logs/status
