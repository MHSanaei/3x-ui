export type TuicCongestionController = 'bbr' | 'cubic' | 'new_reno';

export function normalizeTuicCongestionController(value: unknown): TuicCongestionController {
  if (typeof value !== 'string' || value.trim() === '') return 'bbr';

  switch (value.trim().toLowerCase()) {
    case 'bbr':
      return 'bbr';
    case 'cubic':
      return 'cubic';
    case 'reno':
    case 'new_reno':
      return 'new_reno';
    default:
      return 'new_reno';
  }
}

export function resolveTuicServerSettings(
  settings: Record<string, unknown>,
): Record<string, unknown> {
  const nested =
    settings.server && typeof settings.server === 'object' && !Array.isArray(settings.server)
      ? (settings.server as Record<string, unknown>)
      : {};
  const result: Record<string, unknown> = { ...settings };
  delete result.server;
  delete result.clients;
  for (const [key, value] of Object.entries(nested)) {
    if (value == null || value === '' || (Array.isArray(value) && value.length === 0)) continue;
    if (typeof value === 'number' && value <= 0) continue;
    result[key] = value;
  }
  return result;
}
