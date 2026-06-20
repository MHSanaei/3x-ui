import { describe, it, expect } from 'vitest';

import {
  REMARK_VARIABLES,
  hasRemarkTokens,
  previewRemark,
  wrapToken,
} from '@/lib/remark/remarkVariables';

describe('remark variables', () => {
  it('wrapToken / hasRemarkTokens', () => {
    expect(wrapToken('EMAIL')).toBe('{{EMAIL}}');
    expect(hasRemarkTokens('hi {{EMAIL}}')).toBe(true);
    expect(hasRemarkTokens('plain')).toBe(false);
  });

  it('previewRemark substitutes known tokens and drops unknown', () => {
    expect(previewRemark('plain text')).toBe('plain text');
    expect(previewRemark('{{EMAIL}}')).toBe('john');
    expect(previewRemark('{{EMAIL}} · {{TRAFFIC_LEFT}} · {{DAYS_LEFT}}d')).toBe('john · 41.60GB · 12d');
    expect(previewRemark('{{NOT_A_TOKEN}}')).toBe('');
  });

  it('every catalog token previews to its own sample', () => {
    for (const v of REMARK_VARIABLES) {
      expect(v.sample.length).toBeGreaterThan(0);
      expect(previewRemark(wrapToken(v.token))).toBe(v.sample);
    }
  });
});
