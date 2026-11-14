export const COUNTRY_BINS = 5;

// Logarithmic, since one country usually dwarfs the rest, and spanning the data's own
// range: anchored at 1, a narrow spread would land every country in the top bins.
export function countryBin(count: number, lowest: number, peak: number): number {
  if (peak <= lowest) return COUNTRY_BINS;
  const position = Math.log(count / lowest) / Math.log(peak / lowest);
  return Math.min(Math.max(Math.floor(position * COUNTRY_BINS) + 1, 1), COUNTRY_BINS);
}

export function countryName(code: string, language: string): string {
  try {
    return new Intl.DisplayNames([language], { type: 'region' }).of(code) ?? code;
  } catch {
    return code;
  }
}
