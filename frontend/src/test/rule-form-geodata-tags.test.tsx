import { describe, it, expect, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

import RuleFormModal from '@/pages/xray/routing/RuleFormModal';

import { renderWithProviders } from './test-utils';

function domainInput(): HTMLInputElement {
  const control = document.getElementById('domain');
  const select = control?.closest('.ant-select') as HTMLElement;
  return select.querySelector('input') as HTMLInputElement;
}

function selectedTags(fieldId: string): string[] {
  const control = document.getElementById(fieldId);
  const select = control?.closest('.ant-select') as HTMLElement;
  return Array.from(select.querySelectorAll('.ant-select-selection-item')).map(
    (el) => el.getAttribute('title') ?? el.textContent ?? '',
  );
}

describe('RuleFormModal domain/ip tags autocomplete', () => {
  it('renders a comma-separated existing value as tags and preserves it unchanged on save', () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <RuleFormModal
        open
        rule={{ type: 'field', domain: 'google.com,geosite:cn', enabled: true }}
        inboundTags={[]}
        outboundTags={['block']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    expect(selectedTags('domain')).toEqual(['google.com', 'geosite:cn']);

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0]).toMatchObject({ domain: ['google.com', 'geosite:cn'] });
  });

  it('adds a typed value to the tag list on Enter and includes it on save', () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['block']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    const input = domainInput();
    fireEvent.change(input, { target: { value: 'example.com' } });
    fireEvent.keyDown(input, { key: 'Enter', code: 'Enter', keyCode: 13, which: 13 });

    fireEvent.click(screen.getByRole('button', { name: 'Create' }));
    expect(onConfirm.mock.calls[0][0]).toMatchObject({ domain: ['example.com'] });
  });

  it('commits a typed value on blur even without pressing Enter or a comma', () => {
    // Regression test: a plain Select mode="tags" only commits the search
    // text on Enter/tokenSeparator. Clicking Save directly is a blur, not an
    // Enter -- without TagsAutocomplete's onBlur commit, the typed value was
    // silently dropped and the rule saved with no `domain` key at all.
    const onConfirm = vi.fn();
    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['block']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    const input = domainInput();
    fireEvent.change(input, { target: { value: 'blurred.com' } });
    // A real click on Save blurs the still-focused input first (native
    // browser focus handling); jsdom's fireEvent.click doesn't replicate
    // that side effect, so blur is fired explicitly to match what a real
    // click does immediately before Save's own handler runs.
    fireEvent.blur(input);
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0]).toMatchObject({ domain: ['blurred.com'] });
  });

  it('omits domain entirely when left empty', () => {
    const onConfirm = vi.fn();
    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['block']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0]).not.toHaveProperty('domain');
  });
});
