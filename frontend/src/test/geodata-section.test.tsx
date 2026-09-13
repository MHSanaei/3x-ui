import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';

import GeodataSection from '@/pages/index/GeodataSection';
import { HttpUtil, Msg } from '@/utils';

const STANDARD_SOURCES = [
  {
    url: 'https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat',
    file: 'geoip.dat',
  },
  {
    url: 'https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat',
    file: 'geosite.dat',
  },
  {
    url: 'https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat',
    file: 'geoip_IR.dat',
  },
  {
    url: 'https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat',
    file: 'geosite_IR.dat',
  },
  {
    url: 'https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat',
    file: 'geoip_RU.dat',
  },
  {
    url: 'https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat',
    file: 'geosite_RU.dat',
  },
];

afterEach(() => {
  vi.restoreAllMocks();
});

describe('GeodataSection', () => {
  it('fills the six standard sources supplied by the panel', async () => {
    vi.spyOn(HttpUtil, 'post').mockResolvedValue(
      new Msg(
        true,
        '',
        JSON.stringify({
          xraySetting: { outbounds: [] },
          geodataSources: STANDARD_SOURCES,
        }),
      ),
    );
    const user = userEvent.setup();

    render(<GeodataSection active onBusy={vi.fn()} onClose={vi.fn()} />);

    const button = (await screen.findByRole('button', {
      name: 'Use standard sources',
    })) as HTMLButtonElement;
    await waitFor(() => expect(button.disabled).toBe(false));
    await user.click(button);

    await waitFor(() => {
      for (const source of STANDARD_SOURCES) {
        expect(screen.getByDisplayValue(source.url)).toBeTruthy();
        expect(screen.getByDisplayValue(source.file)).toBeTruthy();
      }
    });
  });
});
