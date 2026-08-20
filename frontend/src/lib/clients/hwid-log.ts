// Shape of one entry in a client's HWID (registered-device) log, as returned
// by POST /panel/api/clients/hwids/:email.
export type ClientHwidInfo = {
  id: number;
  firstSeen: number;
  lastSeen: number;
  userAgent: string;
  deviceOs: string;
  osVersion: string;
  deviceModel: string;
};

// normalizeClientHwids accepts the API payload and returns typed entries,
// dropping anything that isn't a real HWID row (missing/non-numeric id).
export function normalizeClientHwids(obj: unknown): ClientHwidInfo[] {
  if (!Array.isArray(obj)) return [];
  return obj.filter(
    (x): x is ClientHwidInfo =>
      !!x && typeof x === 'object' && typeof (x as ClientHwidInfo).id === 'number',
  );
}
