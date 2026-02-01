/**
 * Configuration for the Solvr MCP server.
 * Reads environment variables for API key and URL.
 */

export interface Config {
  apiKey: string;
  apiUrl: string;
}

const DEFAULT_API_URL = 'https://api.solvr.dev';

/**
 * Loads configuration from environment variables.
 * @throws Error if SOLVR_API_KEY is missing
 */
export function loadConfig(): Config {
  const apiKey = process.env.SOLVR_API_KEY;

  if (!apiKey) {
    throw new Error('SOLVR_API_KEY environment variable is required');
  }

  const apiUrl = process.env.SOLVR_API_URL || DEFAULT_API_URL;

  return {
    apiKey,
    apiUrl,
  };
}
