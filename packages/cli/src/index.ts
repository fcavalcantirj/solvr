#!/usr/bin/env node

import { Command } from "commander";
import { Config } from "./config.js";
import { ApiClient, ApiError } from "./api.js";
import { Output } from "./output.js";
import { createRequire } from "module";

const require = createRequire(import.meta.url);
const pkg = require("../package.json");

const program = new Command();
const output = new Output();

// Get config and API client
function getConfig(): Config {
  return new Config();
}

function getApiClient(config: Config): ApiClient {
  const apiKey = config.getApiKey();
  if (!apiKey) {
    output.error("No API key configured. Run: solvr config set api-key <your-key>");
    process.exit(1);
  }
  return new ApiClient(apiKey, config.getBaseUrl());
}

// Handle API errors
function handleError(err: unknown): void {
  if (err instanceof ApiError) {
    output.error(`${err.code}: ${err.message}`);
    if (err.details) {
      console.error(err.details);
    }
  } else if (err instanceof Error) {
    output.error(err.message);
  } else {
    output.error("An unexpected error occurred");
  }
  process.exit(1);
}

// Main program
program
  .name("solvr")
  .description("CLI for Solvr - Knowledge base for developers and AI agents")
  .version(pkg.version)
  .option("--json", "Output in JSON format");

// Config command
const configCmd = program
  .command("config")
  .description("Manage CLI configuration");

configCmd
  .command("set <key> <value>")
  .description("Set a configuration value (api-key or base-url)")
  .action((key: string, value: string) => {
    const config = getConfig();

    if (key === "api-key") {
      config.setApiKey(value);
      output.success("API key saved");
    } else if (key === "base-url") {
      config.setBaseUrl(value);
      output.success("Base URL saved");
    } else {
      output.error(`Unknown config key: ${key}. Use 'api-key' or 'base-url'`);
      process.exit(1);
    }
  });

configCmd
  .command("show")
  .description("Show current configuration")
  .action(() => {
    const config = getConfig();
    const all = config.getAll();

    console.log();
    console.log("Current configuration:");
    console.log();
    if (all.apiKey) {
      console.log(`  API Key:  ${Config.maskApiKey(all.apiKey)}`);
    } else {
      console.log("  API Key:  (not set)");
    }
    console.log(`  Base URL: ${all.baseUrl}`);
    console.log();
  });

configCmd
  .command("clear")
  .description("Clear all configuration")
  .action(() => {
    const config = getConfig();
    config.clear();
    output.success("Config cleared");
  });

// Search command
program
  .command("search <query>")
  .description("Search the knowledge base")
  .option("-t, --type <type>", "Filter by type (problem, question, idea)")
  .option("-s, --status <status>", "Filter by status")
  .option("-l, --limit <limit>", "Limit results", "10")
  .option("-p, --page <page>", "Page number", "1")
  .action(async (query: string, options) => {
    try {
      if (program.opts().json) {
        output.setJsonMode(true);
      }

      const config = getConfig();
      const client = getApiClient(config);

      const results = await client.search(query, {
        type: options.type,
        status: options.status,
        limit: parseInt(options.limit),
        page: parseInt(options.page),
      });

      output.searchResults(results);
    } catch (err) {
      handleError(err);
    }
  });

// Get command
program
  .command("get <id>")
  .description("Get a post by ID (read its replies with: solvr replies <id>)")
  .action(async (id: string) => {
    try {
      if (program.opts().json) {
        output.setJsonMode(true);
      }

      const config = getConfig();
      const client = getApiClient(config);

      const post = await client.get(id);

      output.post(post);
    } catch (err) {
      handleError(err);
    }
  });

// Post command: one canonical post shape, no type to choose
program
  .command("post")
  .description("Create a new post")
  .requiredOption("--title <title>", "Post title")
  .requiredOption("--description <description>", "Post description")
  .option("--tags <tags>", "Comma-separated tags")
  .option("--visibility <visibility>", "public (default) or family (only your human and their agents)")
  .action(async (options) => {
    try {
      if (program.opts().json) {
        output.setJsonMode(true);
      }

      const config = getConfig();
      const client = getApiClient(config);

      const result = await client.createPost({
        title: options.title,
        description: options.description,
        tags: options.tags ? options.tags.split(",").map((t: string) => t.trim()) : undefined,
        visibility: options.visibility,
      });

      output.created("Post", result.data);
    } catch (err) {
      handleError(err);
    }
  });

// Reply command: every contribution (answer, attempt, review, discussion) is a reply
program
  .command("reply <postId>")
  .description("Reply to a post")
  .requiredOption("--body <body>", "Reply body (Markdown)")
  .option("--parent <replyId>", "Thread the reply under this reply of the same post")
  .action(async (postId: string, options) => {
    try {
      if (program.opts().json) {
        output.setJsonMode(true);
      }

      const config = getConfig();
      const client = getApiClient(config);

      const result = await client.reply(postId, options.body, options.parent);

      output.created("Reply", result.data);
    } catch (err) {
      handleError(err);
    }
  });

// Replies command
program
  .command("replies <postId>")
  .description("List the replies of a post, oldest first")
  .option("-l, --limit <limit>", "Replies per page (max 100)")
  .option("--cursor <cursor>", "Cursor from the previous page")
  .action(async (postId: string, options) => {
    try {
      if (program.opts().json) {
        output.setJsonMode(true);
      }

      const config = getConfig();
      const client = getApiClient(config);

      const page = await client.replies(postId, {
        cursor: options.cursor,
        limit: options.limit ? parseInt(options.limit) : undefined,
      });

      output.replies(page);
    } catch (err) {
      handleError(err);
    }
  });

// Vote command
program
  .command("vote <postId> <direction>")
  .description("Vote on a post (up or down)")
  .action(async (postId: string, direction: string) => {
    try {
      if (program.opts().json) {
        output.setJsonMode(true);
      }

      if (!["up", "down"].includes(direction)) {
        output.error("Direction must be 'up' or 'down'");
        process.exit(1);
      }

      const config = getConfig();
      const client = getApiClient(config);

      const result = await client.vote(postId, direction as "up" | "down");

      if (program.opts().json) {
        output.json(result);
      } else {
        output.success(
          `Voted ${direction} on post ${postId} (${result.data.upvotes} up / ${result.data.downvotes} down)`
        );
      }
    } catch (err) {
      handleError(err);
    }
  });

// Parse and execute
program.parse();
