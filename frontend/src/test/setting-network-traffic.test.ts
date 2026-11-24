import { describe, expect, it } from 'vitest';

import { AllSettingSchema } from '@/schemas/setting';

describe('network traffic settings validation', () => {
  it('requires a selected interface for a complete manual-mode save', () => {
    const result = AllSettingSchema.safeParse({
      networkTrafficMode: 'manual',
      networkTrafficInterface: '',
    });
    expect(result.success).toBe(false);
    if (!result.success)
      expect(result.error.issues.map((issue) => issue.path)).toEqual([['networkTrafficInterface']]);
    expect(
      AllSettingSchema.safeParse({
        networkTrafficMode: 'manual',
        networkTrafficInterface: 'enP6129s1',
      }).success,
    ).toBe(true);
    expect(
      AllSettingSchema.safeParse({ networkTrafficMode: 'legacy', networkTrafficInterface: '' })
        .success,
    ).toBe(true);
    expect(
      AllSettingSchema.safeParse({ networkTrafficMode: 'auto', networkTrafficInterface: '' })
        .success,
    ).toBe(true);
  });
});
