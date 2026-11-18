# Master panel failover (HA)

- Status: implemented (`internal/ha`, `ha_main.go`)
- Prior art: [telemux](https://github.com/AndreyOsipuk/telemux) ADR 0002 "master failover"
  (`internal/role`, `internal/peers`, `internal/patroni`). Both projects are GPL-3.0; the ported
  code names its source.

## Context

The master panel is the single owner of the configuration: inbounds, clients, nodes and traffic
live in its database, and nodes follow it (`runtime.Remote`, `nodes.config_dirty`, a sync pass
every 5 s in `job/node_traffic_sync_job.go`). When the master machine dies, nodes keep serving
their last configuration, but nobody manages them, counts traffic, disables expired clients or
serves subscriptions until an operator moves the master by hand.

Goal: when the master machine fails or is cut off, another machine takes over on its own and
safely, without split-brain.

### Requirements

1. **No split-brain.** At most one panel writes the database and drives the nodes. A panel that
   cannot prove it is in the majority stops acting as master by itself (fencing).
2. **No lost acknowledged writes.** A change the panel answered `success` to survives a failover.
3. **The new master does not trust the old one's memory.** Everything the old master kept in
   process memory (push fingerprints, caches, queues) is treated as lost; nodes are fully
   reconciled.
4. **No flapping.** A short network blip does not move the master; it never moves back without
   an operator.
5. **Asymmetric partitions are settled by quorum**, not by "I cannot see the master, so it died".
6. **Clients find the new master on their own**: an admin in a browser, API clients and bots,
   subscription users.
7. **User VPN traffic does not depend on where the panel runs.**

## Decision

**Patroni + etcd on three machines** decide where the PostgreSQL primary is (election by etcd
quorum, demotion of an isolated primary, `pg_rewind`, `synchronous_mode: quorum`). The panel runs
no election of its own: **its role is derived from the role of the local PostgreSQL**. On top of
that it keeps a second, independent fencing loop that can only take the master role away, never
grant it.

A panel is master when all of these hold (each check fails closed):

1. the local PostgreSQL is a primary (`pg_is_in_recovery() = false`);
2. the local Patroni considers itself the leader (`GET /patroni`: `role = primary|master`,
   `state = running`);
3. the panel sees a majority of voting machines - itself plus at least one peer - over any
   channel: an HTTP ping of the peer panel, or a live streaming standby in `pg_stat_replication`
   with a fresh `reply_time`;
4. the conditions have held for `master_hold` (master is lost at once, gained slowly). A freshly
   started process skips the hold, so restarting the panel on a healthy primary does not stop it.

Without quorum for longer than `fence_after` (10 s) the node is `fenced` - earlier than Patroni
may promote a replica (`ttl`, 30 s). The invariant `fence_after + interval + probe timeout < ttl`
is checked against Patroni's `GET /config` at startup (warning in the log).

### Three panels, one database

- One database: a Patroni cluster (primary + two replicas). On every machine the panel talks
  **only to the local PostgreSQL** (`XUI_DB_DSN` on `127.0.0.1`). When the panel is master the
  local server is the primary by definition, so no multi-host DSN is needed.
- Everything that defines the master's identity is already in the database and shared by the
  three panels: `panelGuid` (nodes see the same master), the session cookie secret (an admin
  session survives a failover), API tokens, `LoginEpoch`, 2FA, the master mTLS client certificate
  and node CA, node tokens.
- Not in the database and required to match on all three machines: the node-token encryption key
  (`/etc/x-ui/node_token_key.json` or `XUI_NODE_TOKEN_KEY`). Without it a new master cannot
  decrypt `nodes.api_token`; HA mode refuses to start when a token does not decrypt.
- Schema migrations (`InitDB`) run only on the master, at promotion. A non-master never writes
  (on a replica it cannot: `InitDB` issues unconditional `UPDATE`/`DELETE` on every start).

### What a non-master panel does

**Standby redirects; it does not serve read-only.** A read-only panel on a replica would have to
close dozens of write paths that run without an admin action: unconditional statements in
`InitDB`, `AddTraffic` from `GetXrayConfig` (also on `GET getConfigJson`), lazy
`GetPanelGuid`/`GetSecret`/`Ensure*Cert`, subscription writes on every `GET` (`last_sub_fetch`,
HWID), read-like POST endpoints. Each one missed is a 500 on the replica or, worse, a write that
bypasses the master. Redirecting has no such class of bugs.

The process lives in two phases:

- **standby** (from process start until the node is master):
  - the role monitor and the HA endpoint (below) run;
  - `InitDB` is not called; only the panel and subscription port, path and certificate settings
    are read (plain `SELECT`s a replica serves);
  - **not started**: cron jobs (node sync and heartbeat, traffic, resets, IP limits, LDAP, ...),
    Telegram and Discord bots, xray and other child processes, the subscription server;
  - the panel port answers `/panel/api/*` with
    `409 {"error":"not_master","master":...,"master_url":...}` and an `X-3xui-Master` header;
    other `GET`s (browser) get `307` to the same path on the master; with no known master, `503`;
  - the subscription port answers `307` to the same path on the master's subscription server
    (peers publish it in their ping), or `503` with no known master.
- **master**: the whole regular panel - `InitDB`, jobs, bots, xray, subscriptions. The role
  monitor keeps running.

Transitions:

- **standby → master**: once per process. Before jobs start the new master marks **every** node
  `config_dirty` (requirement 3): the first sync pass runs a full `ReconcileNode` per node with
  empty `pushedFP`, rediscovers `adoptedAliases` and rewrites nodes from the database before it
  pulls their traffic snapshot (which also covers the 90 s in-memory tombstones of deleted
  clients that lived only in the old master).
- **master → not master** (fenced, replica, PostgreSQL unreachable): the process refuses writes
  immediately (`GinGuard` answers like a standby), stops and exits with code 3; systemd
  (`Restart=on-failure`) brings it back as a standby. Restarting instead of "stop everything in
  place" is deliberate: it is guaranteed to drop all in-memory state (remote caches, the pending
  xray restart flag, the bot long-poll offset, the login limiter) and needs no per-subsystem
  shutdown code.

