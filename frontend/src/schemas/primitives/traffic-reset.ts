/**
 * The traffic reset cycles an inbound or a client may be put on. Shared so the
 * inbound form, the client form and the bulk-add form cannot drift apart.
 */
export const TRAFFIC_RESETS = ['never', 'hourly', 'daily', 'weekly', 'monthly'] as const;

export type TrafficResetCycle = (typeof TRAFFIC_RESETS)[number];
