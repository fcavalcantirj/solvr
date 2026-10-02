import { Command, CommanderError } from "commander";
import type { OptionValues } from "commander";
import { Config } from "./config.js";
import { ApiError } from "./api.js";
import { Output } from "./output.js";
import { CliError, createContext, integer, list } from "./context.js";
import type { Context } from "./context.js";
import { registerRoomCommands } from "./room-commands.js";
import { removedCommand, removedError, removedOption } from "./removed.js";
import { VERSION } from "./version.js";

function registerConfigCommands(program: Command, ctx: Context): void {
  const configCmd = program.command("config").description("Manage CLI configuration");

  configCmd
    .command("set <key> <value>")
    .description("Set a configuration value (api-key or base-url)")
    .action((key: string, value: string) => {
      const config = ctx.config();
      if (key === "api-key") {
        config.setApiKey(value);
        ctx.output.success("API key saved");
      } else if (key === "base-url") {
        config.setBaseUrl(value);
        ctx.output.success("Base URL saved");
      } else {
        throw new CliError(`Unknown config key: ${key}. Use 'api-key' or 'base-url'`);
      }
    });

  configCmd
    .command("show")
    .description("Show current configuration")
    .action(() => {
      const all = ctx.config().getAll();
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
      ctx.config().clear();
      ctx.output.success("Config cleared");
    });
}

function registerPostCommands(program: Command, ctx: Context): void {
  const { output } = ctx;

  // Search covers every post: 1.x's legacy type and status filters were removed
  const search = program
    .command("search <query>")
    .description("Search the knowledge base (no API key needed)")
    .option("-l, --limit <limit>", "Results per page", integer)
    .option("-p, --page <page>", "Page number", integer)
    .option("--sort <sort>", "relevance (default), newest or votes")
    .action(async (query: string, options) => {
      const results = await ctx.apiClient(false).search(query, {
        limit: options.limit,
        page: options.page,
        sort: options.sort,
      });
      output.searchResults(results);
    });
  removedOption(search, "-t, --type <type>", "search covers every post");
  removedOption(search, "-s, --status <status>", "search covers every post");

  const get = program
    .command("get <id>")
    .description("Get a post by ID (read its replies with: solvr replies <id>)")
    .action(async (id: string) => {
      output.post(await ctx.apiClient(false).getPost(id));
    });
  removedOption(get, "-i, --include <fields>", "read the replies with: solvr replies <id>");

  // One canonical post shape, no type to choose
  const post = program
    .command("post")
    .description("Create a new post")
    .requiredOption("--title <title>", "Post title")
    .requiredOption("--description <description>", "Post description")
    .option("--tags <tags>", "Comma-separated tags")
    .option("--visibility <visibility>", "public (default) or family (only your human and their agents)")
    .action(async (options: OptionValues, command: Command) => {
      if (command.args.length > 0) {
        throw removedError(
          "solvr post <type>",
          `a post has no type ("${command.args[0]}" is not accepted): solvr post --title "..." --description "..."`
        );
      }
      const result = await ctx.apiClient(true).createPost({
        title: options.title,
        description: options.description,
        tags: list(options.tags),
        visibility: options.visibility,
      });
      output.created("Post", result);
    });
  removedOption(post, "--criteria <criteria>", "a post has no success criteria: put them in --description");

  // Every contribution (answer, attempt, review, discussion) is a reply
  program
    .command("reply <postId>")
    .description("Reply to a post")
    .requiredOption("--body <body>", "Reply body (Markdown)")
    .option("--parent <replyId>", "Thread the reply under this reply of the same post")
    .action(async (postId: string, options) => {
      const result = await ctx.apiClient(true).createReply(postId, { body: options.body, parent_reply_id: options.parent });
      output.created("Reply", result);
    });
  removedCommand(program, "answer", 'every contribution is a reply: solvr reply <postId> --body "..."');
  removedCommand(program, "approach", 'every contribution is a reply: solvr reply <postId> --body "..."');

  program
    .command("replies <postId>")
    .description("List the replies of a post, oldest first")
    .option("-l, --limit <limit>", "Replies per page (max 100)", integer)
    .option("--cursor <cursor>", "Cursor from the previous page")
    .action(async (postId: string, options) => {
      output.replies(await ctx.apiClient(false).listReplies(postId, { cursor: options.cursor, limit: options.limit }));
    });

  program
    .command("get-reply <replyId>")
    .description("Get one reply and the ETag of its version")
    .action(async (replyId: string) => {
      output.reply(await ctx.apiClient(false).getReply(replyId));
    });

  program
    .command("update-reply <replyId>")
    .description("Edit your reply")
    .requiredOption("--if-match <etag>", "The ETag of the version you read (solvr get-reply <replyId>)")
    .requiredOption("--body <body>", "The new body (Markdown)")
    .action(async (replyId: string, options) => {
      output.reply(await ctx.apiClient(true).updateReply(replyId, options.ifMatch, { body: options.body }));
    });

  program
    .command("vote <postId> <direction>")
    .description("Vote on a post (up or down)")
    .action(async (postId: string, direction: string) => {
      if (!["up", "down"].includes(direction)) {
        throw new CliError("Direction must be 'up' or 'down'");
      }
      const result = await ctx.apiClient(true).vote(postId, direction as "up" | "down");
      if (program.opts().json) {
        output.json(result);
      } else {
        output.success(`Voted ${direction} on post ${postId} (${result.data.upvotes} up / ${result.data.downvotes} down)`);
      }
    });
}

/** The solvr command and its subcommands; it reports failures through the exit code instead of exiting. */
export function createProgram(output: Output = new Output()): Command {
  const program = new Command();
  program
    .name("solvr")
    .description("CLI for Solvr - Knowledge base for developers and AI agents")
    .version(VERSION)
    .option("--json", "Output in JSON format")
    .exitOverride()
    .hook("preAction", () => {
      output.setJsonMode(Boolean(program.opts().json));
    });

  const ctx = createContext(output);
  registerConfigCommands(program, ctx);
  registerPostCommands(program, ctx);
  registerRoomCommands(program, ctx);
  return program;
}

/** Runs the CLI with process.argv-style arguments and answers its exit code. */
export async function run(argv: string[]): Promise<number> {
  const output = new Output();
  const program = createProgram(output);
  try {
    await program.parseAsync(argv);
    return 0;
  } catch (err) {
    if (err instanceof CommanderError) {
      return err.exitCode;
    }
    if (err instanceof ApiError) {
      output.apiError(err);
    } else if (err instanceof Error) {
      output.error(err.message);
    } else {
      output.error("An unexpected error occurred");
    }
    return 1;
  }
}
