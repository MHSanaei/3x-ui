import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import i18next from 'i18next';

import TwoFactorModal from '@/pages/settings/TwoFactorModal';
import skSK from '../../../internal/web/translation/sk-SK.json';

vi.mock('antd', async (importOriginal) => ({
  ...(await importOriginal<typeof import('antd')>()),
  QRCode: ({ value }: { value: string }) => <output data-testid="enrollment-uri">{value}</output>,
}));

i18next.addResourceBundle('sk-SK', 'translation', skSK, true, true);

afterEach(async () => {
  cleanup();
  await act(async () => {
    await i18next.changeLanguage('en-US');
  });
});

describe('2FA enrollment localization boundary', () => {
  it('preserves the original otpauth account and parameters across panel languages', async () => {
    for (const language of ['en-US', 'sk-SK']) {
      await act(async () => {
        await i18next.changeLanguage(language);
      });
      render(
        <TwoFactorModal
          open
          token="JBSWY3DPEHPK3PXP"
          onConfirm={() => {}}
          onOpenChange={() => {}}
        />,
      );
      const uri = new URL(screen.getByTestId('enrollment-uri').textContent ?? '');
      expect(uri.protocol).toBe('otpauth:');
      expect(uri.hostname).toBe('totp');
      expect(decodeURIComponent(uri.pathname)).toBe('/3x-ui:Administrator');
      expect(Object.fromEntries(uri.searchParams)).toEqual({
        issuer: '3x-ui',
        secret: 'JBSWY3DPEHPK3PXP',
        algorithm: 'SHA1',
        digits: '6',
        period: '30',
      });
      expect(
        screen.getByRole('button', { name: language === 'sk-SK' ? 'Zrušiť' : 'Cancel' }),
      ).toBeTruthy();
      cleanup();
    }
  });
});
