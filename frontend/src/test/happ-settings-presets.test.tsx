import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

import { AllSetting } from '@/models/setting';
import HappSettingsContent from '@/pages/settings/HappSettingsContent';

import { renderWithProviders } from './test-utils';

const chinaProfile = {
  Name: 'Bypass-CN',
  GlobalProxy: 'true',
  RouteOrder: 'block-proxy-direct',
  RemoteDNSType: 'DoH',
  RemoteDNSDomain: 'https://cloudflare-dns.com/dns-query',
  RemoteDNSIP: '1.1.1.1',
  DomesticDNSType: 'DoH',
  DomesticDNSDomain: 'https://dns.alidns.com/dns-query',
  DomesticDNSIP: '223.5.5.5',
  Geoipurl: '',
  Geositeurl: '',
  LastUpdated: '1787658176',
  DnsHosts: {
    'cloudflare-dns.com': '1.1.1.1',
    'dns.alidns.com': '223.5.5.5',
  },
  DirectSites: ['geosite:private', 'geosite:cn', 'geosite:geolocation-cn'],
  DirectIp: [
    'geoip:cn',
    '127.0.0.0/8',
    '10.0.0.0/8',
    '172.16.0.0/12',
    '192.168.0.0/16',
    '169.254.0.0/16',
    '224.0.0.0/4',
    '255.255.255.255',
  ],
  ProxySites: [],
  ProxyIp: [],
  BlockSites: [],
  BlockIp: [],
  DomainStrategy: 'IPIfNonMatch',
  FakeDNS: 'false',
  UseChunkFiles: 'true',
};

function renderSettings(isMobile = false) {
  const updateSetting = vi.fn<(patch: Partial<AllSetting>) => void>();

  function SettingsHarness() {
    const [allSetting, setAllSetting] = useState(() => {
      const settings = new AllSetting();
      settings.subRoutingRules = 'https://example.com/existing-routing';
      return settings;
    });

    return (
      <HappSettingsContent
        allSetting={allSetting}
        updateSetting={(patch) => {
          updateSetting(patch);
          setAllSetting((previous) => Object.assign(new AllSetting(), previous, patch));
        }}
        isMobile={isMobile}
        remoteSourceBadge={() => null}
      />
    );
  }

  renderWithProviders(<SettingsHarness />);
  return updateSetting;
}

function openPresets() {
  const select = screen.getByRole('combobox').closest('.ant-select');
  if (!select) throw new Error('Routing preset select not found');
  fireEvent.mouseDown(select.querySelector('.ant-select-selector') ?? select);
  return Array.from(
    document.querySelectorAll(
      '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
    ),
  );
}

function choosePreset(label: string) {
  const option = openPresets().find((item) => item.textContent === label);
  if (!option) throw new Error(`Routing preset '${label}' not found`);
  fireEvent.click(option);
}

function routingRules() {
  return (screen.getByRole('textbox', { name: 'Routing rules' }) as HTMLTextAreaElement).value;
}

function routingProfile(): Record<string, unknown> {
  const prefix = 'happ://routing/onadd/';
  const link = routingRules();
  expect(link.startsWith(prefix)).toBe(true);
  return JSON.parse(atob(link.slice(prefix.length))) as Record<string, unknown>;
}

describe('Happ routing preset controls', () => {
  it('keeps selecting and toggling local until Apply updates the routing rules', () => {
    const updateSetting = renderSettings();

    choosePreset('China Direct (Bypass-CN)');
    const includeAdblock = screen.getByRole('switch', { name: 'Include AdBlock' });
    expect(includeAdblock.getAttribute('aria-checked')).toBe('false');
    fireEvent.click(includeAdblock);

    expect(includeAdblock.getAttribute('aria-checked')).toBe('true');
    expect(updateSetting).not.toHaveBeenCalled();
    expect(routingRules()).toBe('https://example.com/existing-routing');

    fireEvent.click(screen.getByRole('button', { name: 'Apply preset' }));

    expect(updateSetting).toHaveBeenCalledExactlyOnceWith({ subRoutingRules: routingRules() });
    expect(routingProfile().BlockSites).toEqual(['geosite:category-ads-all']);
  });

  it.each([
    ['Iran Bypass', 'Iran Bypass'],
    ['China Direct (Bypass-CN)', 'Bypass-CN'],
    ['Full Proxy', 'Global Proxy'],
  ])('applies %s without AdBlock by default', (label, profileName) => {
    renderSettings();
    choosePreset(label);

    expect(
      screen.getByRole('switch', { name: 'Include AdBlock' }).getAttribute('aria-checked'),
    ).toBe('false');
    fireEvent.click(screen.getByRole('button', { name: 'Apply preset' }));

    expect(routingProfile()).toMatchObject({ Name: profileName, BlockSites: [] });
  });

  it('preserves China routing when opting in and clears AdBlock when reapplied after opting out', () => {
    const updateSetting = renderSettings(true);
    choosePreset('China Direct (Bypass-CN)');
    const includeAdblock = screen.getByRole('switch', { name: 'Include AdBlock' });
    fireEvent.click(includeAdblock);
    fireEvent.click(screen.getByRole('button', { name: 'Apply preset' }));

    expect(routingProfile()).toEqual({
      ...chinaProfile,
      BlockSites: ['geosite:category-ads-all'],
    });
    const withAdblock = routingRules();

    fireEvent.click(includeAdblock);
    expect(routingRules()).toBe(withAdblock);
    expect(updateSetting).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole('button', { name: 'Apply preset' }));

    expect(updateSetting).toHaveBeenCalledTimes(2);
    expect(routingProfile()).toEqual(chinaProfile);
  });

  it('offers only base presets so AdBlock cannot replace a country preset', () => {
    renderSettings();

    expect(openPresets().map((option) => option.textContent)).toEqual([
      'Iran Bypass',
      'China Direct (Bypass-CN)',
      'Full Proxy',
      'Disable Routing (happ://routing/off)',
    ]);
  });

  it('disables AdBlock for Off and applies the off link even after opting in', () => {
    const updateSetting = renderSettings();
    const includeAdblock = screen.getByRole('switch', { name: 'Include AdBlock' });
    fireEvent.click(includeAdblock);
    choosePreset('Disable Routing (happ://routing/off)');

    expect((includeAdblock as HTMLButtonElement).disabled).toBe(true);
    expect(updateSetting).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Apply preset' }));

    expect(updateSetting).toHaveBeenCalledExactlyOnceWith({
      subRoutingRules: 'happ://routing/off',
    });
    expect(routingRules()).toBe('happ://routing/off');
  });
});
