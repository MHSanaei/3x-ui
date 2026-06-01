import { describe, it, expect } from 'vitest';

import { isPanelUpdateAvailable } from '@/lib/panel-version';

// Parity with web/service/panel.go isNewerVersion.
describe('isPanelUpdateAvailable', () => {
  it('flags a strictly newer latest', () => {
    expect(isPanelUpdateAvailable('2.6.5', '2.6.4')).toBe(true);
    expect(isPanelUpdateAvailable('v2.7.0', 'v2.6.9')).toBe(true);
    expect(isPanelUpdateAvailable('3.0.0', '2.9.9')).toBe(true);
  });

  it('returns false when equal or the node is ahead', () => {
    expect(isPanelUpdateAvailable('2.6.4', '2.6.4')).toBe(false);
    expect(isPanelUpdateAvailable('v2.6.4', '2.6.4')).toBe(false);
    expect(isPanelUpdateAvailable('2.6.4', '2.6.5')).toBe(false);
  });

  it('ignores a leading v on either side', () => {
    expect(isPanelUpdateAvailable('v2.6.5', '2.6.4')).toBe(true);
    expect(isPanelUpdateAvailable('2.6.5', 'v2.6.4')).toBe(true);
  });

  it('never flags when a version is unknown', () => {
    expect(isPanelUpdateAvailable('', '2.6.4')).toBe(false);
    expect(isPanelUpdateAvailable('2.6.5', '')).toBe(false);
  });

  it('falls back to string inequality for non-semver tags', () => {
    expect(isPanelUpdateAvailable('nightly-2', 'nightly-1')).toBe(true);
    expect(isPanelUpdateAvailable('nightly-1', 'nightly-1')).toBe(false);
  });
});
