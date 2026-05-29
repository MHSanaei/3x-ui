import { z } from 'zod';

export const DefaultsPayloadSchema = z.object({
  expireDiff: z.number().optional(),
  trafficDiff: z.number().optional(),
  tgBotEnable: z.boolean().optional(),
  subEnable: z.boolean().optional(),
  subTitle: z.string().optional(),
  subURI: z.string().optional(),
  subJsonURI: z.string().optional(),
  subJsonEnable: z.boolean().optional(),
  subClashURI: z.string().optional(),
  subClashEnable: z.boolean().optional(),
  pageSize: z.number().optional(),
  remarkModel: z.string().optional(),
  datepicker: z.enum(['gregorian', 'jalalian']).optional(),
  ipLimitEnable: z.boolean().optional(),
}).loose();

export type DefaultsPayload = z.infer<typeof DefaultsPayloadSchema>;
