import { z } from 'zod';

export const CurTotalInputSchema = z.object({
  current: z.number().optional(),
  total: z.number().optional(),
});

export const NetIOSchema = z.object({
  up: z.number(),
  down: z.number(),
});

export const NetTrafficSchema = z.object({
  sent: z.number(),
  recv: z.number(),
});

export const PublicIPSchema = z.object({
  ipv4: z.union([z.string(), z.number()]),
  ipv6: z.union([z.string(), z.number()]),
});

export const AppStatsSchema = z.object({
  threads: z.number(),
  mem: z.number(),
  uptime: z.number(),
});

export const XrayInfoSchema = z.object({
  state: z.string(),
  errorMsg: z.string(),
  version: z.string(),
  color: z.string(),
}).partial();

export const StatusSchema = z.object({
  cpu: z.number().optional(),
  cpuCores: z.number().optional(),
  logicalPro: z.number().optional(),
  cpuSpeedMhz: z.number().optional(),
  disk: CurTotalInputSchema.optional(),
  loads: z.array(z.number()).optional(),
  mem: CurTotalInputSchema.optional(),
  netIO: NetIOSchema.optional(),
  netTraffic: NetTrafficSchema.optional(),
  publicIP: PublicIPSchema.optional(),
  swap: CurTotalInputSchema.optional(),
  tcpCount: z.number().optional(),
  udpCount: z.number().optional(),
  uptime: z.number().optional(),
  appUptime: z.number().optional(),
  appStats: AppStatsSchema.optional(),
  xray: XrayInfoSchema.optional(),
});

export type StatusInput = z.infer<typeof StatusSchema>;
