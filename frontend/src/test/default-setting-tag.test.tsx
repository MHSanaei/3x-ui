import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { keys } from '@/api/queryKeys';
import DefaultSettingTag, { matchesFactoryDefault } from '@/components/ui/DefaultSettingTag';
import { makeTestQueryClient, renderWithProviders } from './test-utils';

function clientWithDefaults(defaults: Record<string, string>) {
  const queryClient = makeTestQueryClient();
  queryClient.setQueryData(keys.settings.factoryDefaults(), defaults);
  return queryClient;
}

describe('matchesFactoryDefault', () => {
  it('compares by value with type-aware coercion', () => {
    expect(matchesFactoryDefault(2096, '2096')).toBe(true);
    expect(matchesFactoryDefault(8443, '2096')).toBe(false);
    expect(matchesFactoryDefault(true, 'true')).toBe(true);
    expect(matchesFactoryDefault(false, 'true')).toBe(false);
    expect(matchesFactoryDefault('/sub/', '/sub/')).toBe(true);
    expect(matchesFactoryDefault('/other/', '/sub/')).toBe(false);
  });

  it('never matches when the key has no shipped default', () => {
    expect(matchesFactoryDefault(2096, undefined)).toBe(false);
  });

  it('rejects blank or unparsable defaults instead of coercing them', () => {
    expect(matchesFactoryDefault(0, '')).toBe(false);
    expect(matchesFactoryDefault(0, '  ')).toBe(false);
    expect(matchesFactoryDefault(0, 'none')).toBe(false);
    expect(matchesFactoryDefault(false, '')).toBe(false);
    expect(matchesFactoryDefault(false, 'no')).toBe(false);
  });
});

describe('DefaultSettingTag', () => {
  it('shows the tag when the current value equals the shipped default, however it got there', () => {
    renderWithProviders(<DefaultSettingTag settingKey="subPort" value={2096} />, {
      queryClient: clientWithDefaults({ subPort: '2096' }),
    });

    expect(screen.getByText('Default')).toBeDefined();
  });

  it('renders nothing when the value differs from the default', () => {
    renderWithProviders(<DefaultSettingTag settingKey="subPort" value={8443} />, {
      queryClient: clientWithDefaults({ subPort: '2096' }),
    });

    expect(screen.queryByText('Default')).toBeNull();
  });

  it('renders nothing while defaults are unknown', () => {
    renderWithProviders(<DefaultSettingTag settingKey="subPort" value={2096} />, {
      queryClient: makeTestQueryClient(),
    });

    expect(screen.queryByText('Default')).toBeNull();
  });
});
