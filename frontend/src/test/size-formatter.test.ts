import { describe, expect, it } from 'vitest';
import { SizeFormatter } from '@/utils';

describe('SizeFormatter.sizeFormat', () => {
  it('formats zero and negative values', () => {
    expect(SizeFormatter.sizeFormat(0)).toBe('0 B');
    expect(SizeFormatter.sizeFormat(-1)).toBe('0 B');
    expect(SizeFormatter.sizeFormat(null)).toBe('0 B');
    expect(SizeFormatter.sizeFormat(undefined)).toBe('0 B');
  });

  it('formats bytes', () => {
    expect(SizeFormatter.sizeFormat(512)).toBe('512 B');
  });

  it('formats kilobytes', () => {
    expect(SizeFormatter.sizeFormat(1536)).toBe('1.50 KB');
  });
});

describe('SizeFormatter.speedFormat', () => {
  it('formats zero and negative values', () => {
    expect(SizeFormatter.speedFormat(0)).toBe('0 B/s');
    expect(SizeFormatter.speedFormat(-1)).toBe('0 B/s');
    expect(SizeFormatter.speedFormat(null)).toBe('0 B/s');
    expect(SizeFormatter.speedFormat(undefined)).toBe('0 B/s');
  });

  it('formats non-finite values as zero', () => {
    expect(SizeFormatter.speedFormat(NaN)).toBe('0 B/s');
    expect(SizeFormatter.speedFormat(Infinity)).toBe('0 B/s');
    expect(SizeFormatter.sizeFormat(NaN)).toBe('0 B');
    expect(SizeFormatter.sizeFormat(Infinity)).toBe('0 B');
  });

  it('formats bytes per second', () => {
    expect(SizeFormatter.speedFormat(512)).toBe('512 B/s');
    expect(SizeFormatter.speedFormat(1023)).toBe('1023 B/s');
  });

  it('formats kilobytes per second', () => {
    expect(SizeFormatter.speedFormat(1024)).toBe('1.00 KB/s');
    expect(SizeFormatter.speedFormat(1536)).toBe('1.50 KB/s');
  });

  it('formats megabytes per second', () => {
    expect(SizeFormatter.speedFormat(1024 * 1024)).toBe('1.00 MB/s');
    expect(SizeFormatter.speedFormat(2.5 * 1024 * 1024)).toBe('2.50 MB/s');
  });

  it('formats gigabytes per second', () => {
    expect(SizeFormatter.speedFormat(1024 * 1024 * 1024)).toBe('1.00 GB/s');
  });
});
