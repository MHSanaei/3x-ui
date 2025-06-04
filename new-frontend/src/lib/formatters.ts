export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

export function formatUptime(seconds: number): string {
  if (seconds <= 0) return 'N/A';
  const d = Math.floor(seconds / (3600 * 24));
  const h = Math.floor((seconds % (3600 * 24)) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);

  let result = '';
  if (d > 0) result += `${d}d `;
  if (h > 0) result += `${h}h `;
  if (m > 0) result += `${m}m `;
  if (s > 0 || result === '') result += `${s}s`; // show seconds if other units are zero or if it's the only unit
  return result.trim();
}

export function formatPercentage(value: number, total: number): number {
  if (total === 0) return 0;
  return parseFloat(((value / total) * 100).toFixed(2));
}

export function toFixedIfNecessary( value: number | undefined, fractionDigits: number ): string {
    if (value === undefined) return 'N/A';
    return value.toFixed(fractionDigits);
}
