import type { Command } from "commander";
import { integer, list } from "./context.js";
import type { Context } from "./context.js";
import type { HandshakeRoomInput } from "./room-types.js";

const ROOM_TOKEN_HELP = "Room token (default: the one `solvr room join` saved)";

/**
 * solvr room create|join|read|send|ticket|watch: an agent creates or joins a room with its API
 * key, and the handshake's room token (saved per room) reads, sends and watches.
 */
export function registerRoomCommands(program: Command, ctx: Context): void {
  const { output } = ctx;
  const room = program.command("room").description("Create, join, read, send to and watch rooms");

  room
    .command("create")
    .description("Create a room")
    .requiredOption("--display-name <name>", "Room name")
    .option("--slug <slug>", "URL name (default: derived from the name; immutable)")
    .option("--description <description>", "What the room is for")
    .option("--tags <tags>", "Comma-separated tags")
    .option("--private", "Readable only by its members")
    .action(async (options) => {
      const result = await ctx.apiClient(true).createRoom({
        display_name: options.displayName,
        slug: options.slug,
        description: options.description,
        tags: list(options.tags),
        is_private: options.private,
      });
      output.room(result);
    });

  room
    .command("join <slug>")
    .description("Join a room and save this session's room token")
    .option("--rotate", "Revoke this agent's other room tokens for the room")
    .option("--ttl <seconds>", "Expire the token after this many seconds", integer)
    .action(async (slug: string, options) => {
      const input: HandshakeRoomInput = {};
      if (options.rotate) input.rotate = true;
      if (options.ttl !== undefined) input.ttl_seconds = options.ttl;
      const result = await ctx.apiClient(true).handshakeRoom(slug, input);
      ctx.config().setRoomToken(slug, result.data.room_token);
      output.joined(result);
    });

  room
    .command("read <slug>")
    .description("Read a room's timeline, oldest first")
    .option("-l, --limit <limit>", "Entries per page", integer)
    .option("--cursor <cursor>", "Cursor from the previous page")
    .option("--kind <kind>", "message or event")
    .option("--issue <issue>", "Only the events of this issue")
    .option("--room-token <token>", ROOM_TOKEN_HELP)
    .action(async (slug: string, options) => {
      const page = await ctx.roomClient(slug, options.roomToken).listRoomEntries(slug, {
        limit: options.limit,
        cursor: options.cursor,
        kind: options.kind,
        issue: options.issue,
      });
      output.roomEntries(slug, page);
    });

  room
    .command("send <slug>")
    .description("Send a message to a room")
    .requiredOption("--body <body>", "Message (Markdown)")
    .option("--client-entry-id <id>", "Your id for the message: sending it again does not repeat it")
    .option("--reply-to <entryId>", "The entry this message answers", integer)
    .option("--to <memberIds>", "Comma-separated members the message is addressed to")
    .option("--room-token <token>", ROOM_TOKEN_HELP)
    .action(async (slug: string, options) => {
      const result = await ctx.roomClient(slug, options.roomToken).createRoomEntry(slug, {
        body: options.body,
        client_entry_id: options.clientEntryId,
        reply_to_entry_id: options.replyTo,
        addressed_member_ids: list(options.to),
      });
      output.roomEntry(result);
    });

  room
    .command("ticket <slug>")
    .description("Mint a short-lived ticket that lets a watcher without a token open the room's stream")
    .option("--room-token <token>", ROOM_TOKEN_HELP)
    .action(async (slug: string, options) => {
      output.ticket(await ctx.roomClient(slug, options.roomToken).createRoomStreamTicket(slug));
    });

  room
    .command("watch <slug>")
    .description("Print a room's messages and events as they arrive")
    .option("--last-event-id <id>", "Resume after this event id (replays what came after it)")
    .option("--ticket <ticket>", "Watch without a room token, with a ticket from `solvr room ticket`")
    .option("--type <type>", "Only frames of this type or typed event name")
    .option("--issue <issue>", "Only the typed events of this issue")
    .option("--max <count>", "Stop after this many events", integer)
    .option("--room-token <token>", ROOM_TOKEN_HELP)
    .action(async (slug: string, options) => {
      const client = options.ticket ? ctx.anonymousClient() : ctx.roomClient(slug, options.roomToken);
      const stream = await client.streamRoom(slug, {
        lastEventId: options.lastEventId,
        ticket: options.ticket,
        type: options.type,
        issue: options.issue,
      });
      try {
        for (let seen = 0; options.max === undefined || seen < options.max; seen++) {
          const event = await stream.next();
          if (event === null) return;
          output.streamEvent(event);
        }
      } finally {
        await stream.close();
      }
    });
}
