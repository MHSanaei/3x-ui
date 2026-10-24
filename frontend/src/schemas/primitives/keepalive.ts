import { z } from 'zod';

const invalidKeepAlive = 'pages.clients.amneziaWgKeepAliveInvalid';
const MAX_KEEPALIVE_SECONDS = 65535;

export const AmneziaWGKeepAliveRangeSchema = z.string().refine((value) => {
  if (!/^(0|[1-9]\d*)-(0|[1-9]\d*)$/.test(value)) return false;
  const [minimum, maximum] = value.split('-').map(Number);
  return minimum >= 0 && minimum <= maximum && maximum <= MAX_KEEPALIVE_SECONDS;
}, invalidKeepAlive);

// Numeric values stay permissive here so older stored values remain readable.
export const ClientKeepAliveSchema = z.union([z.number(), AmneziaWGKeepAliveRangeSchema]);
export const OptionalClientKeepAliveSchema = z.preprocess(
  (value) => (value == null ? undefined : value),
  ClientKeepAliveSchema.optional(),
);

const boundedSeconds = z
  .number()
  .refine(
    (value) => Number.isInteger(value) && value >= 0 && value <= MAX_KEEPALIVE_SECONDS,
    invalidKeepAlive,
  );

export const OptionalNumericKeepAliveInputSchema = z.preprocess(
  (value) => (value == null || value === '' ? undefined : value),
  boundedSeconds.optional(),
);

export const OptionalAmneziaWGKeepAliveInputSchema = z.preprocess(
  (value) => (value == null || value === '' ? undefined : value),
  z.union([boundedSeconds, AmneziaWGKeepAliveRangeSchema]).optional(),
);

export function keepAliveInputValue(value: unknown): number | string | undefined {
  if (value == null || value === '') return undefined;
  const text = String(value);
  return /^\d+$/.test(text) ? Number(text) : text;
}
