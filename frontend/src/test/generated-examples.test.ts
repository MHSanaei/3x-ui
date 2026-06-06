import { describe, it, expect } from 'vitest';
import type { ZodType } from 'zod';

import { EXAMPLES } from '@/generated/examples';
import * as zodSchemas from '@/generated/zod';

const registry = zodSchemas as unknown as Record<string, ZodType>;
const names = Object.keys(EXAMPLES);

describe('generated response examples', () => {
  it('has at least one example to validate', () => {
    expect(names.length).toBeGreaterThan(0);
  });

  it('pairs every example with a generated zod schema', () => {
    const missing = names.filter((name) => typeof registry[`${name}Schema`]?.safeParse !== 'function');
    expect(missing).toEqual([]);
  });

  it.each(names)('EXAMPLES.%s satisfies its generated zod schema', (name) => {
    const schema = registry[`${name}Schema`];
    const result = schema.safeParse(EXAMPLES[name]);
    if (!result.success) {
      throw new Error(
        `EXAMPLES.${name} does not match ${name}Schema:\n${JSON.stringify(result.error.issues, null, 2)}`,
      );
    }
    expect(result.success).toBe(true);
  });
});
