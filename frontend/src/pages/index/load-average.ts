export type LoadSeverity =
  | 'healthy'
  | 'increasing'
  | 'elevated'
  | 'nearCapacity'
  | 'aboveCapacity'
  | 'unavailable';

interface LoadAppearance {
  value: number | null;
  ratio: number | null;
  severity: LoadSeverity;
  color: string | null;
}

function interpolate(from: number[], to: number[], fraction: number): string {
  return (
    '#' +
    from
      .map((channel, i) =>
        Math.round(channel + (to[i] - channel) * fraction)
          .toString(16)
          .padStart(2, '0'),
      )
      .join('')
  );
}

export function loadAppearance(
  load: number | null | undefined,
  logicalCPUs: number | null | undefined,
  comparable = true,
): LoadAppearance {
  const value = typeof load === 'number' && Number.isFinite(load) && load >= 0 ? load : null;
  const neutral: LoadAppearance = { value, ratio: null, severity: 'unavailable', color: null };
  if (
    value === null ||
    !comparable ||
    typeof logicalCPUs !== 'number' ||
    !Number.isFinite(logicalCPUs) ||
    logicalCPUs <= 0
  )
    return neutral;
  const ratio = value / logicalCPUs;
  if (ratio > 1) return { value, ratio, severity: 'aboveCapacity', color: '#ef4444' };
  if (ratio >= 0.85) return { value, ratio, severity: 'nearCapacity', color: '#f97316' };
  if (ratio >= 0.7)
    return {
      value,
      ratio,
      severity: 'elevated',
      color: interpolate([234, 179, 8], [249, 115, 22], (ratio - 0.7) / 0.15),
    };
  if (ratio > 0.5)
    return {
      value,
      ratio,
      severity: 'increasing',
      color: interpolate([34, 197, 94], [234, 179, 8], (ratio - 0.5) / 0.2),
    };
  return { value, ratio, severity: 'healthy', color: '#22c55e' };
}
