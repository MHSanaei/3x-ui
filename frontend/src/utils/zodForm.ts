import type { Rule } from 'antd/es/form';
import type { TFunction } from 'i18next';
import type { z } from 'zod';

export function antdRule<T extends z.ZodType>(schema: T, t: TFunction): Rule {
  return {
    validator: async (_rule, value) => {
      const result = schema.safeParse(value);
      if (result.success) return;
      const issue = result.error.issues[0];
      const key = issue?.message ?? 'validation.invalid';
      throw new Error(t(key, { defaultValue: key }));
    },
  };
}
