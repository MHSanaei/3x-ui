import { describe, it, expect } from 'vitest';
import { AllSettingSchema } from '@/schemas/setting';

describe('subUpdates range', () => {
  it('accepts values the backend allows (gte=0, lte=525600)', () => {
    for (const v of [0, 12, 168, 720, 525600]) {
      const r = AllSettingSchema.safeParse({ subUpdates: v });
      expect(r.success, `subUpdates=${v} should be valid`).toBe(true);
    }
  });

  it('rejects values outside the backend range', () => {
    for (const v of [-1, 525601, 1.5]) {
      expect(AllSettingSchema.safeParse({ subUpdates: v }).success, `subUpdates=${v} should be invalid`).toBe(false);
    }
  });
});
