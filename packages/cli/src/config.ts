import * as fs from "fs";
import * as path from "path";
import * as os from "os";

interface ConfigData {
  apiKey?: string;
  baseUrl?: string;
}

const DEFAULT_BASE_URL = "https://api.solvr.dev";

/**
 * Manages CLI configuration stored in ~/.solvr/config.json
 */
export class Config {
  private configPath: string;
  private data: ConfigData;

  constructor(configPath?: string) {
    this.configPath =
      configPath ||
      process.env.SOLVR_CONFIG_PATH ||
      path.join(os.homedir(), ".solvr", "config.json");
    this.data = this.load();
  }

  private load(): ConfigData {
    try {
      if (fs.existsSync(this.configPath)) {
        const content = fs.readFileSync(this.configPath, "utf-8");
        return JSON.parse(content);
      }
    } catch {
      // Ignore parse errors, return empty config
    }
    return {};
  }

  private save(): void {
    const dir = path.dirname(this.configPath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    fs.writeFileSync(this.configPath, JSON.stringify(this.data, null, 2));
  }

  /**
   * Get the stored API key
   */
  getApiKey(): string | undefined {
    return this.data.apiKey || process.env.SOLVR_API_KEY;
  }

  /**
   * Set the API key
   */
  setApiKey(apiKey: string): void {
    this.data.apiKey = apiKey;
    this.save();
  }

  /**
   * Get the base URL for API requests
   */
  getBaseUrl(): string {
    return this.data.baseUrl || process.env.SOLVR_BASE_URL || DEFAULT_BASE_URL;
  }

  /**
   * Set the base URL
   */
  setBaseUrl(baseUrl: string): void {
    this.data.baseUrl = baseUrl;
    this.save();
  }

  /**
   * Clear all stored configuration
   */
  clear(): void {
    this.data = {};
    if (fs.existsSync(this.configPath)) {
      fs.unlinkSync(this.configPath);
    }
  }

  /**
   * Get all configuration values
   */
  getAll(): { apiKey?: string; baseUrl: string } {
    return {
      apiKey: this.getApiKey(),
      baseUrl: this.getBaseUrl(),
    };
  }

  /**
   * Mask an API key for display (show only last 4 chars)
   */
  static maskApiKey(apiKey: string): string {
    if (apiKey.length <= 8) {
      return "solvr_sk_****";
    }
    return `solvr_sk_...${apiKey.slice(-4)}`;
  }
}
