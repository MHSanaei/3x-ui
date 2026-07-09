import type { z } from 'zod';

export function rhfZodValidate<S extends z.ZodType>(schema: S) {
  return (value: unknown): string | true => {
    const result = schema.safeParse(value);
    if (result.success) return true;
    return result.error.issues[0]?.message ?? 'validation.invalid';
  };
}
