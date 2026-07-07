'use client';

import { useCallback, useEffect, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import {
  generateX25519KeyPair,
  isX25519Available,
  randomShortId,
  randomUuid,
  realityClientLink,
  realityServerInbound,
  type RealityConfig,
  type X25519KeyPair,
} from '@/lib/xray/reality';
import { ToolFrame } from './tool-frame';
import { TextField, SelectField } from './shared/fields';
import { OutputBlock } from './shared/output-block';
import { CopyButton } from './shared/copy-button';

const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'ios', 'android', 'edge', 'random'] as const;

export function RealityConfigGenerator() {
  const [address, setAddress] = useState('your-server.com');
  const [port, setPort] = useState('443');
  const [dest, setDest] = useState('www.microsoft.com:443');
  const [sni, setSni] = useState('www.microsoft.com');
  const [fingerprint, setFingerprint] = useState<string>('chrome');
  const [uuid, setUuid] = useState('');
  const [shortId, setShortId] = useState('');
  const [keys, setKeys] = useState<X25519KeyPair | null>(null);
  const [unavailable, setUnavailable] = useState(false);

  const regenerate = useCallback(async () => {
    setUuid(randomUuid());
    setShortId(randomShortId(4));
    if (!isX25519Available()) {
      setUnavailable(true);
      return;
    }
    try {
      setKeys(await generateX25519KeyPair());
      setUnavailable(false);
    } catch {
      setUnavailable(true);
    }
  }, []);

  // Generate keys/identifiers on the client after hydration. This is a genuine
  // client-only side effect (WebCrypto + randomness), not derived render state.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void regenerate();
  }, [regenerate]);

  const config: RealityConfig | null =
    keys && uuid
      ? {
          address,
          port: Number(port) || 443,
          uuid,
          dest,
          serverNames: [sni],
          shortIds: [shortId],
          privateKey: keys.privateKey,
          publicKey: keys.publicKey,
          fingerprint,
          spiderX: '/',
          flow: 'xtls-rprx-vision',
        }
      : null;

  const serverJson = config ? JSON.stringify(realityServerInbound(config), null, 2) : '';
  const clientLink = config ? realityClientLink(config) : '';

  return (
    <ToolFrame
      title="REALITY config generator"
      description="Generate a VLESS + REALITY inbound and client link. Keys are created in your browser — nothing is sent anywhere."
      onReset={() => void regenerate()}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <TextField
          label="Server address"
          value={address}
          onChange={setAddress}
          hint="Your domain or IP"
        />
        <TextField label="Port" value={port} onChange={setPort} inputMode="numeric" />
        <TextField
          label="Dest (camouflage target)"
          value={dest}
          onChange={setDest}
          hint="A real TLS 1.3 site, e.g. www.microsoft.com:443"
        />
        <TextField label="SNI / Server name" value={sni} onChange={setSni} />
        <SelectField
          label="Fingerprint"
          value={fingerprint}
          onChange={setFingerprint}
          options={FINGERPRINTS}
        />
      </div>

      {unavailable ? (
        <div className="mt-4 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
          Your browser can&apos;t generate X25519 keys here. Generate them on the server instead:
          <div className="mt-2">
            <OutputBlock label="run on the server" value="xray x25519" />
          </div>
        </div>
      ) : (
        <>
          <div className="mt-4 flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">Generated keys &amp; identifiers</span>
            <button
              type="button"
              onClick={() => void regenerate()}
              className="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
            >
              <RefreshCw className="size-3.5" aria-hidden />
              Regenerate
            </button>
          </div>

          <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-2">
            <KeyRow label="Public key" value={keys?.publicKey ?? ''} />
            <KeyRow label="Private key" value={keys?.privateKey ?? ''} />
            <KeyRow label="UUID" value={uuid} />
            <KeyRow label="Short ID" value={shortId} />
          </div>

          <div className="mt-4 grid grid-cols-1 gap-4">
            <OutputBlock label="Server inbound (Xray JSON)" value={serverJson} />
            <OutputBlock label="Client share link" value={clientLink} qr />
          </div>
        </>
      )}
    </ToolFrame>
  );
}

function KeyRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-2 rounded-lg border bg-fd-background px-3 py-2">
      <span className="shrink-0 text-xs font-medium text-fd-muted-foreground">{label}</span>
      <code dir="ltr" className="flex-1 truncate text-start text-xs">
        {value}
      </code>
      <CopyButton value={value} label="" className="px-1.5" />
    </div>
  );
}
