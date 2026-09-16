import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, within } from '@testing-library/react';
import { EditorView } from 'codemirror';

import { AllSetting } from '@/models/setting';
import HappSettingsContent from '@/pages/settings/HappSettingsContent';
import { toBase64Utf8 } from '@/pages/settings/happPresets';

import { renderWithProviders } from './test-utils';

const profile = {
  Name: '当前草稿',
  GlobalProxy: 'true',
  RouteOrder: 'block-proxy-direct',
  RemoteDNSDomain: 'https://dns.example/dns-query',
  DomainStrategy: 'IPIfNonMatch',
  DnsHosts: { 'dns.example': '1.1.1.1' },
  DirectSites: ['geosite:cn', 'regexp:^example{1,2}\\.com$'],
  ProxySites: ['geosite:google'],
  BlockSites: ['geosite:category-ads-all'],
  DirectIp: ['geoip:cn'],
  ProxyIp: ['1.1.1.1/32'],
  BlockIp: ['0.0.0.0/8'],
  FutureSetting: { nested: [{ enabled: false, count: 0, text: '保留' }] },
};

function routingLink(value: Record<string, unknown>, mode = 'onadd') {
  return `happ://routing/${mode}/${toBase64Utf8(JSON.stringify(value))}`;
}

function renderSettings(input = routingLink(profile)) {
  const updateSetting = vi.fn<(patch: Partial<AllSetting>) => void>();

  function SettingsHarness() {
    const [allSetting, setAllSetting] = useState(() =>
      Object.assign(new AllSetting(), { subRoutingRules: input }),
    );

    return (
      <HappSettingsContent
        allSetting={allSetting}
        updateSetting={(patch) => {
          updateSetting(patch);
          setAllSetting((previous) => Object.assign(new AllSetting(), previous, patch));
        }}
        isMobile={false}
        remoteSourceBadge={() => null}
      />
    );
  }

  renderWithProviders(<SettingsHarness />);
  return updateSetting;
}

function openEditor() {
  fireEvent.click(screen.getByRole('button', { name: 'Visual Rule Generator' }));
  return within(screen.getByRole('dialog'));
}

function routingRules() {
  return (screen.getByRole('textbox', { name: 'Routing rules' }) as HTMLTextAreaElement).value;
}

