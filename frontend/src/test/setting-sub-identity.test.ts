import { describe, it, expect } from 'vitest';
import { AllSettingSchema } from '@/schemas/setting';
import { AllSetting } from '@/models/setting';

describe('subShowIdentityOnAllLinks', () => {
  it('defaults to false on AllSetting', () => {
    expect(new AllSetting().subShowIdentityOnAllLinks).toBe(false);
  });

  it('accepts boolean values in the settings schema', () => {
    for (const v of [true, false]) {
      const r = AllSettingSchema.safeParse({ subShowIdentityOnAllLinks: v });
      expect(r.success, `subShowIdentityOnAllLinks=${v}`).toBe(true);
    }
  });

  it('rejects non-boolean values', () => {
    for (const v of ['true', 1, null]) {
      expect(AllSettingSchema.safeParse({ subShowIdentityOnAllLinks: v }).success).toBe(false);
    }
  });
});
