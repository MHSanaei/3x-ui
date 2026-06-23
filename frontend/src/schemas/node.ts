import { z } from 'zod';

export const NodeRecordSchema = z.object({
  id: z.number(),
  name: z.string().optional(),
  remark: z.string().optional(),
  scheme: z.string().optional(),
  address: z.string().optional(),
  port: z.number().optional(),
  basePath: z.string().optional(),
  apiToken: z.string().optional(),
  enable: z.boolean().optional(),
  status: z.string().optional(),
  latencyMs: z.number().optional(),
  cpuPct: z.number().optional(),
  memPct: z.number().optional(),
  xrayVersion: z.string().optional(),
  panelVersion: z.string().optional(),
  uptimeSecs: z.number().optional(),
  inboundCount: z.number().optional(),
  clientCount: z.number().optional(),
  onlineCount: z.number().optional(),
  activeCount: z.number().optional(),
  disabledCount: z.number().optional(),
  depletedCount: z.number().optional(),
  lastHeartbeat: z.number().optional(),
  lastError: z.string().optional(),
  // Xray state captured from the remote node's own /panel/api/server/status.
  // Lets the nodes list show a distinct indicator when the panel API is reachable
  // (status=online) but the Xray core on that node has failed.
  xrayState: z.string().optional(),
  xrayError: z.string().optional(),
  allowPrivateAddress: z.boolean().optional(),
  tlsVerifyMode: z.enum(['verify', 'skip', 'pin', 'mtls']).optional(),
  pinnedCertSha256: z.string().optional(),
  inboundSyncMode: z.enum(['all', 'selected']).optional(),
  // Backend serializes a nil []string as null for nodes saved before #5178.
  inboundTags: z.array(z.string()).nullish(),
  outboundTag: z.string().optional(),
  // Multi-hop node tree (#4983): a node's stable GUID, its parent's GUID, and
  // whether it's a read-only transitive sub-node surfaced from a downstream node.
  guid: z.string().optional(),
  parentGuid: z.string().optional(),
  transitive: z.boolean().optional(),
}).loose();

export const NodeListSchema = z.array(NodeRecordSchema);

export const ProbeResultSchema = z.object({
  status: z.string(),
  latencyMs: z.number().optional(),
  xrayVersion: z.string().optional(),
  error: z.string().optional(),
  // Present on successful probe; used to surface "connected to panel, but xray failed on node".
  xrayState: z.string().optional(),
  xrayError: z.string().optional(),
}).loose();

export const NodeFormSchema = z.object({
  id: z.number().optional(),
  name: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  remark: z.string().optional(),
  scheme: z.enum(['http', 'https']),
  address: z.string().trim().min(1, 'pages.nodes.toasts.fillRequired'),
  port: z.number().int().min(1).max(65535),
  basePath: z.string(),
  // mTLS nodes authenticate via the client certificate, so the token is optional
  // there; every other verify mode still requires one (matches remote.do()).
  apiToken: z.string().trim(),
  enable: z.boolean(),
  allowPrivateAddress: z.boolean(),
  tlsVerifyMode: z.enum(['verify', 'skip', 'pin', 'mtls']),
  pinnedCertSha256: z.string().optional().default(''),
  inboundSyncMode: z.enum(['all', 'selected']).optional().default('all'),
  // Unmounted when sync mode is "all" (absent from antd onFinish values) and
  // serialized as null by the backend for a nil slice — tolerate both.
  inboundTags: z.array(z.string()).nullish().transform((tags) => tags ?? []),
  outboundTag: z.string().optional(),
}).superRefine((val, ctx) => {
  if (val.tlsVerifyMode !== 'mtls' && val.apiToken.length === 0) {
    ctx.addIssue({
      code: 'custom',
      path: ['apiToken'],
      message: 'pages.nodes.toasts.fillRequired',
    });
  }
});

export type NodeRecord = z.infer<typeof NodeRecordSchema>;
export type ProbeResult = z.infer<typeof ProbeResultSchema>;
export type NodeFormValues = z.infer<typeof NodeFormSchema>;
