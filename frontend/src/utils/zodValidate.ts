import type { z } from 'zod';
import { Msg } from '@/utils';

export function parseMsg<T extends z.ZodType>(
  msg: Msg<unknown>,
  schema: T,
  context: string,
): Msg<z.infer<T>> {
  if (!msg.success) {
    return msg as Msg<z.infer<T>>;
  }
  const result = schema.safeParse(msg.obj);
  if (!result.success) {
    console.warn(`[zod] ${context} response failed validation`, result.error.issues);
    throw new Error(`${context} response failed validation`);
  }
  return new Msg<z.infer<T>>(msg.success, msg.msg, result.data);
}
