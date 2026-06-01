import { describe, expect, it } from 'vitest';

import {
  parseOutboundLink,
  parseShadowsocksLink,
  parseTrojanLink,
  parseVlessLink,
  parseVmessLink,
  parseHysteria2Link,
  parseWireguardLink,
} from '@/lib/xray/outbound-link-parser';
import { Base64 } from '@/utils';

// Focused acceptance tests for the share-link parsers — one happy-path
// case per protocol family, plus a few common edge cases. The parsers
// produce wire-shape outbound rows; the modal hands them to
// rawOutboundToFormValues to seed Form.useForm.

describe('parseVmessLink', () => {
  it('parses a vmess:// link with ws + tls', () => {
    const json = {
      v: '2', ps: 'imported-vmess', add: '1.2.3.4', port: 8443,
      id: '11111111-2222-4333-8444-555555555555', aid: 0, scy: 'auto',
      net: 'ws', host: 'example.com', path: '/ws',
      tls: 'tls', sni: 'example.com', fp: 'chrome', alpn: 'h2,http/1.1',
    };
    const link = `vmess://${Base64.encode(JSON.stringify(json))}`;
    const out = parseVmessLink(link);
    expect(out).not.toBeNull();
    expect(out?.protocol).toBe('vmess');
    expect(out?.tag).toBe('imported-vmess');
    const settings = out?.settings as { vnext: Array<{ address: string; port: number; users: Array<{ id: string; security: string }> }> };
    expect(settings.vnext[0].address).toBe('1.2.3.4');
    expect(settings.vnext[0].port).toBe(8443);
    expect(settings.vnext[0].users[0].id).toBe('11111111-2222-4333-8444-555555555555');
    const stream = out?.streamSettings as Record<string, unknown>;
    expect(stream.network).toBe('ws');
    expect(stream.security).toBe('tls');
    expect((stream.wsSettings as Record<string, unknown>).path).toBe('/ws');
    expect((stream.tlsSettings as Record<string, unknown>).serverName).toBe('example.com');
    expect((stream.tlsSettings as Record<string, unknown>).alpn).toEqual(['h2', 'http/1.1']);
  });

  it('returns null for non-vmess links', () => {
    expect(parseVmessLink('vless://x@y:1')).toBeNull();
  });

  it('returns null for malformed base64', () => {
    expect(parseVmessLink('vmess://!!!not-base64!!!')).toBeNull();
  });
});

