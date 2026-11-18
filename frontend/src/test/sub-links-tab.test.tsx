import { fireEvent, render, screen, within } from '@testing-library/react';
import { QRCode } from 'antd';
import { afterEach, describe, expect, it, vi } from 'vitest';

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

afterEach(() => {
  window.history.replaceState({}, '', '/');
});

describe('SubLinksTab inline QR card', () => {
  it('switches the inline QR code between enabled subscription formats', () => {
    const view = renderWithProviders(
      <SubLinksTab
        subUrl={SUB_URL}
        subJsonUrl={JSON_URL}
        subClashUrl={CLASH_URL}
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

  it('defaults the inline QR code to CLASH when opened via the Clash URL even alongside a malformed URI', () => {
    window.history.replaceState({}, '', '/clash/abc?html=1');

    const view = renderWithProviders(
      <SubLinksTab
        subUrl={SUB_URL}
        subJsonUrl="http://[::1"
        subClashUrl={CLASH_URL}
        onCopy={vi.fn()}
      />,
    );

    const card = view.container.querySelector('.sub-qr-card') as HTMLElement;
    expect(card.querySelector('.sub-qr-code svg')?.innerHTML).toBe(expectedQrMarkup(CLASH_URL));
  });

  it('renders the inline QR card without a switcher when only Clash subscription is present', () => {
    const view = renderWithProviders(
      <SubLinksTab subUrl="" subJsonUrl="" subClashUrl={CLASH_URL} onCopy={vi.fn()} />,
    );

    const card = view.container.querySelector('.sub-qr-card') as HTMLElement;
    expect(card).not.toBeNull();
    expect(card.querySelector('.sub-qr-code svg')?.innerHTML).toBe(expectedQrMarkup(CLASH_URL));
    expect(card.querySelector('.ant-segmented')).toBeNull();
    expect(screen.getByText('Clash / Mihomo')).not.toBeNull();
  });
});
