import { describe, expect, it } from 'vitest';

import { AllSetting } from '@/models/setting';
import { AllSettingSchema } from '@/schemas/setting';

describe('subscription profile mode', () => {
  it.each([undefined, {}, { subProfileUrl: '' }, { subProfileUrl: '   ' }])(
    'defaults to no link without a legacy URL: %j',
    (data) => {
      expect(new AllSetting(data).subProfileMode).toBe('none');
    },
  );

  it('preserves a legacy custom URL when its mode is missing', () => {
    const setting = new AllSetting({ subProfileUrl: 'https://example.com/profile' });
    expect(setting.subProfileMode).toBe('custom');
    expect(setting.subProfileUrl).toBe('https://example.com/profile');
  });

  it.each(['none', 'builtin', 'custom'])(
    'honors the explicit %s mode with a stored URL',
    (mode) => {
      const result = AllSettingSchema.safeParse({
        subProfileMode: mode,
        subProfileUrl: 'https://example.com/profile',
      });
      expect(result.success).toBe(true);
      if (!result.success) return;
      expect(new AllSetting(result.data).subProfileMode).toBe(mode);
    },
  );

  it.each(['auto', '', null, true])('rejects an invalid mode: %j', (mode) => {
    expect(AllSettingSchema.safeParse({ subProfileMode: mode }).success).toBe(false);
  });
});
