import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { loadConfig, Config } from './config.js';

describe('Config', () => {
  const originalEnv = process.env;

  beforeEach(() => {
    vi.resetModules();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  describe('loadConfig', () => {
    it('loads SOLVR_API_KEY from environment', () => {
      process.env.SOLVR_API_KEY = 'solvr_test_key_123';
      const config = loadConfig();
      expect(config.apiKey).toBe('solvr_test_key_123');
    });

    it('loads SOLVR_API_URL from environment', () => {
      process.env.SOLVR_API_KEY = 'solvr_test_key';
      process.env.SOLVR_API_URL = 'https://api.custom.solvr.dev';
      const config = loadConfig();
      expect(config.apiUrl).toBe('https://api.custom.solvr.dev');
    });

    it('uses default API URL when SOLVR_API_URL not set', () => {
      process.env.SOLVR_API_KEY = 'solvr_test_key';
      delete process.env.SOLVR_API_URL;
      const config = loadConfig();
      expect(config.apiUrl).toBe('https://api.solvr.dev');
    });

    it('throws error when SOLVR_API_KEY is missing', () => {
      delete process.env.SOLVR_API_KEY;
      expect(() => loadConfig()).toThrow('SOLVR_API_KEY environment variable is required');
    });

    it('throws error when SOLVR_API_KEY is empty', () => {
      process.env.SOLVR_API_KEY = '';
      expect(() => loadConfig()).toThrow('SOLVR_API_KEY environment variable is required');
    });

    it('returns correct Config shape', () => {
      process.env.SOLVR_API_KEY = 'solvr_my_key';
      process.env.SOLVR_API_URL = 'https://api.test.solvr.dev';
      const config = loadConfig();

      expect(config).toMatchObject<Config>({
        apiKey: 'solvr_my_key',
        apiUrl: 'https://api.test.solvr.dev',
      });
    });
  });
});
