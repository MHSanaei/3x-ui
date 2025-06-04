import { Inbound, ClientSetting } from '@/types/inbound';

const SERVER_ADDRESS = 'YOUR_SERVER_IP_OR_DOMAIN';
const encode = encodeURIComponent;

export function generateSubscriptionLink(inbound: Inbound, client: ClientSetting): string | null {
  if (!inbound || !client) return null;

  const protocol = inbound.protocol;
  const port = inbound.port;
  let remark = client.email || client.id || client.password || 'client';
  if (protocol === 'shadowsocks') {
    remark = inbound.remark || 'shadowsocks_client';
  }

  let streamSettings: Record<string, unknown> = {};
  try {
    streamSettings = inbound.streamSettings ? JSON.parse(inbound.streamSettings) : {};
  } catch (e) { console.error("Error parsing streamSettings for link:", e); }

  const network = (streamSettings.network as string) || 'tcp';
  const security = (streamSettings.security as string) || 'none';

  let tlsSettings: Record<string, unknown> = {};
  if (security === 'tls' && typeof streamSettings.tlsSettings === 'object' && streamSettings.tlsSettings !== null) {
    tlsSettings = streamSettings.tlsSettings as Record<string, unknown>;
  }
  let realitySettings: Record<string, unknown> = {};
  if (security === 'reality' && typeof streamSettings.realitySettings === 'object' && streamSettings.realitySettings !== null) {
     realitySettings = streamSettings.realitySettings as Record<string, unknown>;
  }

  switch (protocol) {
    case 'vmess': {
      const vmessConfig: Record<string, string | undefined> = {
        v: "2", ps: remark, add: SERVER_ADDRESS, port: port.toString(), id: client.id,
        aid: "0", scy: "auto", net: network,
        type: ((streamSettings.tcpSettings as Record<string, unknown>)?.header as Record<string, unknown>)?.type === 'http' ? 'http' : 'none',
        host: ((streamSettings.wsSettings as Record<string, unknown>)?.headers as Record<string, string>)?.Host || (network === 'h2' ? tlsSettings.serverName as string : undefined) || '',
        path: (streamSettings.wsSettings as Record<string, unknown>)?.path as string || (network === 'grpc' ? (streamSettings.grpcSettings as Record<string, unknown>)?.serviceName as string : undefined) || '',
        tls: security === 'tls' ? "tls" : "",
        sni: tlsSettings.serverName as string || '',
      };
      const alpnArray = tlsSettings.alpn as string[];
      if (alpnArray && Array.isArray(alpnArray) && alpnArray.length > 0) {
        vmessConfig.alpn = alpnArray.join(',');
      }
      if (security === 'tls' && tlsSettings.fingerprint) {
        vmessConfig.fp = tlsSettings.fingerprint as string;
      }
      const jsonString = JSON.stringify(vmessConfig);
      return `vmess://${btoa(jsonString)}`;
    }
    case 'vless': {
      let link = `vless://${client.id}@${SERVER_ADDRESS}:${port}`;
      const params = new URLSearchParams();
      params.append('type', network);

      if (security === 'tls' || security === 'reality') {
        params.append('security', security);
        if (tlsSettings.serverName) params.append('sni', tlsSettings.serverName as string);
        if (tlsSettings.fingerprint) params.append('fp', tlsSettings.fingerprint as string);
        if (security === 'reality') {
            if (realitySettings.publicKey) params.append('pbk', realitySettings.publicKey as string);
            if (realitySettings.shortId) params.append('sid', realitySettings.shortId as string);
        }
      }
      if (client.flow) params.append('flow', client.flow);

      if (network === 'ws' && streamSettings.wsSettings) {
        const wsOpts = streamSettings.wsSettings as Record<string, unknown>;
        params.append('path', encode(wsOpts.path as string || '/'));
        if (wsOpts.headers && typeof (wsOpts.headers as Record<string,unknown>).Host === 'string') {
          params.append('host', encode((wsOpts.headers as Record<string,unknown>).Host as string));
        }
      } else if (network === 'grpc' && streamSettings.grpcSettings) {
        const grpcOpts = streamSettings.grpcSettings as Record<string, unknown>;
        params.append('serviceName', encode(grpcOpts.serviceName as string || ''));
        if (grpcOpts.multiMode) params.append('mode', 'multi');
      }

      const query = params.toString();
      if (query) link += `?${query}`;
      link += `#${encode(remark)}`;
      return link;
    }
    case 'trojan': {
      let link = `trojan://${client.password}@${SERVER_ADDRESS}:${port}`;
      const params = new URLSearchParams();
      if (security === 'tls' || security === 'reality') {
        if (tlsSettings.serverName) params.append('sni', tlsSettings.serverName as string);
        if (security === 'reality') params.append('security', 'reality');
      }

      if (network !== 'tcp') params.append('type', network);
      if (network === 'ws' && streamSettings.wsSettings) {
         const wsOpts = streamSettings.wsSettings as Record<string, unknown>;
         params.append('path', encode(wsOpts.path as string || '/'));
         if (wsOpts.headers && typeof (wsOpts.headers as Record<string,unknown>).Host === 'string') {
          params.append('host', encode((wsOpts.headers as Record<string,unknown>).Host as string));
        }
      }
      // For Trojan, client.flow is not typically added to the URL query parameters in standard formats.
      // If a specific Trojan variant uses it, it would be a custom addition.
      // The VLESS case already handles client.flow correctly.

      const query = params.toString();
      if (query) link += `?${query}`;
      link += `#${encode(remark)}`;
      return link;
    }
    case 'shadowsocks': {
      let ssSettings: Record<string, unknown> = {};
      try {
        const parsed = inbound.settings ? JSON.parse(inbound.settings) : {};
        if (typeof parsed.method === 'string' && typeof parsed.password === 'string') {
            ssSettings = parsed as Record<string, unknown>;
        }
      } catch (e) { console.error("Error parsing SS settings for link:", e); }
      const method = ssSettings.method as string || client.encryption;
      const password = ssSettings.password as string;

      if (!method || !password) {
          console.warn("Shadowsocks method or password not found in inbound settings for link generation.");
          return null;
      }

      const encodedPart = btoa(`${method}:${password}`);
      return `ss://${encodedPart}@${SERVER_ADDRESS}:${port}#${encode(remark)}`;
    }
    default:
      return null;
  }
}
