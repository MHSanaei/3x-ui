import type { PanelRestart } from '@/generated/zod';

function isIp(h: string): boolean {
  if (typeof h !== 'string') return false;
  const v4 = h.split('.');
  if (v4.length === 4 && v4.every((p) => /^\d{1,3}$/.test(p) && Number(p) <= 255)) return true;
  if (!h.includes(':') || h.includes(':::')) return false;
  const parts = h.split('::');
  if (parts.length > 2) return false;
  const split = (s: string) => (s ? s.split(':').filter(Boolean) : []);
  const head = split(parts[0]);
  const tail = split(parts[1]);
  const valid = (seg: string) => /^[0-9a-fA-F]{1,4}$/.test(seg);
  if (![...head, ...tail].every(valid)) return false;
  const groups = head.length + tail.length;
  return parts.length === 2 ? groups < 8 : groups === 8;
}

interface RestartUrlSettings {
  webBasePath?: string;
  webDomain?: string;
}

// Scheme and port follow the panel only where the browser's own match the listener that
// answered; a proxy or a published port in front of it keeps the address it was reached on.
export function panelUrlAfterRestart(
  current: string,
  restart: PanelRestart | undefined,
  { webBasePath, webDomain }: RestartUrlSettings,
): string {
  const url = new URL(current);
  if (restart) {
    const pageTls = url.protocol === 'https:';
    const pagePort = Number(url.port) || (pageTls ? 443 : 80);
    if (pageTls === restart.tls) url.protocol = restart.nextTls ? 'https:' : 'http:';
    url.port = String(pagePort === restart.port ? restart.nextPort : pagePort);
  }
  if (isIp(url.hostname.replace(/^\[|\]$/g, '')) && webDomain && isIp(webDomain)) {
    url.hostname = webDomain.includes(':') ? `[${webDomain}]` : webDomain;
  }
  let base = webBasePath ? webBasePath.replace(/^\//, '') : '';
  if (base && !base.endsWith('/')) base += '/';
  url.pathname = `/${base}panel/settings`;
  return url.toString();
}
