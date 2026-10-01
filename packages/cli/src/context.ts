import { InvalidArgumentError } from "commander";
import { Config } from "./config.js";
import { ApiClient } from "./api.js";
import type { Output } from "./output.js";

/** A usage error of the CLI itself (reported without an API error code). */
export class CliError extends Error {}

/** What the commands share: the output, the configuration and the clients of each credential. */
export interface Context {
  output: Output;
  config(): Config;
  /** A client presenting the API key; without one, required fails and otherwise the client is anonymous. */
  apiClient(required: boolean): ApiClient;
  /** A client presenting the room token given, or the one `solvr room join <slug>` saved. */
  roomClient(slug: string, token?: string): ApiClient;
  /** A client presenting no credential. */
  anonymousClient(): ApiClient;
}

/** Parses an integer option value. */
export function integer(value: string): number {
  const n = Number(value);
  if (!Number.isInteger(n)) {
    throw new InvalidArgumentError("Not an integer.");
  }
  return n;
}

/** A comma-separated list, trimmed, without empty items. */
export function list(value?: string): string[] | undefined {
  return value === undefined ? undefined : value.split(",").map((item) => item.trim()).filter(Boolean);
}

export function createContext(output: Output): Context {
  const config = () => new Config();
  return {
    output,
    config,
    apiClient(required) {
      const c = config();
      const apiKey = c.getApiKey();
      if (!apiKey && required) {
        throw new CliError("No API key configured. Run: solvr config set api-key <your-key>");
      }
      return new ApiClient(apiKey ?? null, c.getBaseUrl());
    },
    roomClient(slug, token) {
      const c = config();
      const roomToken = token || c.getRoomToken(slug);
      if (!roomToken) {
        throw new CliError(`No room token for ${slug}. Run: solvr room join ${slug}`);
      }
      return new ApiClient(null, c.getBaseUrl()).withRoomToken(roomToken);
    },
    anonymousClient() {
      return new ApiClient(null, config().getBaseUrl());
    },
  };
}
