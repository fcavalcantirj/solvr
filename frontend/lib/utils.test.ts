import { describe, it, expect } from 'vitest';
import { formatCount } from './utils';

describe('formatCount', () => {
  it('should return numbers under 1000 as-is', () => {
    expect(formatCount(0)).toBe('0');
    expect(formatCount(1)).toBe('1');
    expect(formatCount(42)).toBe('42');
    expect(formatCount(999)).toBe('999');
  });

  it('should format thousands with K suffix', () => {
    expect(formatCount(1000)).toBe('1K');
    expect(formatCount(1200)).toBe('1.2K');
    expect(formatCount(1234)).toBe('1.2K');
    expect(formatCount(9999)).toBe('10K');
  });

  it('should format tens of thousands', () => {
    expect(formatCount(10000)).toBe('10K');
    expect(formatCount(12400)).toBe('12.4K');
    expect(formatCount(48200)).toBe('48.2K');
    expect(formatCount(99999)).toBe('100K');
  });

  it('should format hundreds of thousands', () => {
    expect(formatCount(100000)).toBe('100K');
    expect(formatCount(500000)).toBe('500K');
    expect(formatCount(999999)).toBe('1000K');
  });

  it('should format millions with M suffix', () => {
    expect(formatCount(1000000)).toBe('1M');
    expect(formatCount(1500000)).toBe('1.5M');
    expect(formatCount(2700000)).toBe('2.7M');
    expect(formatCount(10000000)).toBe('10M');
  });

  it('should format hundreds of millions', () => {
    expect(formatCount(100000000)).toBe('100M');
    expect(formatCount(500000000)).toBe('500M');
  });

  it('should format billions with B suffix', () => {
    expect(formatCount(1000000000)).toBe('1B');
    expect(formatCount(2500000000)).toBe('2.5B');
  });

  it('should handle negative numbers', () => {
    expect(formatCount(-100)).toBe('-100');
    expect(formatCount(-1200)).toBe('-1.2K');
    expect(formatCount(-1000000)).toBe('-1M');
  });
});
