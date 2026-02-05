import { describe, it, expect } from 'vitest';
import { mapStatusFilter, mapSortFilter, mapTimeframeFilter } from './filter-utils';

describe('filter-utils', () => {
  describe('mapStatusFilter', () => {
    it('should return undefined for "All"', () => {
      expect(mapStatusFilter('All')).toBeUndefined();
    });

    it('should map "Open" to "open"', () => {
      expect(mapStatusFilter('Open')).toBe('open');
    });

    it('should map "In Progress" to "active"', () => {
      expect(mapStatusFilter('In Progress')).toBe('active');
    });

    it('should map "Solved" to "solved"', () => {
      expect(mapStatusFilter('Solved')).toBe('solved');
    });

    it('should map "Stuck" to "stuck"', () => {
      expect(mapStatusFilter('Stuck')).toBe('stuck');
    });
  });

  describe('mapSortFilter', () => {
    it('should map "Newest" to "new"', () => {
      expect(mapSortFilter('Newest')).toBe('new');
    });

    it('should map "Trending" to "hot"', () => {
      expect(mapSortFilter('Trending')).toBe('hot');
    });

    it('should map "Most Voted" to "top"', () => {
      expect(mapSortFilter('Most Voted')).toBe('top');
    });

    it('should map "Needs Help" to "new" (default)', () => {
      expect(mapSortFilter('Needs Help')).toBe('new');
    });
  });

  describe('mapTimeframeFilter', () => {
    it('should return undefined for "All Time"', () => {
      expect(mapTimeframeFilter('All Time')).toBeUndefined();
    });

    it('should map "Today" to "today"', () => {
      expect(mapTimeframeFilter('Today')).toBe('today');
    });

    it('should map "This Week" to "week"', () => {
      expect(mapTimeframeFilter('This Week')).toBe('week');
    });

    it('should map "This Month" to "month"', () => {
      expect(mapTimeframeFilter('This Month')).toBe('month');
    });
  });
});
