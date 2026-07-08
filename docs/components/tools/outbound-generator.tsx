'use client';

import { useState } from 'react';
import {
  buildOutboundJson,
  type OutboundInput,
  type OutboundKind,
  type Network,
  type Security,
  type ProxyServerInput,
  type StreamInput,
  type WireguardInput,
} from '@/lib/xray/outbounds';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField } from './shared/fields';
import { OutputBlock } from './shared/output-block';

const KINDS: readonly OutboundKind[] = [
  'freedom',
  'blackhole',
  'vless',
  'vmess',
  'trojan',
  'shadowsocks',
  'socks',
  'http',
  'wireguard',
  'warp',
];
const NETWORKS: readonly Network[] = ['tcp', 'kcp', 'ws', 'grpc', 'httpupgrade', 'xhttp'];
const SECURITIES: readonly Security[] = ['none', 'tls', 'reality'];
const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random'];
const DOMAIN_STRATEGIES = ['AsIs', 'UseIP', 'UseIPv4', 'UseIPv6', 'ForceIP'];

const PROXY_KINDS = new Set<OutboundKind>([
  'vless',
  'vmess',
  'trojan',
  'shadowsocks',
  'socks',
  'http',
]);
const STREAM_KINDS = new Set<OutboundKind>(['vless', 'vmess', 'trojan', 'shadowsocks']);
const WG_KINDS = new Set<OutboundKind>(['wireguard', 'warp']);