describe('parseVmessLink — XHTTP advanced fields', () => {
  it('round-trips xhttp knobs from the vmess JSON', () => {
    const json = {
      v: '2', ps: 'imported-xhttp', add: '1.2.3.4', port: 443,
      id: '11111111-2222-4333-8444-555555555555', aid: 0, scy: 'auto',
      net: 'xhttp', host: 'edge.example', path: '/sp', mode: 'stream-up',
      xPaddingBytes: '500-1500',
      scMaxEachPostBytes: '2000000',
      scMinPostsIntervalMs: '60',
      uplinkChunkSize: 8192,
      noGRPCHeader: true,
      tls: 'tls', sni: 'edge.example',
    };
    const link = `vmess://${Base64.encode(JSON.stringify(json))}`;
    const out = parseVmessLink(link);
    const stream = out?.streamSettings as Record<string, unknown>;
    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    expect(xhttp.host).toBe('edge.example');
    expect(xhttp.path).toBe('/sp');
    expect(xhttp.mode).toBe('stream-up');
    expect(xhttp.xPaddingBytes).toBe('500-1500');
    expect(xhttp.scMaxEachPostBytes).toBe('2000000');
    expect(xhttp.scMinPostsIntervalMs).toBe('60');
    expect(xhttp.uplinkChunkSize).toBe(8192);
    expect(xhttp.noGRPCHeader).toBe(true);
  });

  it('round-trips xhttp padding-obfs knobs from the vmess JSON', () => {
    const json = {
      v: '2', ps: 'imported-pad', add: '1.2.3.4', port: 443,
      id: '11111111-2222-4333-8444-555555555555', aid: 0, scy: 'auto',
      net: 'xhttp', host: 'edge.example', path: '/sp',
      xPaddingObfsMode: true,
      xPaddingKey: 'secret-key',
      xPaddingHeader: 'X-Pad',
      xPaddingPlacement: 'header',
      xPaddingMethod: 'random',
      sessionKey: 'X-Session',
      seqKey: 'X-Seq',
      noSSEHeader: true,
      scMaxBufferedPosts: 50,
      tls: 'tls',
    };
    const link = `vmess://${Base64.encode(JSON.stringify(json))}`;
    const out = parseVmessLink(link);
    const xhttp = (out?.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    expect(xhttp.xPaddingObfsMode).toBe(true);
    expect(xhttp.xPaddingKey).toBe('secret-key');
    expect(xhttp.xPaddingHeader).toBe('X-Pad');
    expect(xhttp.xPaddingPlacement).toBe('header');
    expect(xhttp.xPaddingMethod).toBe('random');
    expect(xhttp.sessionKey).toBe('X-Session');
    expect(xhttp.seqKey).toBe('X-Seq');
    expect(xhttp.noSSEHeader).toBe(true);
    expect(xhttp.scMaxBufferedPosts).toBe(50);
  });
});

describe('parseVlessLink — XHTTP advanced fields', () => {
  it('round-trips xhttp knobs from URL query params', () => {
    const link
      = 'vless://uuid@srv.example:443'
      + '?type=xhttp&security=tls&host=edge.example&path=%2Fsp&mode=stream-up'
      + '&xPaddingBytes=500-1500&scMaxEachPostBytes=2000000'
      + '&scMinPostsIntervalMs=60&uplinkChunkSize=8192&noGRPCHeader=true'
      + '#imported-xhttp';
    const out = parseVlessLink(link);
    const stream = out?.streamSettings as Record<string, unknown>;
    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    expect(xhttp.host).toBe('edge.example');
    expect(xhttp.path).toBe('/sp');
    expect(xhttp.mode).toBe('stream-up');
    expect(xhttp.xPaddingBytes).toBe('500-1500');
    expect(xhttp.scMaxEachPostBytes).toBe('2000000');
    expect(xhttp.scMinPostsIntervalMs).toBe('60');
    expect(xhttp.uplinkChunkSize).toBe(8192);
    expect(xhttp.noGRPCHeader).toBe(true);
  });

  it('round-trips xhttp padding-obfs knobs from URL query params', () => {
    const link
      = 'vless://uuid@srv.example:443'
      + '?type=xhttp&security=tls&host=edge.example&path=%2Fsp'
      + '&xPaddingObfsMode=true&xPaddingKey=secret-key&xPaddingHeader=X-Pad'
      + '&xPaddingPlacement=header&xPaddingMethod=random'
      + '&sessionKey=X-Session&seqKey=X-Seq&noSSEHeader=true'
      + '&scMaxBufferedPosts=50'
      + '#imported-pad';
    const out = parseVlessLink(link);
    const xhttp = (out?.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    expect(xhttp.xPaddingObfsMode).toBe(true);
    expect(xhttp.xPaddingKey).toBe('secret-key');
    expect(xhttp.xPaddingHeader).toBe('X-Pad');
    expect(xhttp.xPaddingPlacement).toBe('header');
    expect(xhttp.xPaddingMethod).toBe('random');
    expect(xhttp.sessionKey).toBe('X-Session');
    expect(xhttp.seqKey).toBe('X-Seq');
    expect(xhttp.noSSEHeader).toBe(true);
    expect(xhttp.scMaxBufferedPosts).toBe(50);
  });
});

describe('parseVlessLink', () => {
  it('parses a vless:// link with reality', () => {
    const link
      = 'vless://11111111-2222-4333-8444-555555555555@srv.example:443'
      + '?type=tcp&security=reality&pbk=pubkey&sid=abcd&fp=chrome&sni=cloudflare.com&flow=xtls-rprx-vision'
      + '#imported-vless';
    const out = parseVlessLink(link);
    expect(out?.protocol).toBe('vless');
    expect(out?.tag).toBe('imported-vless');
    const settings = out?.settings as { id: string; flow: string; address: string; port: number };
    expect(settings.id).toBe('11111111-2222-4333-8444-555555555555');
    expect(settings.address).toBe('srv.example');
    expect(settings.port).toBe(443);
    expect(settings.flow).toBe('xtls-rprx-vision');
    const stream = out?.streamSettings as Record<string, unknown>;
    expect(stream.security).toBe('reality');
    const reality = stream.realitySettings as Record<string, unknown>;
    expect(reality.publicKey).toBe('pubkey');
    expect(reality.shortId).toBe('abcd');
    expect(reality.serverName).toBe('cloudflare.com');
  });
});

describe('parseTrojanLink', () => {
  it('parses a trojan:// link with ws + tls', () => {
    const link = 'trojan://secret-pw@srv.example:8443?type=ws&security=tls&host=example.com&path=/tj&sni=example.com#imported-trojan';
    const out = parseTrojanLink(link);
    expect(out?.protocol).toBe('trojan');
    const settings = out?.settings as { servers: Array<{ address: string; port: number; password: string }> };
    expect(settings.servers[0].address).toBe('srv.example');
    expect(settings.servers[0].port).toBe(8443);
    expect(settings.servers[0].password).toBe('secret-pw');
    const stream = out?.streamSettings as Record<string, unknown>;
    expect(stream.network).toBe('ws');
    expect((stream.wsSettings as Record<string, unknown>).path).toBe('/tj');
  });
});

describe('parseShadowsocksLink', () => {
  it('parses the modern userinfo@host:port form', () => {
    // ss://base64(method:password)@host:port#remark
    const userinfo = Base64.encode('2022-blake3-aes-128-gcm:supersecret');
    const link = `ss://${userinfo}@1.2.3.4:8388#imported-ss`;
    const out = parseShadowsocksLink(link);
    expect(out?.protocol).toBe('shadowsocks');
    expect(out?.tag).toBe('imported-ss');
    const settings = out?.settings as { servers: Array<{ address: string; port: number; method: string; password: string }> };
    expect(settings.servers[0].address).toBe('1.2.3.4');
    expect(settings.servers[0].port).toBe(8388);
    expect(settings.servers[0].method).toBe('2022-blake3-aes-128-gcm');
    expect(settings.servers[0].password).toBe('supersecret');
  });

  it('keeps the port when the link carries a query string (2022 two-key password)', () => {
    const link = 'ss://MjAyMi1ibGFrZTMtYWVzLTI1Ni1nY206LzhsdFZKaU90azE2QmhKZG9WZVRmSkNNUEJlRGhjcmkycTN0dzU1OUZvYz06YUhuTTB6ZnpFaTdRejc5dzlxNWFFWWVQVnpDU0wxaHV4RnZXZFB6OFZHST0@localhost:30757?type=tcp#pahf4urt53';
    const out = parseShadowsocksLink(link);
    expect(out?.protocol).toBe('shadowsocks');
    expect(out?.tag).toBe('pahf4urt53');
    const settings = out?.settings as { servers: Array<{ address: string; port: number; method: string; password: string }> };
    expect(settings.servers[0].address).toBe('localhost');
    expect(settings.servers[0].port).toBe(30757);
    expect(settings.servers[0].method).toBe('2022-blake3-aes-256-gcm');
    expect(settings.servers[0].password).toBe('/8ltVJiOtk16BhJdoVeTfJCMPBeDhcri2q3tw559Foc=:aHnM0zfzEi7Qz79w9q5aEYePVzCSL1huxFvWdPz8VGI=');
  });

  it('parses the legacy base64-of-whole form', () => {
    // ss://base64(method:password@host:port)#remark
    const inner = Base64.encode('aes-256-gcm:legacypw@10.0.0.1:1080');
    const link = `ss://${inner}#imported-legacy`;
    const out = parseShadowsocksLink(link);
    const settings = out?.settings as { servers: Array<{ address: string; port: number; method: string; password: string }> };
    expect(settings.servers[0].address).toBe('10.0.0.1');
    expect(settings.servers[0].port).toBe(1080);
    expect(settings.servers[0].method).toBe('aes-256-gcm');
    expect(settings.servers[0].password).toBe('legacypw');
  });
});

describe('parseHysteria2Link', () => {
  it('parses a hysteria2:// link with sni', () => {
    const link = 'hysteria2://auth-secret@srv.example:443?sni=example.com#imported-hy2';
    const out = parseHysteria2Link(link);
    expect(out?.protocol).toBe('hysteria');
    expect(out?.tag).toBe('imported-hy2');
    const settings = out?.settings as { address: string; port: number; version: number };
    expect(settings.address).toBe('srv.example');
    expect(settings.port).toBe(443);
    expect(settings.version).toBe(2);
    const stream = out?.streamSettings as Record<string, unknown>;
    const hys = stream.hysteriaSettings as Record<string, unknown>;
    expect(hys.auth).toBe('auth-secret');
    expect((stream.tlsSettings as Record<string, unknown>).serverName).toBe('example.com');
  });

  it('also accepts hy2:// alias', () => {
    const out = parseHysteria2Link('hy2://auth@srv:443?sni=example.com');
    expect(out?.protocol).toBe('hysteria');
  });
});

describe('parseVlessLink — extra / fm / x_padding_bytes (B20)', () => {
  it('round-trips a real inbound-generated link with extra+fm+reality+xhttp', () => {
    // Real user-reported link — bundled xhttp knobs via `extra` JSON,
    // full finalmask via `fm` JSON, reality auth, snake_case
    // x_padding_bytes alias. All three parse-paths must combine.
    const link = 'vless://b622ac2f-f155-47db-a3b2-b64e8d7f6342@localhost:37723?'
      + 'encryption=none&'
      + 'extra=%7B%22scMaxEachPostBytes%22%3A%221000000%22%2C%22scMinPostsIntervalMs%22%3A%2230%22%2C%22xPaddingBytes%22%3A%22100-1000%22%7D&'
      + 'fm=%7B%22quicParams%22%3A%7B%22congestion%22%3A%22bbr%22%2C%22maxIdleTimeout%22%3A30%2C%22udpHop%22%3A%7B%22interval%22%3A%225-10%22%2C%22ports%22%3A%2220000-50000%22%7D%7D%7D&'
      + 'fp=chrome&host=&mode=auto&path=%2F&'
      + 'pbk=nJw4k4CPf5jf64V8nnDwWa8iClDnUvQ1lCI4iKzfJ0o&'
      + 'security=reality&sid=14ebccc4d3&sni=aws.amazon.com&'
      + 'spx=%2F97L2FjycXEwrE67&type=xhttp&x_padding_bytes=100-1000'
      + '#sda-8ud3us6rt';
    const parsed = parseVlessLink(link);
    expect(parsed).not.toBeNull();
    expect(parsed!.tag).toBe('sda-8ud3us6rt');

    const stream = parsed!.streamSettings as Record<string, unknown>;
    expect(stream.network).toBe('xhttp');
    expect(stream.security).toBe('reality');

    const xhttp = stream.xhttpSettings as Record<string, unknown>;
    expect(xhttp.xPaddingBytes).toBe('100-1000');
    expect(xhttp.scMaxEachPostBytes).toBe('1000000');
    expect(xhttp.scMinPostsIntervalMs).toBe('30');

    const reality = stream.realitySettings as Record<string, unknown>;
    expect(reality.publicKey).toBe('nJw4k4CPf5jf64V8nnDwWa8iClDnUvQ1lCI4iKzfJ0o');
    expect(reality.shortId).toBe('14ebccc4d3');
    expect(reality.spiderX).toBe('/97L2FjycXEwrE67');
    expect(reality.serverName).toBe('aws.amazon.com');

    const finalmask = stream.finalmask as Record<string, unknown>;
    expect(finalmask).toBeDefined();
    const quicParams = finalmask.quicParams as Record<string, unknown>;
    expect(quicParams.congestion).toBe('bbr');
    expect(quicParams.maxIdleTimeout).toBe(30);
    expect((quicParams.udpHop as Record<string, unknown>).interval).toBe('5-10');
    expect((quicParams.udpHop as Record<string, unknown>).ports).toBe('20000-50000');
  });

  it('falls back to x_padding_bytes when extra has no xPaddingBytes', () => {
    const link = 'vless://u@h:1?type=xhttp&security=none&path=%2F&host=&mode=auto&x_padding_bytes=200-2000#t';
    const parsed = parseVlessLink(link);
    const xhttp = (parsed!.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    expect(xhttp.xPaddingBytes).toBe('200-2000');
  });

  it('extra takes precedence — camelCase wins over snake_case alias', () => {
    const link = 'vless://u@h:1?type=xhttp&security=none&path=%2F&host=&mode=auto'
      + '&xPaddingBytes=900-9000&x_padding_bytes=100-1000#t';
    const parsed = parseVlessLink(link);
    const xhttp = (parsed!.streamSettings as Record<string, unknown>).xhttpSettings as Record<string, unknown>;
    expect(xhttp.xPaddingBytes).toBe('900-9000');
  });

  it('ignores malformed extra JSON without breaking the rest of the link', () => {
    const link = 'vless://u@h:1?type=xhttp&security=none&path=%2F&host=&mode=auto'
      + '&extra=not-json&fp=chrome#t';
    const parsed = parseVlessLink(link);
    expect(parsed).not.toBeNull();
    const stream = parsed!.streamSettings as Record<string, unknown>;
    expect((stream.xhttpSettings as Record<string, unknown>).mode).toBe('auto');
  });
});

describe('parseWireguardLink', () => {
  it('parses a wireguard:// link with percent-encoded secret and publickey', () => {
    const link = 'wireguard://IKeuy2+BNspvMffiC47z16seLIGxGtbDIYiZcbh9C1U%3D@localhost:22824'
      + '?publickey=3CnNsCy74TOlupjaii%2BRFp%2FgDMk5vvUuFD0SNZ%2FGl2s%3D'
      + '&address=10.0.0.2%2F32&mtu=1420#-1';
    const out = parseWireguardLink(link);
    expect(out?.protocol).toBe('wireguard');
    expect(out?.tag).toBe('-1');
    const settings = out?.settings as {
      secretKey: string; address: string[]; mtu: number;
      peers: Array<{ publicKey: string; endpoint: string; allowedIPs: string[] }>;
    };
    expect(settings.secretKey).toBe('IKeuy2+BNspvMffiC47z16seLIGxGtbDIYiZcbh9C1U=');
    expect(settings.address).toEqual(['10.0.0.2/32']);
    expect(settings.mtu).toBe(1420);
    expect(settings.peers[0].publicKey).toBe('3CnNsCy74TOlupjaii+RFp/gDMk5vvUuFD0SNZ/Gl2s=');
    expect(settings.peers[0].endpoint).toBe('localhost:22824');
    expect(settings.peers[0].allowedIPs).toEqual(['0.0.0.0/0', '::/0']);
  });

  it('parses reserved, presharedkey and keepalive aliases', () => {
    const link = 'wireguard://privkey@1.2.3.4:51820'
      + '?publickey=peerpub&address=10.0.0.2/32,fd00::2/128'
      + '&reserved=1,2,3&presharedkey=psk-secret&persistentkeepalive=25'
      + '&allowedips=0.0.0.0/0#wg-peer';
    const out = parseWireguardLink(link);
    const settings = out?.settings as {
      reserved: number[];
      peers: Array<{ preSharedKey: string; keepAlive: number; allowedIPs: string[] }>;
      address: string[];
    };
    expect(settings.address).toEqual(['10.0.0.2/32', 'fd00::2/128']);
    expect(settings.reserved).toEqual([1, 2, 3]);
    expect(settings.peers[0].preSharedKey).toBe('psk-secret');
    expect(settings.peers[0].keepAlive).toBe(25);
    expect(settings.peers[0].allowedIPs).toEqual(['0.0.0.0/0']);
  });

  it('returns null for non-wireguard links', () => {
    expect(parseWireguardLink('vless://x@y:1')).toBeNull();
  });
});

describe('parseOutboundLink dispatcher', () => {
  it('dispatches vmess via base64 JSON', () => {
    const json = { v: '2', ps: 'x', add: '1.1.1.1', port: 443, id: '11111111-2222-4333-8444-555555555555', net: 'tcp', tls: 'none' };
    const link = `vmess://${Base64.encode(JSON.stringify(json))}`;
    expect(parseOutboundLink(link)?.protocol).toBe('vmess');
  });

  it('dispatches vless via URL', () => {
    expect(parseOutboundLink('vless://uuid@host:443?type=tcp&security=none')?.protocol).toBe('vless');
  });

  it('dispatches wireguard via URL', () => {
    expect(parseOutboundLink('wireguard://pk@host:22824?publickey=pub&address=10.0.0.2/32')?.protocol).toBe('wireguard');
  });

  it('returns null for an unknown scheme', () => {
    expect(parseOutboundLink('socks5://user:pass@host:1080')).toBeNull();
  });

  it('returns null for empty input', () => {
    expect(parseOutboundLink('')).toBeNull();
    expect(parseOutboundLink('   ')).toBeNull();
  });
});
