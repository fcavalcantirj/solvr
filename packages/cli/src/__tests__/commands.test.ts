import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { execSync } from "child_process";
import * as fs from "fs";
import * as path from "path";
import * as os from "os";

// Helper to run CLI commands in test mode
const runCli = (args: string, env: Record<string, string> = {}) => {
  try {
    const result = execSync(`npx tsx src/index.ts ${args}`, {
      cwd: path.resolve(__dirname, "../.."),
      encoding: "utf-8",
      env: { ...process.env, ...env, SOLVR_TEST_MODE: "true" },
      stdio: ["pipe", "pipe", "pipe"],
    });
    return { stdout: result, stderr: "", exitCode: 0 };
  } catch (error: any) {
    return {
      stdout: error.stdout?.toString() || "",
      stderr: error.stderr?.toString() || "",
      exitCode: error.status || 1,
    };
  }
};

describe("CLI Commands", () => {
  describe("--help", () => {
    it("displays help message", () => {
      const { stdout } = runCli("--help");
      expect(stdout).toContain("solvr");
      expect(stdout).toContain("search");
      expect(stdout).toContain("get");
      expect(stdout).toContain("post");
      expect(stdout).toContain("config");
    });
  });

  describe("--version", () => {
    it("displays version", () => {
      const { stdout } = runCli("--version");
      expect(stdout).toMatch(/\d+\.\d+\.\d+/);
    });
  });

  describe("config command", () => {
    let configPath: string;

    beforeEach(() => {
      configPath = path.join(os.tmpdir(), `solvr-cli-test-${Date.now()}`);
    });

    afterEach(() => {
      if (fs.existsSync(configPath)) {
        fs.rmSync(configPath, { recursive: true });
      }
    });

    it("sets api-key", () => {
      const { stdout, exitCode } = runCli("config set api-key solvr_sk_test123", {
        SOLVR_CONFIG_PATH: configPath,
      });
      expect(exitCode).toBe(0);
      expect(stdout).toContain("API key saved");
    });

    it("sets base-url", () => {
      const { stdout, exitCode } = runCli("config set base-url http://localhost:8080", {
        SOLVR_CONFIG_PATH: configPath,
      });
      expect(exitCode).toBe(0);
      expect(stdout).toContain("Base URL saved");
    });

    it("shows current config", () => {
      // First set some config
      runCli("config set api-key solvr_sk_show", { SOLVR_CONFIG_PATH: configPath });

      const { stdout } = runCli("config show", { SOLVR_CONFIG_PATH: configPath });
      expect(stdout).toContain("solvr_sk_...show");
      expect(stdout).toContain("api.solvr.dev");
    });

    it("clears config", () => {
      runCli("config set api-key solvr_sk_clear", { SOLVR_CONFIG_PATH: configPath });

      const { stdout, exitCode } = runCli("config clear", { SOLVR_CONFIG_PATH: configPath });
      expect(exitCode).toBe(0);
      expect(stdout).toContain("Config cleared");
    });
  });
});

describe("CLI Output Formatting", () => {
  describe("--json flag", () => {
    it("outputs raw JSON when --json is specified", () => {
      // This test will be more meaningful with mocked API responses
      const { stdout, exitCode } = runCli("--help --json");
      // --json with --help should still show help
      expect(stdout).toContain("solvr");
    });
  });
});