export function OutboundGenerator() {
  const [kind, setKind] = useState<OutboundKind>('vless');
  const [tag, setTag] = useState('proxy');

  // proxy server
  const [address, setAddress] = useState('example.com');
  const [port, setPort] = useState('443');
  const [id, setId] = useState('');
  const [password, setPassword] = useState('');
  const [method, setMethod] = useState('2022-blake3-aes-128-gcm');
  const [flow, setFlow] = useState('');
  const [username, setUsername] = useState('');

  // stream
  const [network, setNetwork] = useState<Network>('tcp');
  const [security, setSecurity] = useState<Security>('reality');
  const [host, setHost] = useState('');
  const [path, setPath] = useState('/');
  const [serviceName, setServiceName] = useState('');
  const [sni, setSni] = useState('www.microsoft.com');
  const [fingerprint, setFingerprint] = useState('chrome');
  const [publicKey, setPublicKey] = useState('');
  const [shortId, setShortId] = useState('');

  // freedom
  const [domainStrategy, setDomainStrategy] = useState('AsIs');

  // wireguard
  const [wgSecretKey, setWgSecretKey] = useState('');
  const [wgAddress, setWgAddress] = useState('172.16.0.2/32');
  const [wgPublicKey, setWgPublicKey] = useState('');
  const [wgEndpoint, setWgEndpoint] = useState('');

  const isProxy = PROXY_KINDS.has(kind);
  const hasStream = STREAM_KINDS.has(kind);
  const isWg = WG_KINDS.has(kind);
  const hasPath = network === 'ws' || network === 'httpupgrade' || network === 'xhttp';

  const server: ProxyServerInput = {
    address,
    port: Number(port),
    id,
    password,
    method,
    flow,
    username,
  };

  const stream: StreamInput = {
    network,
    security,
    host,
    path,
    serviceName,
    sni,
    fingerprint,
    publicKey,
    shortId,
  };

  const wireguard: WireguardInput = {
    secretKey: wgSecretKey,
    address: wgAddress
      .split(',')
      .map((a) => a.trim())
      .filter(Boolean),
    publicKey: wgPublicKey,
    endpoint: wgEndpoint,
  };

  const input: OutboundInput = {
    kind,
    tag,
    server: isProxy ? server : undefined,
    wireguard: isWg ? wireguard : undefined,
    stream: hasStream ? stream : undefined,
    domainStrategy: kind === 'freedom' ? domainStrategy : undefined,
  };

  function reset() {
    setKind('vless');
    setTag('proxy');
    setAddress('example.com');
    setPort('443');
    setId('');
    setPassword('');
    setMethod('2022-blake3-aes-128-gcm');
    setFlow('');
    setUsername('');
    setNetwork('tcp');
    setSecurity('reality');
    setHost('');
    setPath('/');
    setServiceName('');
    setSni('www.microsoft.com');
    setFingerprint('chrome');
    setPublicKey('');
    setShortId('');
    setDomainStrategy('AsIs');
    setWgSecretKey('');
    setWgAddress('172.16.0.2/32');
    setWgPublicKey('');
    setWgEndpoint('');
  }

  return (
    <ToolFrame
      title="Outbound config generator"
      description="Build an Xray outbound object — freedom, blackhole, a proxy protocol, WireGuard, or WARP — to paste into your Xray configuration."
      onReset={reset}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <SelectField
          label="Kind"
          value={kind}
          onChange={(v) => setKind(v as OutboundKind)}
          options={KINDS}
        />
        <TextField label="Tag" value={kind === 'warp' ? 'warp' : tag} onChange={setTag} />

        {kind === 'freedom' ? (
          <SelectField
            label="Domain strategy"
            value={domainStrategy}
            onChange={setDomainStrategy}
            options={DOMAIN_STRATEGIES}
          />
        ) : null}

        {isProxy ? (
          <>
            <TextField label="Address" value={address} onChange={setAddress} />
            <TextField label="Port" value={port} onChange={setPort} inputMode="numeric" />
            {(kind === 'vless' || kind === 'vmess') && (
              <TextField label="UUID (id)" value={id} onChange={setId} />
            )}
            {kind === 'vless' && (
              <TextField
                label="Flow"
                value={flow}
                onChange={setFlow}
                placeholder="xtls-rprx-vision (optional)"
              />
            )}
            {(kind === 'trojan' || kind === 'shadowsocks') && (
              <TextField label="Password" value={password} onChange={setPassword} />
            )}
            {kind === 'shadowsocks' && (
              <TextField label="Method (cipher)" value={method} onChange={setMethod} />
            )}
            {(kind === 'socks' || kind === 'http') && (
              <>
                <TextField
                  label="Username"
                  value={username}
                  onChange={setUsername}
                  placeholder="optional"
                />
                <TextField label="Password" value={password} onChange={setPassword} />
              </>
            )}
          </>
        ) : null}

        {isWg ? (
          <>
            <TextField
              label="Private key (secretKey)"
              value={wgSecretKey}
              onChange={setWgSecretKey}
            />
            <TextField label="Local address" value={wgAddress} onChange={setWgAddress} />
            <TextField label="Peer public key" value={wgPublicKey} onChange={setWgPublicKey} />
            <TextField
              label="Peer endpoint"
              value={wgEndpoint}
              onChange={setWgEndpoint}
              placeholder={kind === 'warp' ? 'engage.cloudflareclient.com:2408' : 'host:51820'}
            />
          </>
        ) : null}
      </div>

      {hasStream ? (
        <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
          <SelectField
            label="Transport"
            value={network}
            onChange={(v) => setNetwork(v as Network)}
            options={NETWORKS}
          />
          <SelectField
            label="Security"
            value={security}
            onChange={(v) => setSecurity(v as Security)}
            options={SECURITIES}
          />
          {hasPath ? (
            <>
              <TextField label="Path" value={path} onChange={setPath} />
              <TextField label="Host" value={host} onChange={setHost} placeholder="optional" />
            </>
          ) : null}
          {network === 'grpc' ? (
            <TextField label="serviceName" value={serviceName} onChange={setServiceName} />
          ) : null}
          {security !== 'none' ? (
            <>
              <TextField label="SNI (serverName)" value={sni} onChange={setSni} />
              <SelectField
                label="Fingerprint"
                value={fingerprint}
                onChange={setFingerprint}
                options={FINGERPRINTS}
              />
            </>
          ) : null}
          {security === 'reality' ? (
            <>
              <TextField label="Public key (pbk)" value={publicKey} onChange={setPublicKey} />
              <TextField label="Short ID (sid)" value={shortId} onChange={setShortId} />
            </>
          ) : null}
        </div>
      ) : null}

      <div className="mt-4">
        <OutputBlock label="Outbound (Xray JSON)" value={buildOutboundJson(input)} />
      </div>
    </ToolFrame>
  );
}
