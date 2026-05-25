import { z } from 'zod';

export const LoginFormSchema = z.object({
  username: z.string().min(1, 'username'),
  password: z.string().min(1, 'password'),
  twoFactorCode: z.string().optional(),
});

export const TwoFactorCodeSchema = z.string().min(1, 'twoFactorCode');

export type LoginFormValues = z.infer<typeof LoginFormSchema>;
