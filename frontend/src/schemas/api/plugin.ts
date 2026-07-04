import { z } from 'zod';

export const PluginEntrySchema = z.object({
  runtime: z.enum(['wasm', 'process', 'http']).or(z.string()),
  path: z.string().optional(),
  command: z.string().optional(),
  args: z.array(z.string()).optional(),
  env: z.record(z.string(), z.string()).optional(),
}).loose();

export const PluginPermissionSchema = z.object({
  name: z.string(),
  scope: z.string(),
  reason: z.string(),
}).loose();

export const PluginHookSchema = z.object({
  name: z.string(),
  handler: z.string(),
  priority: z.number().int(),
}).loose();

export const PluginUISchema = z.object({
  zone: z.string(),
  label: z.string(),
  route: z.string().optional(),
  component: z.string().optional(),
}).loose();

export const PluginManifestSchema = z.object({
  schemaVersion: z.string(),
  id: z.string(),
  name: z.string(),
  version: z.string(),
  description: z.string(),
  author: z.string(),
  homepage: z.string().optional(),
  entry: PluginEntrySchema,
  permissions: z.array(PluginPermissionSchema),
  hooks: z.array(PluginHookSchema),
  ui: z.array(PluginUISchema),
  config: z.record(z.string(), z.unknown()),
}).loose();
export type PluginManifest = z.infer<typeof PluginManifestSchema>;

export const PluginRecordSchema = z.object({
  id: z.string(),
  name: z.string(),
  version: z.string(),
  description: z.string(),
  author: z.string(),
  enabled: z.boolean(),
  status: z.string(),
  installedAt: z.string().optional(),
  packagePath: z.string().optional(),
  manifest: PluginManifestSchema,
}).loose();
export type PluginRecord = z.infer<typeof PluginRecordSchema>;

export const PluginCapabilitiesSchema = z.object({
  runtimes: z.array(z.string()),
  hooks: z.array(z.string()),
  permissions: z.array(z.string()),
  uiZones: z.array(z.string()),
}).loose();
export type PluginCapabilities = z.infer<typeof PluginCapabilitiesSchema>;

export const PluginCatalogSchema = z.object({
  manifestVersion: z.string(),
  capabilities: PluginCapabilitiesSchema,
  installed: z.array(PluginRecordSchema),
  template: PluginManifestSchema,
}).loose();
export type PluginCatalog = z.infer<typeof PluginCatalogSchema>;
