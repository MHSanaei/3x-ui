import { useForm } from 'react-hook-form';
import type { FieldValues, Resolver, UseFormProps, UseFormReturn } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import type { z } from 'zod';

export function useZodForm<TFieldValues extends FieldValues>(
  schema: z.ZodType<TFieldValues>,
  options?: Omit<UseFormProps<TFieldValues>, 'resolver'>,
): UseFormReturn<TFieldValues> {
  const resolver = zodResolver(schema as z.ZodType<TFieldValues, TFieldValues>) as Resolver<TFieldValues>;
  return useForm<TFieldValues>({
    mode: 'onSubmit',
    reValidateMode: 'onChange',
    shouldUnregister: false,
    ...options,
    resolver,
  });
}
