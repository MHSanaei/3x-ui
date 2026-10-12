type Raw = Record<string, unknown>;

function isEmptyHopValue(value: unknown): boolean {
  return value == null || value === 0 || (typeof value === 'string' && value.trim() === '');
}

// xray-core 26.9.9 hops a client only through the "udphop" UDP mask. Mirrors
// internal/util/maskcompat: quicParams.udpHop becomes an intervalRemote mask.
export function upgradeLegacyUdpHop(finalmask: Raw): { next: Raw; changed: boolean } {
  const quicParams = finalmask.quicParams;
  if (!quicParams || typeof quicParams !== 'object' || !('udpHop' in quicParams)) {
    return { next: finalmask, changed: false };
  }
  const { udpHop, ...restQuic } = quicParams as Raw;
  const next: Raw = { ...finalmask };
  if (Object.keys(restQuic).length > 0) next.quicParams = restQuic;
  else delete next.quicParams;

  const hop = (udpHop && typeof udpHop === 'object' ? udpHop : {}) as Raw;
  const udp = Array.isArray(finalmask.udp) ? [...(finalmask.udp as unknown[])] : [];
  const hasMask = udp.some(isUdpHopMask);
  if (!isEmptyHopValue(hop.ports) && !hasMask) {
    const settings: Raw = { mode: 'intervalRemote', remotePorts: hop.ports };
    if (!isEmptyHopValue(hop.interval)) settings.interval = hop.interval;
    udp.push({ type: 'udphop', settings });
    next.udp = udp;
  }
  return { next, changed: true };
}

function isUdpHopMask(mask: unknown): boolean {
  return String((mask as Raw | null)?.type ?? '').toLowerCase() === 'udphop';
}

// Mirrors maskcompat.NormalizeUDPHop: the core reverses the UDP masks and refuses ("incorrect
// index") a udphop anywhere but the socket end, so the last one listed is kept alone, last.
export function normalizeUdpHop(finalmask: Raw): { next: Raw; changed: boolean } {
  const udp = Array.isArray(finalmask.udp) ? (finalmask.udp as unknown[]) : [];
  const hops = udp.filter(isUdpHopMask);
  if (hops.length === 0 || (hops.length === 1 && isUdpHopMask(udp[udp.length - 1]))) {
    return { next: finalmask, changed: false };
  }
  const next = {
    ...finalmask,
    udp: [...udp.filter((m) => !isUdpHopMask(m)), hops[hops.length - 1]],
  };
  return { next, changed: true };
}
