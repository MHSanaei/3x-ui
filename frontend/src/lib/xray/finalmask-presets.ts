import type { TcpMask } from '@/schemas/protocols/stream/finalmask';

// PattN's client presets: one fragment splits the TLS ClientHello into three segments
// (the first may be empty), a second splits the next packet with a 1 ms delay.
export const TCP_MASK_PRESETS: Readonly<Record<string, TcpMask[]>> = Object.freeze({
  'tlshello-0-len': [
    {
      type: 'fragment',
      settings: { packets: 'tlshello', lengths: ['0', '104', '1'], delays: ['0'], maxSplit: '0' },
    },
    {
      type: 'fragment',
      settings: { packets: '1-1', lengths: ['114', '1'], delays: ['1'], maxSplit: '11' },
    },
  ],
  'tlshello-6-98': [
    {
      type: 'fragment',
      settings: { packets: 'tlshello', lengths: ['6', '98', '1'], delays: ['0'], maxSplit: '0' },
    },
    {
      type: 'fragment',
      settings: { packets: '1-1', lengths: ['114', '1'], delays: ['1'], maxSplit: '11' },
    },
  ],
});
