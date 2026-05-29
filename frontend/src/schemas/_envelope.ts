import { z } from 'zod';

export const msgSchema = <T extends z.ZodType>(obj: T) =>
  z.object({
    success: z.boolean(),
    msg: z.string().default(''),
    obj: obj.nullable(),
  });

export type MsgOf<S extends z.ZodType> = z.infer<ReturnType<typeof msgSchema<S>>>;
