export interface RealityTarget {
  target: string;
  sni: string;
}

export const REALITY_TARGETS: readonly RealityTarget[] = [
  { target: 'www.amazon.com:443', sni: 'www.amazon.com' },
  { target: 'aws.amazon.com:443', sni: 'aws.amazon.com' },
  { target: 'www.oracle.com:443', sni: 'www.oracle.com' },
  { target: 'www.nvidia.com:443', sni: 'www.nvidia.com' },
  { target: 'www.amd.com:443', sni: 'www.amd.com' },
  { target: 'www.intel.com:443', sni: 'www.intel.com' },
  { target: 'www.sony.com:443', sni: 'www.sony.com' },
];

export function getRandomRealityTarget(): RealityTarget {
  const randomIndex = Math.floor(Math.random() * REALITY_TARGETS.length);
  const selected = REALITY_TARGETS[randomIndex];
  return {
    target: selected.target,
    sni: selected.sni,
  };
}
