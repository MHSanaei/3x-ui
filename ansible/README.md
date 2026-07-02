# Ansible deployment for 3x-ui

This repository contains an Ansible playbook and role for installing and configuring 3x-ui on one or more remote Linux servers.

The deployment includes:
- system prerequisites and package installation
- optional PostgreSQL setup or SQLite usage
- 3x-ui binary installation and configuration
- SSL setup (none, domain, IP, or custom certificate)
- systemd service configuration and startup

## Repository structure

- `playbook.yml` – the main Ansible playbook entrypoint
- `inventories/hosts.sample.yml` – sample inventory file
- `roles/install-3x-ui/` – the role that performs the installation

## Requirements

Before running the playbook, make sure you have:
- Ansible installed on your control machine
- SSH access to the target host
- `sudo` privileges on the target host
- Python available on the target host

## Quick start

1. Copy the sample inventory file:
   ```bash
   cp inventories/hosts.sample.yml inventories/hosts.yml
   ```

2. Edit `inventories/hosts.yml` and set your target host details.

3. Review the role defaults in `roles/install-3x-ui/defaults/main.yml` and override any values you need.

4. Run the playbook:
   ```bash
   ansible-playbook -i inventories/hosts.yml playbook.yml
   ```

## Example inventory

```yaml
all:
  children:
    xui_servers:
      hosts:
        server_1:
          ansible_host: 203.0.113.10
          ansible_port: 22
          ansible_user: root
```

## Key variables

The role exposes several variables through `roles/install-3x-ui/defaults/main.yml`:

- `xui_db_type` – database backend (`sqlite` or `postgres`)
- `xui_version` – 3x-ui release version to install
- `xui_username` / `xui_password` – panel login credentials
- `xui_port` – web port for the panel
- `xui_ssl_mode` – SSL mode (`none`, `domain`, `ip`, `custom`)
- `xui_domain` – domain name used when `xui_ssl_mode: domain`
- `xui_listen_ip` – interface the panel should bind to

> For production deployments, change the default credentials and consider protecting secrets with Ansible Vault.

## Useful tags

You can run the playbook with tags to limit the work:

```bash
ansible-playbook -i inventories/hosts.yml playbook.yml --tags setup
ansible-playbook -i inventories/hosts.yml playbook.yml --tags install
```

## Security notes

- Do not leave default passwords in place.
- Use Ansible Vault for sensitive values such as database passwords.
- Review the SSL settings before deploying to production.

