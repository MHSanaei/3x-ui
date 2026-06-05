import { z } from 'zod';

// Field-level schemas mirror the server-side rules in web/service/user.go so
// client and server validation agree. Messages are i18n keys resolved by the
// `antdRule` adapter at validation time.
export const FullNameSchema = z
  .string()
  .trim()
  .min(2, 'pages.register.errors.fullName')
  .max(100, 'pages.register.errors.fullName');

// Optional leading +, then a digit, then 4-19 digits/separators.
export const PhoneSchema = z
  .string()
  .trim()
  .regex(/^\+?[0-9][0-9 ()\-.]{4,19}$/, 'pages.register.errors.phone');

export const EmailSchema = z
  .string()
  .trim()
  .max(254, 'pages.register.errors.email')
  .regex(/^[^\s@]+@[^\s@]+\.[^\s@]+$/, 'pages.register.errors.email');

export const UsernameSchema = z
  .string()
  .trim()
  .regex(/^[A-Za-z0-9_]{3,32}$/, 'pages.register.errors.username');

export const PasswordSchema = z
  .string()
  .min(8, 'pages.register.errors.password')
  .refine(
    (pw) => /[a-z]/.test(pw) && /[A-Z]/.test(pw) && /[0-9]/.test(pw),
    'pages.register.errors.password',
  );

export interface RegisterFormValues {
  fullName: string;
  phone: string;
  email: string;
  username: string;
  password: string;
  confirmPassword: string;
}

/**
 * passwordScore returns a 0-4 strength score used by the strength meter. It
 * rewards length and character-class diversity. Purely advisory — the binding
 * acceptance rule is PasswordSchema (and the server) which require length 8 +
 * mixed case + a digit.
 */
export function passwordScore(pw: string): number {
  if (!pw) return 0;
  let score = 0;
  if (pw.length >= 8) score++;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) score++;
  if (/[0-9]/.test(pw)) score++;
  if (/[^A-Za-z0-9]/.test(pw)) score++;
  if (pw.length >= 12 && score >= 3) score++;
  return Math.min(score, 4);
}
