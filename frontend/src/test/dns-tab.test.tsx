import { useEffect, useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

import DnsTab from '@/pages/xray/dns/DnsTab';
import type { SetTemplate, XraySettingsValue } from '@/hooks/useXraySetting';
import { renderWithProviders } from './test-utils';

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
