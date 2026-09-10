import { describe, it, expect } from 'vitest';
import { AllSettingSchema } from '@/schemas/setting';
import { AllSetting } from '@/models/setting';

describe('subInfoNode settings', () => {
  it('defaults on AllSetting', () => {
    const s = new AllSetting();
    expect(s.subInfoNodeEnable).toBe(false);
    expect(s.subExpiredTemplate).toBe('⛔ {{EMAIL}} | Expired: {{EXPIRE_DATE}}');
    expect(s.subTrafficDepletedTemplate).toBe(
      '🚫 {{EMAIL}} | Traffic Depleted | {{TRAFFIC_USED}}/{{TRAFFIC_TOTAL}}',
    );
  });

  it('accepts valid values in the settings schema', () => {
    const r = AllSettingSchema.safeParse({
      subInfoNodeEnable: true,
      subExpiredTemplate: 'custom expired',
      subTrafficDepletedTemplate: 'custom depleted',
    });
    expect(r.success).toBe(true);
  });

  it('rejects invalid types', () => {
    expect(AllSettingSchema.safeParse({ subInfoNodeEnable: 'true' }).success).toBe(false);
  });
});
