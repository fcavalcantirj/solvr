import { describe, it, expect } from 'vitest';
import { APIError, isNotFoundError, isServerError } from './api-error';

describe('api-error', () => {
  describe('APIError', () => {
    it('should create error with status code', () => {
      const error = new APIError('Not found', 404);
      expect(error.message).toBe('Not found');
      expect(error.statusCode).toBe(404);
      expect(error.name).toBe('APIError');
    });
  });

  describe('isNotFoundError', () => {
    it('should return true for 404 APIError', () => {
      const error = new APIError('Not found', 404);
      expect(isNotFoundError(error)).toBe(true);
    });

    it('should return false for other status codes', () => {
      const error = new APIError('Server error', 500);
      expect(isNotFoundError(error)).toBe(false);
    });

    it('should return false for regular Error', () => {
      const error = new Error('Some error');
      expect(isNotFoundError(error)).toBe(false);
    });
  });

  describe('isServerError', () => {
    it('should return true for 500 APIError', () => {
      const error = new APIError('Server error', 500);
      expect(isServerError(error)).toBe(true);
    });

    it('should return true for 503 APIError', () => {
      const error = new APIError('Service unavailable', 503);
      expect(isServerError(error)).toBe(true);
    });

    it('should return false for 404 APIError', () => {
      const error = new APIError('Not found', 404);
      expect(isServerError(error)).toBe(false);
    });

    it('should return false for regular Error', () => {
      const error = new Error('Some error');
      expect(isServerError(error)).toBe(false);
    });
  });
});
