// Pure builders for firewall rules (ufw + nftables) from a port list.

export type PortProtocol = 'tcp' | 'udp' | 'both';

export interface PortRule {
  port: number;
  protocol: PortProtocol;
  label?: string;
}

export interface FirewallOptions {
  ports: PortRule[];
  allowSsh: boolean;
  sshPort: number;
}

function expand(protocol: PortProtocol): ('tcp' | 'udp')[] {
  return protocol === 'both' ? ['tcp', 'udp'] : [protocol];
}

export function buildUfwCommands(o: FirewallOptions): string {
  const lines: string[] = [];
  if (o.allowSsh) lines.push(`ufw allow ${o.sshPort}/tcp   # SSH`);
  for (const rule of o.ports) {
    for (const proto of expand(rule.protocol)) {
      const comment = rule.label ? `   # ${rule.label}` : '';
      lines.push(`ufw allow ${rule.port}/${proto}${comment}`);
    }
  }
  lines.push('ufw enable');
  return lines.join('\n');
}

export function buildNftablesRuleset(o: FirewallOptions): string {
  const accepts: string[] = [];
  if (o.allowSsh) accepts.push(`        tcp dport ${o.sshPort} accept   # SSH`);
  for (const rule of o.ports) {
    for (const proto of expand(rule.protocol)) {
      const comment = rule.label ? `   # ${rule.label}` : '';
      accepts.push(`        ${proto} dport ${rule.port} accept${comment}`);
    }
  }
  return `#!/usr/sbin/nft -f

flush ruleset

table inet filter {
    chain input {
        type filter hook input priority 0; policy drop;

        iif "lo" accept
        ct state established,related accept
        icmp type echo-request accept
${accepts.join('\n')}
    }

    chain forward {
        type filter hook forward priority 0; policy drop;
    }

    chain output {
        type filter hook output priority 0; policy accept;
    }
}`;
}
