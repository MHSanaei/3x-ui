import happ from './app-icons/happ.svg';
import incy from './app-icons/incy.webp';
import shadowrocket from './app-icons/shadowrocket.webp';
import singBox from './app-icons/sing-box.svg';
import streisand from './app-icons/streisand.webp';
import v2box from './app-icons/v2box.webp';
import v2rayng from './app-icons/v2rayng.svg';
import v2raytun from './app-icons/v2raytun.svg';

// Tinted entries are Arcticons line art drawn in the theme colour; the rest are full-colour app icons.
export const APP_ICONS: Record<string, { src: string; tinted: boolean }> = {
  V2Box: { src: v2box, tinted: false },
  V2RayNG: { src: v2rayng, tinted: true },
  'Sing-box': { src: singBox, tinted: true },
  V2RayTun: { src: v2raytun, tinted: true },
  Happ: { src: happ, tinted: true },
  Incy: { src: incy, tinted: false },
  Shadowrocket: { src: shadowrocket, tinted: false },
  Streisand: { src: streisand, tinted: false },
};
