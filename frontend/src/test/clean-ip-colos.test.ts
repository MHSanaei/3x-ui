import { describe, expect, it } from 'vitest';

import { coloPlace, flagOf } from '@/pages/clean-ip/colos';

describe('coloPlace', () => {
  it('names the datacenter city and country in the panel language', () => {
    expect(coloPlace('FRA', 'en-US')).toEqual({
      country: 'DE',
      countryName: 'Germany',
      city: 'Frankfurt',
    });
    expect(coloPlace('FRA', 'fa-IR')).toEqual({
      country: 'DE',
      countryName: 'آلمان',
      city: 'فرانکفورت',
    });
  });

  it('returns null for a colo the table does not know, so the page shows the raw code', () => {
    expect(coloPlace('ZZZ', 'en-US')).toBeNull();
  });
});

describe('flagOf', () => {
  it('turns a country code into its flag emoji', () => {
    expect(flagOf('DE')).toBe('🇩🇪');
    expect(flagOf('NL')).toBe('🇳🇱');
  });
});