No extra database lock against two masters is needed: only the PostgreSQL primary can write and
Patroni keeps one primary; the second loop makes a panel on a primary without quorum step down
before Patroni may promote a replica.

### First install

On an empty database a standby cannot read the port settings: the tables do not exist yet. It
then answers only on the HA endpoint until the first master migrates; from the next restart on
the standby takes the panel and subscription ports as usual.

### HA endpoint

A separate small HTTP server (`XUI_HA_LISTEN`), running in both phases:

| Path             | Access                 | Answer                                                                                                                   |
| ---------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `GET /ha/leader` | open                   | `200 {"master":true,"code":...}` on the master; otherwise `503` with `master`, `master_url`. For load balancers and bots |
| `GET /ha/role`   | open                   | role, reason, quorum, visible peers, master - for diagnostics                                                            |
| `GET /ha/peer`   | `Bearer XUI_HA_SECRET` | `{code, role, panel_url, sub_url}` - the quorum channel and the master hint                                              |

Without the cluster secret nobody can pose as a peer. The open paths expose nothing beyond the
role and public URLs.

### How nodes and clients find the master

- **Nodes** do not look for the master: the master pushes to and polls the nodes. Node tokens,
  the mTLS certificate and `panelGuid` are shared, so a node cannot tell the new master from the
  old one. The only requirement: node firewalls (the node panel port, fail2ban `ignoreip`) admit
  all three machines.
- **API clients and bots**: keep the three panel URLs. Send to the last known master; on
  `409 not_master` follow `master_url`; if a machine is down, try the list with `GET /ha/leader`.
  Requests should be idempotent per client email (`addClient` already answers `Duplicate email`).
- **Admin in a browser**: any of the three panels redirects to the master; the cookie session is
  shared.
- **Subscription users**: the subscription domain points at a front (haproxy, nginx) that picks
  the master by `GET /ha/leader`, or at all three machines - a standby answers `307` to the
  master's subscription. VPN clients follow redirects, but a front with a health check is the
  primary path: it works even when the master's own host name is unreachable for a user.
