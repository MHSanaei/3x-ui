import type { z } from 'zod';

export function rhfZodValidate<S extends z.ZodType>(schema: S) {
  return (value: unknown): string | true => {
    const result = schema.safeParse(value);
    if (result.success) return true;
    return result.error.issues[0]?.message ?? 'validation.invalid';
  };
}

export function rhfZodValidateAsync<S extends z.ZodType>(schema: S) {
  return async (value: unknown): Promise<string | true> => {
    const result = await schema.safeParseAsync(value);
    if (result.success) return true;
    return result.error.issues[0]?.message ?? 'validation.invalid';
  };
}
