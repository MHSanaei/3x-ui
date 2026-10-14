import { z } from 'zod';

const ColorRegex = /^(#([0-9a-fA-F]{3,8})|rgba?\([^)]+\)|transparent)$/;

export const ThemeTokenSchema = z
  .object({
    id: z.string().min(1).max(32),
    name: z.string().min(1).max(48),
    baseMode: z.enum(['light', 'dark']),

    // Brand colors
    colorPrimary: z.string().regex(ColorRegex, 'Invalid hex color for colorPrimary'),
    colorPrimaryHover: z
      .string()
      .regex(ColorRegex, 'Invalid hex color for colorPrimaryHover')
      .optional(),

    // Surfaces & Backgrounds
    colorBgBase: z.string().regex(ColorRegex, 'Invalid hex color for colorBgBase'),
    colorBgContainer: z.string().regex(ColorRegex, 'Invalid hex color for colorBgContainer'),
    colorBgElevated: z.string().regex(ColorRegex, 'Invalid hex color for colorBgElevated'),

    // Borders & Text
    colorBorder: z.string().regex(ColorRegex, 'Invalid hex color for colorBorder'),
    colorText: z.string().regex(ColorRegex, 'Invalid hex color for colorText').optional(),
    colorTextSecondary: z
      .string()
      .regex(ColorRegex, 'Invalid hex color for colorTextSecondary')
      .optional(),

    // Layout, Shape & Density
    borderRadius: z.number().int().min(0).max(16),
    density: z.enum(['default', 'compact']).default('default'),
    cardStyle: z.enum(['modern', 'flat']).default('modern'),
  })
  .strict();

export type ThemeTokens = z.infer<typeof ThemeTokenSchema>;