- **The panel's own Telegram bot** runs on the master only (never in standby), so there are never
  two `getUpdates` with one token (Telegram's 409 Conflict).

### Xray on the panel machines

A panel may have **local inbounds** (`node_id IS NULL`) served by its own xray. In a shared
database "local" means "on whichever machine is master now": after a failover these inbounds
would come up on another IP while user links (`shareAddr`) point to the old one. Besides, a
standby runs no xray and a demoted panel exits.

So **in HA mode the panels host no inbounds of their own.** Inbounds that must run on a panel
machine move to a **regular node on the same machine** - a second x-ui instance in node mode
(own SQLite, own port, own xray) registered with the master as a node. Then:

- user VPN traffic on that machine does not depend on whether it is master and is not
  interrupted by a panel failover (requirement 7); when the machine dies its inbounds die, as
  today;
- traffic, IP limits, resets and reconciliation of these inbounds take the regular node path -
  all in the database, surviving a master change. Local-xray accounting (`StatsLastValues` in
  memory) and the TUIC journal in a local directory are not lost on failover, because panels no
  longer have them;
- HA mode refuses to add an inbound without a node (`success: false` with the reason), and
  refuses to start while any local inbound exists, even a disabled one (it could simply be
  enabled later).

The panel's own xray runs only on the master and only serves internal needs (outbound tests and
the like).

## Scenarios

| Scenario                              | Patroni                                     | Panel                                                                                                         |
| ------------------------------------- | ------------------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| Master machine down                   | Key expires (≤ 30 s), sync replica promoted | New primary becomes master after `master_hold`: `InitDB`, all nodes dirty, full reconcile. Nodes kept serving |
| Master cut off from both peers        | Demotes before the key expires              | `fenced` after `fence_after` (10 s), process exits, restarts as standby                                       |
| Asymmetry: A sees B, not C            | etcd A+B in quorum                          | Quorum holds - no failover                                                                                    |
| 5 s network blip                      | Key renewed on retry                        | `fence_after` not reached - master stays                                                                      |
| etcd down, network fine               | `failsafe_mode`                             | HTTP quorum holds - master stays                                                                              |
| Panel crashed on the master, PG alive | PostgreSQL role unchanged                   | systemd restarts it; a fresh process skips the hold - master at once                                          |
| Old master returns                    | `pg_rewind`, joins as a replica             | `pg_is_in_recovery = true` - standby                                                                          |

## Failures

| Failure                                   | Behaviour                                                                                                                                                                                                                                                                        |
| ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Local PostgreSQL unreachable              | `unknown`, not master                                                                                                                                                                                                                                                            |
| Patroni unreachable                       | `fenced` (fail closed)                                                                                                                                                                                                                                                           |
| Node-token key differs                    | HA mode does not start (error in the log)                                                                                                                                                                                                                                        |
| Local inbounds (even disabled) in HA mode | HA mode does not start (error in the log)                                                                                                                                                                                                                                        |
| Planned upgrade                           | Standbys first (they do not write), then `patronictl switchover` to an upgraded machine, then the former master. Migrations run on the new master at promotion; rolling back the version on a machine that became master on the new schema works as it does today - forward only |

### What does not carry over between machines (and why that is fine)

- `pushedFP`, `remoteIDByTag`, `adoptedAliases`, node client caches - rebuilt by the first
  `ReconcileNode` (all nodes dirty).
- Online clients, speeds, the sub-node tree, metric history - refilled within 5-20 s.
- The login attempt limiter - per process (as on any restart today).
- Multi-step dialogs and buttons of the built-in Telegram bot older than the failover stop working
  (as on any panel restart).
- A pending xray restart (`isNeedXrayRestart`) - the new master starts xray from scratch.

## Configuration

| env                                  | Default | Meaning                                                                                |
| ------------------------------------ | ------- | -------------------------------------------------------------------------------------- |
| `XUI_HA_NODE`                        | empty   | this machine's code (= Patroni member name). Empty - HA off, the panel works as before |
| `XUI_HA_LISTEN`                      | -       | HA endpoint address, e.g. `:2097`                                                      |
| `XUI_HA_PEERS`                       | -       | peers: `b=https://b.example:2097,c=https://c.example:2097`                             |
| `XUI_HA_SECRET`                      | -       | cluster secret of the peer ping                                                        |
| `XUI_HA_PATRONI_URL`                 | -       | local Patroni REST (`http://127.0.0.1:8008`). Empty - condition 2 is not checked       |
| `XUI_HA_PANEL_URL`, `XUI_HA_SUB_URL` | -       | public panel and subscription origins of this machine - for hints and redirects        |
| `XUI_HA_FENCE_AFTER`                 | `10s`   | how long without quorum before `fenced`                                                |
| `XUI_HA_MASTER_HOLD`                 | `5s`    | hold before becoming master                                                            |
| `XUI_HA_INTERVAL`                    | `2s`    | check period                                                                           |