function generatedProfile(): Record<string, unknown> {
  const payload = routingRules().replace(/^happ:\/\/routing\/(?:onadd|add)\//, '');
  return JSON.parse(
    new TextDecoder().decode(Uint8Array.from(atob(payload), (char) => char.charCodeAt(0))),
  ) as Record<string, unknown>;
}

function basicField(name: string) {
  return screen.getByRole('textbox', { name }) as HTMLTextAreaElement;
}

function jsonEditor() {
  const content = screen.getByRole('textbox', { name: 'JSON editor' });
  const editor = EditorView.findFromDOM(content);
  if (!editor) throw new Error('CodeMirror editor not mounted');
  return editor;
}

function changeJson(value: string) {
  const editor = jsonEditor();
  act(() => {
    editor.dispatch({ changes: { from: 0, to: editor.state.doc.length, insert: value } });
  });
}

describe('Happ routing editor', () => {
  it('loads the current unsaved routing draft when opened', () => {
    const updateSetting = renderSettings();
    const latest = { ...profile, DirectSites: ['domain:latest-draft.example'] };
    fireEvent.change(screen.getByRole('textbox', { name: 'Routing rules' }), {
      target: { value: routingLink(latest) },
    });
    updateSetting.mockClear();

    const dialog = openEditor();

    expect((dialog.getAllByRole('textbox')[0] as HTMLTextAreaElement).value).toBe(
      'domain:latest-draft.example',
    );
    expect(updateSetting).not.toHaveBeenCalled();
  });

  it.each(['add', 'onadd'])(
    'preserves other fields and %s semantics when editing basic rules',
    (mode) => {
      const updateSetting = renderSettings(routingLink(profile, mode));
      openEditor();
      const edited = 'regexp:^other{1,2}\\.com$\n  domain:another.example\n';
      fireEvent.change(basicField('Direct Domains (Bypass)'), { target: { value: edited } });

      expect(basicField('Direct Domains (Bypass)').value).toBe(edited);
      expect(updateSetting).not.toHaveBeenCalled();
      fireEvent.click(screen.getByRole('button', { name: 'Generate Deeplink' }));

      expect(routingRules().startsWith(`happ://routing/${mode}/`)).toBe(true);
      expect(generatedProfile()).toEqual({
        ...profile,
        DirectSites: ['regexp:^other{1,2}\\.com$', 'domain:another.example'],
      });
      expect(updateSetting).toHaveBeenCalledExactlyOnceWith({ subRoutingRules: routingRules() });
      expect(screen.queryByRole('dialog')).toBeNull();
    },
  );

  it('syncs both tabs while retaining full-profile edits', () => {
    const updateSetting = renderSettings();
    openEditor();
    fireEvent.change(basicField('Blocked Domains (Ad/Malware)'), {
      target: { value: 'geosite:category-ads-all\ndomain:ads.example' },
    });
    fireEvent.click(screen.getByRole('tab', { name: 'Advanced editor' }));
    const advanced = JSON.parse(jsonEditor().state.doc.toString()) as Record<string, unknown>;
    expect(advanced).toEqual({
      ...profile,
      BlockSites: ['geosite:category-ads-all', 'domain:ads.example'],
    });

    const next = { ...advanced, ProxyIp: ['9.9.9.9/32'], FutureSetting: { deep: ['new'] } };
    changeJson(JSON.stringify(next, null, 2));
    fireEvent.click(screen.getByRole('tab', { name: 'Basic rules' }));
    expect(basicField('Proxy IPs / CIDRs').value).toBe('9.9.9.9/32');
    expect(basicField('Blocked Domains (Ad/Malware)').value).toBe(
      'geosite:category-ads-all\ndomain:ads.example',
    );
    expect(updateSetting).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Generate Deeplink' }));
    expect(generatedProfile()).toEqual(next);
  });

  it.each(['{"Name":', '{"DirectSites": [42]}'])(
    'retains invalid JSON %s until repaired',
    (invalid) => {
      const updateSetting = renderSettings();
      openEditor();
      fireEvent.click(screen.getByRole('tab', { name: 'Advanced editor' }));
      changeJson(invalid);

      const generate = screen.getByRole('button', {
        name: 'Generate Deeplink',
      }) as HTMLButtonElement;
      expect(generate.disabled).toBe(true);
      expect(screen.getByRole('alert').textContent).toContain('arrays of strings');
      fireEvent.click(screen.getByRole('tab', { name: 'Basic rules' }));
      expect(
        screen.getByRole('tab', { name: 'Advanced editor' }).getAttribute('aria-selected'),
      ).toBe('true');
      expect(jsonEditor().state.doc.toString()).toBe(invalid);
      fireEvent.click(generate);
      expect(updateSetting).not.toHaveBeenCalled();

      const repaired = { ...profile, Name: 'Repaired', BlockIp: ['192.0.2.0/24'] };
      changeJson(JSON.stringify(repaired));
      expect(generate.disabled).toBe(false);
      fireEvent.click(screen.getByRole('tab', { name: 'Basic rules' }));
      expect(basicField('Blocked IPs / CIDRs').value).toBe('192.0.2.0/24');
      fireEvent.click(screen.getByRole('button', { name: 'Generate Deeplink' }));
      expect(generatedProfile()).toEqual(repaired);
    },
  );

  it('discards canceled edits and reloads the latest parent draft on reopening', () => {
    const updateSetting = renderSettings();
    openEditor();
    fireEvent.change(basicField('Direct Domains (Bypass)'), {
      target: { value: 'discard.example' },
    });
    fireEvent.click(screen.getByRole('tab', { name: 'Advanced editor' }));
    changeJson('{invalid');
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(updateSetting).not.toHaveBeenCalled();
    expect(routingRules()).toBe(routingLink(profile));

    const latest = { ...profile, DirectSites: ['latest.example'] };
    fireEvent.change(screen.getByRole('textbox', { name: 'Routing rules' }), {
      target: { value: JSON.stringify(latest) },
    });
    updateSetting.mockClear();
    openEditor();
    expect(screen.getByRole('tab', { name: 'Basic rules' }).getAttribute('aria-selected')).toBe(
      'true',
    );
    expect(basicField('Direct Domains (Bypass)').value).toBe('latest.example');
    expect(screen.queryByRole('alert')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Generate Deeplink' }));
    expect(generatedProfile()).toEqual(latest);
    expect(updateSetting).toHaveBeenCalledTimes(1);
  });

  it.each([
    ['https://example.com/DEFAULT.DEEPLINK', 'Remote URLs cannot be loaded here'],
    ['happ://routing/off', 'Routing is disabled'],
    ['happ://routing/onadd/not-base64', 'not a valid Happ routing link'],
    ['{"BlockIp":42}', 'not a valid Happ routing link'],
  ])('blocks generation for unsupported current rules: %s', (input, message) => {
    const updateSetting = renderSettings(input);
    openEditor();

    expect(screen.getByRole('alert').textContent).toContain(message);
    const generate = screen.getByRole('button', { name: 'Generate Deeplink' }) as HTMLButtonElement;
    expect(generate.disabled).toBe(true);
    fireEvent.click(generate);
    expect(updateSetting).not.toHaveBeenCalled();
    expect(routingRules()).toBe(input);
  });

  it('starts a new custom profile when the current draft is blank', () => {
    const updateSetting = renderSettings('');
    openEditor();
    expect(screen.getByRole('alert').textContent).toContain('No current rules');
    fireEvent.change(basicField('Direct Domains (Bypass)'), {
      target: { value: 'domain:local.example\n' },
    });
    expect(updateSetting).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Generate Deeplink' }));
    expect(routingRules().startsWith('happ://routing/onadd/')).toBe(true);
    expect(generatedProfile()).toEqual({
      Name: 'Custom Rules',
      GlobalProxy: 'true',
      DirectSites: ['domain:local.example'],
      DirectIp: [],
      ProxySites: [],
      ProxyIp: [],
      BlockSites: [],
      BlockIp: [],
      DomainStrategy: 'IPIfNonMatch',
    });
  });

  it('does not add absent list fields to an unchanged raw JSON profile', () => {
    const minimal = { Name: 'Minimal', FutureSetting: profile.FutureSetting };
    renderSettings(JSON.stringify(minimal));
    openEditor();
    fireEvent.click(screen.getByRole('tab', { name: 'Advanced editor' }));
    expect(JSON.parse(jsonEditor().state.doc.toString())).toEqual(minimal);
    fireEvent.click(screen.getByRole('button', { name: 'Generate Deeplink' }));
    expect(generatedProfile()).toEqual(minimal);
  });
});
