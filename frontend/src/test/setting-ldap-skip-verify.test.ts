import { describe, it, expect } from 'vitest';
import { AllSettingSchema } from '@/schemas/setting';
import { AllSetting } from '@/models/setting';

describe('ldapInsecureSkipVerify', () => {
  it('parses through the Zod schema', () => {
    const r = AllSettingSchema.safeParse({ ldapInsecureSkipVerify: true });
    expect(r.success).toBe(true);
    expect(r.success && r.data.ldapInsecureSkipVerify).toBe(true);
  });

  it('rejects non-boolean values', () => {
    expect(AllSettingSchema.safeParse({ ldapInsecureSkipVerify: 'yes' }).success).toBe(false);
  });

  it('defaults to false on the model and clones from payload', () => {
    expect(new AllSetting().ldapInsecureSkipVerify).toBe(false);
    expect(new AllSetting({ ldapInsecureSkipVerify: true }).ldapInsecureSkipVerify).toBe(true);
  });
});
