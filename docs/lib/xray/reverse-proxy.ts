// Pure builders for reverse-proxy configs (Nginx / Caddy) and cert commands.

export type ProxyServer = 'nginx' | 'caddy';
export type CertTool = 'certbot' | 'acme.sh';

export interface ReverseProxyOptions {
  server: ProxyServer;
  domain: string;
  panelPort: string;
  /** Web base path the panel is served under, e.g. `/panel`. */
  panelPath: string;
  certTool: CertTool;
}

function normalizePath(path: string): string {
  const p = path.trim();
  if (!p) return '/';
  return p.startsWith('/') ? p : `/${p}`;
}

export function buildProxyConfig(o: ReverseProxyOptions): string {
  return o.server === 'nginx' ? buildNginx(o) : buildCaddy(o);
}

function buildNginx(o: ReverseProxyOptions): string {
  const path = normalizePath(o.panelPath);
  const loc = path === '/' ? '/' : `${path.replace(/\/$/, '')}/`;
  return `server {
    listen 443 ssl http2;
    server_name ${o.domain};

    ssl_certificate     /etc/letsencrypt/live/${o.domain}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${o.domain}/privkey.pem;

    location ${loc} {
        proxy_pass http://127.0.0.1:${o.panelPort};
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }
}`;
}

function buildCaddy(o: ReverseProxyOptions): string {
  const path = normalizePath(o.panelPath);
  if (path === '/') {
    return `${o.domain} {
    reverse_proxy 127.0.0.1:${o.panelPort}
}`;
  }
  const matcher = `${path.replace(/\/$/, '')}/*`;
  return `${o.domain} {
    reverse_proxy ${matcher} 127.0.0.1:${o.panelPort}
}`;
}

export function buildCertCommand(o: ReverseProxyOptions): string {
  if (o.certTool === 'certbot') {
    return `certbot certonly --nginx -d ${o.domain}`;
  }
  return `acme.sh --issue -d ${o.domain} --nginx`;
}
