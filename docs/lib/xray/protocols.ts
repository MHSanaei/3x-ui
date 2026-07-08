// Pure decision logic for the protocol wizard. No React/DOM.

export type UseCase = 'censorship' | 'general' | 'speed';
export type CensorshipLevel = 'high' | 'medium' | 'low';
export type ClientSupport = 'modern' | 'broad';

export interface WizardAnswers {
  useCase: UseCase;
  censorship: CensorshipLevel;
  clientSupport: ClientSupport;
}

export interface Recommendation {
  protocol: string;
  transport: string;
  security: string;
  rationale: string;
  links: { title: string; href: string }[];
}

const REALITY_LINK = { title: 'REALITY setup', href: '/docs/config/reality' };
const TRANSPORTS_LINK = { title: 'Transports', href: '/docs/config/transports' };
const INBOUNDS_LINK = { title: 'Inbounds', href: '/docs/config/inbounds' };

export function recommend(a: WizardAnswers): Recommendation {
  const heavyCensorship = a.useCase === 'censorship' || a.censorship === 'high';

  if (heavyCensorship) {
    if (a.clientSupport === 'modern') {
      return {
        protocol: 'VLESS',
        transport: 'TCP',
        security: 'REALITY + XTLS-Vision',
        rationale:
          'REALITY disguises traffic as a real TLS site without a certificate, and XTLS-Vision keeps it fast. The best stealth option for heavy censorship — needs a modern client.',
        links: [REALITY_LINK, INBOUNDS_LINK],
      };
    }
    return {
      protocol: 'VMess',
      transport: 'WebSocket',
      security: 'TLS',
      rationale:
        'WebSocket + TLS works through CDNs and is supported by almost every client, making it a resilient fallback when broad client support matters more than peak stealth.',
      links: [TRANSPORTS_LINK, INBOUNDS_LINK],
    };
  }

  if (a.useCase === 'speed') {
    if (a.clientSupport === 'modern') {
      return {
        protocol: 'VLESS',
        transport: 'TCP',
        security: 'REALITY + XTLS-Vision',
        rationale:
          'XTLS-Vision over raw TCP has the lowest overhead, so it is the fastest option for modern clients.',
        links: [REALITY_LINK, TRANSPORTS_LINK],
      };
    }
    return {
      protocol: 'Trojan',
      transport: 'TCP',
      security: 'TLS',
      rationale: 'Trojan over TCP + TLS is simple and fast, and is widely supported by clients.',
      links: [INBOUNDS_LINK, TRANSPORTS_LINK],
    };
  }

  // general use
  if (a.clientSupport === 'modern') {
    return {
      protocol: 'VLESS',
      transport: 'WebSocket',
      security: 'TLS',
      rationale:
        'VLESS + WebSocket + TLS is a flexible, CDN-friendly default for everyday use with modern clients.',
      links: [TRANSPORTS_LINK, INBOUNDS_LINK],
    };
  }
  return {
    protocol: 'VMess',
    transport: 'WebSocket',
    security: 'TLS',
    rationale: 'VMess + WebSocket + TLS is the most broadly compatible everyday setup.',
    links: [TRANSPORTS_LINK, INBOUNDS_LINK],
  };
}