HA mode requires `XUI_DB_TYPE=postgres`. The panel's PostgreSQL role needs `pg_monitor` (to see
`pg_stat_replication`).

## Where in the code

| What                                                  | Where                                                                 |
| ----------------------------------------------------- | --------------------------------------------------------------------- |
| Role automaton, monitor, `Gate` (process transitions) | `internal/ha/role.go`, `internal/ha/monitor.go`                       |
| Patroni client, peer prober, local PostgreSQL probe   | `internal/ha/patroni.go`, `internal/ha/peers.go`, `internal/ha/pg.go` |
| HA endpoint, standby answers, `GinGuard`              | `internal/ha/http.go`                                                 |
| `XUI_HA_*` variables                                  | `internal/ha/env.go`                                                  |
| Process lifecycle                                     | `ha_main.go`, `main.go`                                               |
| Opening the database without writes (`OpenDB`)        | `internal/database/db.go`                                             |
| Master checks and preparation, local inbound refusal  | `internal/web/service/ha_master.go`, `InboundService.AddInbound`      |
| Ports in standby                                      | `web.Server.StartStandby`, `sub.Server.StartStandby`                  |

## Tests

Unit, no database: the role automaton (all condition combinations, hysteresis, forgiven short
quorum losses, fail-closed on PostgreSQL and Patroni errors, quorum 1/3, 2/3, via replication,
single member); the Patroni client; the peer prober (timeout, wrong code, 401); the HA endpoint;
the standby handler; `GinGuard`; the process `Gate`; env parsing.

With a database (`dbtest`): promotion marks every node `config_dirty` and a stale
`ClearNodeDirty` cannot undo it; HA refuses local inbounds and an undecryptable node token;
adding an inbound without a node is refused.

PostgreSQL (`XUI_TEST_PG_DSN`): `OpenDB` works on a read-only server while `InitDB` fails there;
`pg_is_in_recovery` and `pg_stat_replication` probes.

Before production, on a staging cluster: `kill -9` PostgreSQL on the primary, power off the
master machine, isolate the master with iptables, an asymmetric partition, isolate etcd ports
only, stop the panel on the master, return the old primary, a 10 minute flapping network - zero
failovers. In each: nodes keep their configuration and traffic is not double counted.

## Moving a live master panel without downtime (plan)

User VPN downtime is zero at every step (nodes keep their last configuration). Management
(panel, bots, subscriptions) pauses for seconds at step 6.

0. **Upgrade** the master to a build with HA, `XUI_HA_NODE` empty - no behaviour change.
1. **Local inbounds → a node on the same machine.** A second x-ui instance in node mode on the
   master machine (own port, SQLite), registered as a node. Clone the local inbounds onto it with
   the same keys and `shareAddr` (an inbound cannot be moved between the master and a node in
   place), one at a time: disable the local one → enable the clone on the same port; each inbound
   pauses for seconds. Check user links.
2. **Patroni + etcd cluster** on three machines.
3. **Copy the panel database into the cluster without stopping:** logical replication
   (`CREATE PUBLICATION` on the current master → `CREATE SUBSCRIPTION` on the cluster primary)
   until the lag is zero.
4. **Panels on the two other machines**: same version, same `node_token_key.json`, DSN on the
   local PostgreSQL, HA env - they come up as standbys (no master yet - they answer 503).
5. **Node firewalls**: admit the two new machines.
6. **Switch**: stop x-ui on the old master (management pause starts), wait for zero lag, drop the
   subscription, fix sequences (`setval`); the panel on the cluster primary starts as master (no
   hold on start), the others as standbys. The pause lasts one restart.
7. **Clients**: the panel and subscription front health-checks `/ha/leader` on the three
   machines; bots get the list of three URLs and handle `409 not_master`.
8. **Planned `patronictl switchover`** at a quiet hour and back - check that the admin UI, bots,
   subscriptions and nodes follow on their own.

Rollback before step 6: nothing has been switched, the old master works as before. After step 6:
stop the HA panels, point the old master's DSN back at its original database (left untouched),
and carry writes made after step 6 back by reverse logical replication or by hand.

## Consequences

- Two more services (etcd, Patroni) on three machines.
- A panel failover takes 10-40 s without an operator; nodes and VPN keep working meanwhile.
- In HA mode the panels host no inbounds of their own.
- Commit latency grows by the RTT to the nearest replica (`synchronous_mode: quorum`).
