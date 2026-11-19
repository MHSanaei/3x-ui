import { useEffect, useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';

import DnsTab from '@/pages/xray/dns/DnsTab';
import type { SetTemplate, XraySettingsValue } from '@/hooks/useXraySetting';
import { renderWithProviders } from './test-utils';
import { installPointerEvents, pointAt } from './pointer-events';

function withHosts(hosts: Record<string, string>): XraySettingsValue {
  return {
    dns: {
      hosts,
      servers: [],
    },
  } as unknown as XraySettingsValue;
}

describe('DnsTab', () => {
  it('keeps an empty row after adding a host', () => {
    function Harness() {
      const [templateSettings, setTemplateSettings] = useState<XraySettingsValue | null>(
        withHosts({ 'first.example': '1.1.1.1' }),
      );
      const updateTemplate: SetTemplate = (next) => {
        setTemplateSettings((current) => (typeof next === 'function' ? next(current) : next));
      };

      return <DnsTab templateSettings={templateSettings} setTemplateSettings={updateTemplate} />;
    }

    renderWithProviders(<Harness />);

    fireEvent.click(screen.getByRole('tab', { name: /Hosts$/ }));
    fireEvent.click(screen.getByRole('button', { name: /Add Host$/ }));

    expect(screen.getAllByLabelText('Domain (e.g. domain:example.com)')).toHaveLength(2);
  });

  it('keeps a row visible while its domain is incomplete', () => {
    function Harness() {
      const [templateSettings, setTemplateSettings] = useState<XraySettingsValue | null>(
        withHosts({ 'first.example': '1.1.1.1' }),
      );
      const updateTemplate: SetTemplate = (next) => {
        setTemplateSettings((current) => (typeof next === 'function' ? next(current) : next));
      };

      return <DnsTab templateSettings={templateSettings} setTemplateSettings={updateTemplate} />;
    }

    renderWithProviders(<Harness />);
    fireEvent.click(screen.getByRole('tab', { name: /Hosts$/ }));
    fireEvent.change(screen.getByLabelText('Domain (e.g. domain:example.com)'), {
      target: { value: '' },
    });

    expect(
      (screen.getByLabelText('Domain (e.g. domain:example.com)') as HTMLInputElement).value,
    ).toBe('');
  });

  it('shows hosts from an externally refreshed configuration', () => {
    function Harness() {
      const [templateSettings, setTemplateSettings] = useState<XraySettingsValue | null>(
        withHosts({ 'first.example': '1.1.1.1' }),
      );
      const updateTemplate: SetTemplate = (next) => {
        setTemplateSettings((current) => (typeof next === 'function' ? next(current) : next));
      };

      return (
        <>
          <button
            type="button"
            onClick={() => setTemplateSettings(withHosts({ 'second.example': '2.2.2.2' }))}
          >
            Refresh hosts
          </button>
          <DnsTab templateSettings={templateSettings} setTemplateSettings={updateTemplate} />
        </>
      );
    }

    renderWithProviders(<Harness />);

    fireEvent.click(screen.getByRole('tab', { name: /Hosts$/ }));
    expect(
      (screen.getByLabelText('Domain (e.g. domain:example.com)') as HTMLInputElement).value,
    ).toBe('first.example');

    fireEvent.click(screen.getByRole('button', { name: 'Refresh hosts' }));
    expect(
      (screen.getByLabelText('Domain (e.g. domain:example.com)') as HTMLInputElement).value,
    ).toBe('second.example');
  });

  it('clears an incomplete host draft when DNS is disabled', () => {
    function Harness() {
      const [templateSettings, setTemplateSettings] = useState<XraySettingsValue | null>(
        withHosts({ 'first.example': '1.1.1.1' }),
      );
      const updateTemplate: SetTemplate = (next) => {
        setTemplateSettings((current) => (typeof next === 'function' ? next(current) : next));
      };

      return (
        <>
          <button type="button" onClick={() => setTemplateSettings({})}>
            Disable DNS
          </button>
          <button type="button" onClick={() => setTemplateSettings(withHosts({}))}>
            Enable DNS
          </button>
          <DnsTab templateSettings={templateSettings} setTemplateSettings={updateTemplate} />
        </>
      );
    }

    renderWithProviders(<Harness />);
    fireEvent.click(screen.getByRole('tab', { name: /Hosts$/ }));
    fireEvent.change(screen.getByLabelText('Domain (e.g. domain:example.com)'), {
      target: { value: '' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Disable DNS' }));
    fireEvent.click(screen.getByRole('button', { name: 'Enable DNS' }));
    fireEvent.click(screen.getByRole('tab', { name: /Hosts$/ }));

    expect(screen.queryByLabelText('Domain (e.g. domain:example.com)')).toBeNull();
  });

  // xray-core 26.10.10 dns.script is a file path; an empty one fails the core build.
  it('writes the Lua script path and drops the key once cleared', () => {
    const seen = vi.fn();
    const latest = () => seen.mock.lastCall?.[0] as { dns: Record<string, unknown> };
    function Harness() {
      const [templateSettings, setTemplateSettings] = useState<XraySettingsValue | null>(
        withHosts({}),
      );
      useEffect(() => seen(templateSettings), [templateSettings]);
      const updateTemplate: SetTemplate = (next) => {
        setTemplateSettings((current) => (typeof next === 'function' ? next(current) : next));
      };
      return <DnsTab templateSettings={templateSettings} setTemplateSettings={updateTemplate} />;
    }

    renderWithProviders(<Harness />);
    const script = screen.getByRole('textbox', { name: 'Lua Script' });
    fireEvent.change(script, { target: { value: '/etc/xray/dns.lua' } });
    expect(latest().dns.script).toBe('/etc/xray/dns.lua');

    fireEvent.change(script, { target: { value: '' } });
    expect(latest().dns).not.toHaveProperty('script');
  });
});

describe('DnsTab server reorder', () => {
  beforeEach(installPointerEvents);
  afterEach(() => vi.unstubAllGlobals());

  const google = { address: '8.8.8.8', domains: ['geosite:google'] };
  const rows = () => document.querySelectorAll<HTMLElement>('tr[data-row-key]');
  const servers = () =>
    JSON.parse(screen.getByTestId('settings').textContent || '{}').dns.servers as unknown[];

  function openServers() {
    function Harness() {
      const [settings, setSettings] = useState<XraySettingsValue | null>({
        dns: { servers: ['1.1.1.1', google, 'localhost'] },
      } as unknown as XraySettingsValue);
      return (
        <>
          <DnsTab templateSettings={settings} setTemplateSettings={setSettings} />
          <output data-testid="settings">{JSON.stringify(settings)}</output>
        </>
      );
    }
    renderWithProviders(<Harness />);
    fireEvent.click(screen.getByRole('tab', { name: 'DNS' }));
  }

  it.each([
    {
      dir: 'down',
      from: 0,
      to: 2,
      marker: 'drop-after',
      expected: [google, 'localhost', '1.1.1.1'],
    },
    {
      dir: 'up',
      from: 2,
      to: 0,
      marker: 'drop-before',
      expected: ['localhost', '1.1.1.1', google],
    },
  ])('drags a server $dir into the target slot', ({ from, to, marker, expected }) => {
    openServers();
    const handle = rows()[from].querySelector('.drag-handle');
    if (!handle) throw new Error('missing drag handle');
    pointAt(rows()[to]);
    const pointer = { pointerId: 7, button: 0, clientX: 10, clientY: 32 };
    fireEvent.pointerDown(handle, { ...pointer, clientY: 0 });
    fireEvent.pointerMove(document, pointer);
    expect(rows()[to].className).toContain(marker);
    fireEvent.pointerUp(document, pointer);
    expect(servers()).toEqual(expected);
  });

  it('moves a server from the row menu and disables Move up on the first row', async () => {
    openServers();
    fireEvent.click(within(rows()[0]).getByRole('button', { name: 'more' }));
    const moveUp = await screen.findByRole('menuitem', { name: /Move up/ });
    expect(moveUp.getAttribute('aria-disabled')).toBe('true');
    fireEvent.click(screen.getByRole('menuitem', { name: /Move down/ }));
    expect(servers()).toEqual([google, '1.1.1.1', 'localhost']);
  });
});
