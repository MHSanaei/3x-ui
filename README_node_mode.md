# Installing 3x-ui with Multi-Node Support (Beta)

This guide describes the complete process of installing the panel and nodes from scratch.

------------------------------------------------------------------------

## Requirements

Before starting, make sure you have installed:
- Docker
- Docker Compose (v2)

Check:

```bash
docker --version
docker compose version
```

------------------------------------------------------------------------

## Step 1. Clone the Repository

```bash
git clone https://github.com/konstpic/3x-ui-dev-beta.git
cd 3x-ui-dev-beta
```

------------------------------------------------------------------------

## Step 2. Switch to the Multi-Node Support Branch

```bash
git checkout 3x-new
```

------------------------------------------------------------------------

## Step 3. Launch the Panel (Core)

In the repository root, build and start the panel:

```bash
docker compose build
docker compose up -d
```

------------------------------------------------------------------------

### (Optional) Configure Panel Ports

By default, `network_mode: host` may be used.

If you want to use standard port mapping:
1. Open `docker-compose.yml` in the project root
2. Remove `network_mode: host`
3. Add port mapping:

```yaml
ports:
  - "2053:2053"   # Web UI
  - "2096:2096"   # Subscriptions
```

After making changes, restart the containers:

```bash
docker compose down
docker compose up -d
```

------------------------------------------------------------------------

## Step 4. Launch the Node

Navigate to the `node` folder:

```bash
cd node
docker compose build
docker compose up -d
```

------------------------------------------------------------------------

## Important ❗ About Node Network and Ports (Xray)

Nodes use the **Xray** core, and it's the nodes that accept user connections to **Inbounds**.

### Option 1 (recommended): `network_mode: host`

Use `network_mode: host` if:
- you don't want to manually manage ports
- you plan to use different inbound ports
- you want behavior as close as possible to bare-metal

In this case, **no additional port mapping is required**.

------------------------------------------------------------------------

### Option 2: Using `ports` (without host network)

If `network_mode: host` is **not used**, you need to:

1. Define in advance the ports on which users will connect to inbounds
2. Map these ports in the node's `docker-compose.yml`

Example:

```yaml
ports:
  - "8080:8080"     # Node API
  - "443:443"       # Inbound (example)
  - "8443:8443"     # Inbound (example)
```

⚠️ In this mode, **each inbound port must be explicitly mapped**.

------------------------------------------------------------------------

### Node API Port

Regardless of the chosen mode:
- The node API runs on port **8080**
- This port must be accessible to the panel

------------------------------------------------------------------------

## Step 5. Enable Multi-Node Mode

1. Open the panel web interface
2. Go to **Panel Settings**
3. Enable **Multi-Node**
4. Save settings

After this, the **Nodes** section will appear.

------------------------------------------------------------------------

## Step 6. Register Nodes

1. Go to the **Nodes** section
2. Add a new node, specifying:
   - node server address
   - node API port: **8080**
3. Save

If everything is configured correctly, the node will appear with **Online** status.

------------------------------------------------------------------------

## Step 7. Using Nodes in Inbounds

After registration:
- nodes can be selected when creating and editing **Inbounds**
- user connections will be accepted **by nodes, not by the panel**

------------------------------------------------------------------------

## Possible Issues

If a node has `Offline` status or users cannot connect:
- make sure the containers are running
- check accessibility of port **8080**
- check that inbound ports are:
  - mapped (if host network is not used)
  - not blocked by firewall
- check Docker network settings

------------------------------------------------------------------------

⚠️ The project is in **beta stage**. Bugs and changes are possible.
