import { Base64 } from '@/utils';
import type { VkTurnSettings } from '@/schemas/protocols/inbound/wireguard';

interface VkTurnConnectionInput {
  vkTurn: VkTurnSettings | undefined;
  privateKey: string;
  peerPublicKey: string;
  presharedKey?: string;
  tunnelAddress: string;
  address: string;
  dnsServers?: string;
}

export function buildVkTurnConnectionLink(input: VkTurnConnectionInput): string {
  const {
    vkTurn,
    privateKey,
    peerPublicKey,
    presharedKey = '',
    tunnelAddress,
    address,
    dnsServers,
  } = input;
  const ipv4Address = tunnelAddress
    .split(/[,\s]+/)
    .find((value) => /^\d+\.\d+\.\d+\.\d+\/\d+$/.test(value));
  if (!vkTurn?.enabled || !privateKey || !peerPublicKey || !ipv4Address) return '';
  const host = address.replace(/^\[|\]$/g, '');
  const endpointHost = host.includes(':') ? `[${host}]` : host;
  const settings = {
    privateKey,
    peerPublicKey,
    presharedKey,
    tunnelAddress: ipv4Address,
    peerAddress: `${endpointHost}:${vkTurn.port}`,
    vkLink: vkTurn.callLink,
    dnsServers: dnsServers || '1.1.1.1, 1.0.0.1',
    numConnections: 30,
    useDTLS: true,
    useSrtp: true,
    useUDP: false,
    useWrap: false,
    useWrapA: false,
    useWrapS: false,
    useCsqtt: false,
    wrapKeyHex: '',
  };
  return `vkturnproxy://import?data=${encodeURIComponent(Base64.encode(JSON.stringify({ version: 1, type: 'connection', settings })))}`;
}
