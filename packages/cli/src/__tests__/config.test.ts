import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { Config } from "../config.js";
import * as fs from "fs";
import * as path from "path";
import * as os from "os";

describe("Config", () => {
  let configDir: string;
  let configPath: string;
  let config: Config;

  beforeEach(() => {
    // Use a temp directory for testing
    configDir = path.join(os.tmpdir(), `solvr-test-${Date.now()}`);
    configPath = path.join(configDir, "config.json");
    config = new Config(configPath);
  });

  afterEach(() => {
    // Clean up test directory
    if (fs.existsSync(configDir)) {
      fs.rmSync(configDir, { recursive: true });
    }
  });

  describe("getApiKey", () => {
    it("returns undefined when no API key is set", () => {
      expect(config.getApiKey()).toBeUndefined();
    });

    it("returns the API key when set", () => {
      config.setApiKey("solvr_sk_test123");
      expect(config.getApiKey()).toBe("solvr_sk_test123");
    });
  });

  describe("setApiKey", () => {
    it("stores the API key", () => {
      config.setApiKey("solvr_sk_mykey");
      expect(config.getApiKey()).toBe("solvr_sk_mykey");
    });

    it("creates the config directory if it doesn't exist", () => {
      expect(fs.existsSync(configDir)).toBe(false);
      config.setApiKey("solvr_sk_test");
      expect(fs.existsSync(configDir)).toBe(true);
    });

    it("persists the API key to disk", () => {
      config.setApiKey("solvr_sk_persistent");

      // Create new Config instance to verify persistence
      const newConfig = new Config(configPath);
      expect(newConfig.getApiKey()).toBe("solvr_sk_persistent");
    });
  });

  describe("getBaseUrl", () => {
    it("returns default base URL when not set", () => {
      expect(config.getBaseUrl()).toBe("https://api.solvr.dev");
    });

    it("returns custom base URL when set", () => {
      config.setBaseUrl("http://localhost:8080");
      expect(config.getBaseUrl()).toBe("http://localhost:8080");
    });
  });

  describe("setBaseUrl", () => {
    it("stores the base URL", () => {
      config.setBaseUrl("https://custom.api.com");
      expect(config.getBaseUrl()).toBe("https://custom.api.com");
    });
  });

  describe("clear", () => {
    it("removes all stored config", () => {
      config.setApiKey("solvr_sk_test");
      config.setBaseUrl("http://localhost:8080");

      config.clear();

      expect(config.getApiKey()).toBeUndefined();
      expect(config.getBaseUrl()).toBe("https://api.solvr.dev");
    });
  });

  describe("getAll", () => {
    it("returns all config values", () => {
      config.setApiKey("solvr_sk_all");
      config.setBaseUrl("http://custom.url");

      const all = config.getAll();
      expect(all.apiKey).toBe("solvr_sk_all");
      expect(all.baseUrl).toBe("http://custom.url");
    });

    it("returns empty config when nothing is set", () => {
      const all = config.getAll();
      expect(all.apiKey).toBeUndefined();
      expect(all.baseUrl).toBe("https://api.solvr.dev");
    });
  });
});
