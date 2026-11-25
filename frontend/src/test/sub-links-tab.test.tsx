import { fireEvent, render, within } from '@testing-library/react';
import { QRCode } from 'antd';
import { describe, expect, it, vi } from 'vitest';

import SubConfigsTab from '@/pages/sub/SubConfigsTab';
import SubLinksTab from '@/pages/sub/SubLinksTab';
import { renderWithProviders } from './test-utils';

const SUB_URL = 'https://vpn.example.com/sub/abc';
const JSON_URL = 'https://vpn.example.com/json/abc';
const CLASH_URL = 'https://vpn.example.com/clash/abc';

function expectedQrMarkup(value: string): string {
  const { container, unmount } = render(
    <QRCode
      value={value}
      size={112}
      type="svg"
      bordered={false}
      color="#000000"
      bgColor="#ffffff"
    />,
  );
  const html = container.querySelector('svg')?.innerHTML ?? '';
  unmount();
  return html;
}

describe('SubLinksTab inline QR card', () => {
  it('switches the inline QR code between enabled subscription formats', () => {
    const view = renderWithProviders(
      <SubLinksTab
        subUrl={SUB_URL}
        subJsonUrl={JSON_URL}
        subClashUrl={CLASH_URL}
        format="raw"
        onCopy={vi.fn()}
      />,
    );

    const card = view.container.querySelector('.sub-qr-card') as HTMLElement;
    expect(card).not.toBeNull();
    const qrSvg = () => card.querySelector('.sub-qr-code svg')?.innerHTML;

    expect(qrSvg()).toBe(expectedQrMarkup(SUB_URL));
    expect(qrSvg()).not.toBe(expectedQrMarkup(CLASH_URL));

    fireEvent.click(within(card).getByText('CLASH'));
    expect(qrSvg()).toBe(expectedQrMarkup(CLASH_URL));

    fireEvent.click(within(card).getByText('JSON'));
    expect(qrSvg()).toBe(expectedQrMarkup(JSON_URL));
  });

  it.each([
    ['clash', CLASH_URL],
    ['json', JSON_URL],
  ] as const)('starts on the %s QR code when that route rendered the page', (format, url) => {
    const view = renderWithProviders(
      <SubLinksTab
        subUrl={SUB_URL}
        subJsonUrl={JSON_URL}
        subClashUrl={CLASH_URL}
        format={format}
        onCopy={vi.fn()}
      />,
    );

    const card = view.container.querySelector('.sub-qr-card') as HTMLElement;
    expect(card.querySelector('.sub-qr-code svg')?.innerHTML).toBe(expectedQrMarkup(url));
  });

  it('offers no switcher when only the raw subscription is enabled', () => {
    const view = renderWithProviders(
      <SubLinksTab subUrl={SUB_URL} subJsonUrl="" subClashUrl="" format="raw" onCopy={vi.fn()} />,
    );

    const card = view.container.querySelector('.sub-qr-card') as HTMLElement;
    expect(card.querySelector('.sub-qr-code svg')?.innerHTML).toBe(expectedQrMarkup(SUB_URL));
    expect(card.querySelector('.ant-segmented')).toBeNull();
  });
});

describe('SubConfigsTab row titles (#7198)', () => {
  it('names a remark-less tg://proxy row from linkRemarks at its own index', () => {
    const links = [
      'vless://11111111-1111-4111-8111-111111111111@h.example.com:443?type=tcp#DE-alice',
      'tg://proxy?server=203.0.113.7&port=8443&secret=ee00',
      'tg://proxy?server=203.0.113.8&port=8443&secret=ee11',
    ];
    const view = renderWithProviders(
      <SubConfigsTab links={links} linkRemarks={['', 'mt-sub-user', '']} onCopy={vi.fn()} />,
    );

    const titles = [...view.container.querySelectorAll('.sub-row-title')].map(
      (el) => el.textContent,
    );
    expect(titles).toEqual(['DE-alice', 'mt-sub-user', 'Link 3']);
  });
});
