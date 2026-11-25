import { describe, expect, it } from 'vitest';

import { ClientCreateFormSchema, ClientFormSchema } from '@/schemas/client';

describe('client name validation', () => {
  it.each(['.', '..', ' . ', ' .. '])('rejects the reserved name %j on create and edit', (name) => {
    for (const schema of [ClientCreateFormSchema, ClientFormSchema]) {
      const result = schema.shape.email.safeParse(name);
      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.issues[0].message).toBe('pages.clients.emailReservedName');
      }
    }
  });

  it.each(['...', '.alice', 'alice.', 'alice.example', 'alice..example', 'alice@example.com'])(
    'preserves ordinary dotted names: %s',
    (name) => {
      expect(ClientFormSchema.shape.email.parse(name)).toBe(name);
    },
  );
});
