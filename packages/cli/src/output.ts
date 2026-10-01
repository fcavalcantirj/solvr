import chalk from "chalk";
import type { SearchResponse, Post, ApiResponse, RepliesResponse, Reply } from "./api.js";
import type { ApiError } from "./errors.js";
import type {
  Room,
  RoomEntriesResponse,
  RoomEntryResponse,
  RoomHandshake,
  RoomStreamEvent,
  RoomStreamMessage,
  RoomStreamTicket,
} from "./room-types.js";

/**
 * Output formatting for CLI
 */
export class Output {
  private jsonMode: boolean = false;

  setJsonMode(enabled: boolean): void {
    this.jsonMode = enabled;
  }

  /**
   * Output data as JSON
   */
  json(data: unknown): void {
    console.log(JSON.stringify(data, null, 2));
  }

  /**
   * Output data as a table
   */
  table(
    data: Record<string, unknown>[],
    columns: string[],
    headers?: Record<string, string>
  ): void {
    if (data.length === 0) {
      console.log(chalk.dim("No data"));
      return;
    }

    // Calculate column widths
    const widths: Record<string, number> = {};
    for (const col of columns) {
      const header = headers?.[col] || col;
      widths[col] = header.length;
      for (const row of data) {
        const value = String(row[col] ?? "");
        widths[col] = Math.max(widths[col], value.length);
      }
    }

    // Print header
    const headerLine = columns
      .map((col) => {
        const header = headers?.[col] || col.toUpperCase();
        return header.padEnd(widths[col]);
      })
      .join("  ");
    console.log(chalk.bold(headerLine));
    console.log(chalk.dim("-".repeat(headerLine.length)));

    // Print rows
    for (const row of data) {
      const line = columns
        .map((col) => String(row[col] ?? "").padEnd(widths[col]))
        .join("  ");
      console.log(line);
    }
  }

  /**
   * Output success message
   */
  success(message: string): void {
    console.log(chalk.green("✓") + " " + message);
  }

  /**
   * Output error message
   */
  error(message: string): void {
    console.error(chalk.red("✗") + " " + message);
  }

  /**
   * Output warning message
   */
  warn(message: string): void {
    console.log(chalk.yellow("⚠") + " " + message);
  }

  /**
   * Output info message
   */
  info(message: string): void {
    console.log(chalk.blue("ℹ") + " " + message);
  }

  /**
   * Format and output search results
   */
  searchResults(results: SearchResponse): void {
    if (this.jsonMode) {
      this.json(results);
      return;
    }

    if (results.data.length === 0) {
      console.log(chalk.dim("No results found"));
      return;
    }

    console.log(
      chalk.dim(
        `Found ${results.meta.total} results (${results.meta.took_ms || 0}ms)\n`
      )
    );

    for (const result of results.data) {
      const typeColor =
        result.type === "problem"
          ? chalk.red
          : result.type === "question"
            ? chalk.blue
            : chalk.green;

      console.log(
        typeColor(`[${result.type.toUpperCase()}]`) +
          " " +
          chalk.bold(result.title)
      );
      console.log(
        chalk.dim(`  ID: ${result.id}  Score: ${result.score.toFixed(2)}  Status: ${result.status}`)
      );
      if (result.snippet) {
        console.log(chalk.dim(`  ${result.snippet}`));
      }
      console.log();
    }
  }

  /**
   * Format and output a single post
   */
  post(response: ApiResponse<Post>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const post = response.data;
    const typeColor =
      post.type === "problem"
        ? chalk.red
        : post.type === "question"
          ? chalk.blue
          : chalk.green;

    console.log();
    console.log(
      typeColor(`[${post.type.toUpperCase()}]`) + " " + chalk.bold(post.title)
    );
    console.log(chalk.dim("-".repeat(60)));
    console.log();
    console.log(post.description);
    console.log();
    console.log(chalk.dim("-".repeat(60)));
    console.log(
      chalk.dim(`ID: ${post.id}  Status: ${post.status}  Votes: +${post.upvotes}/-${post.downvotes}`)
    );
    if (post.tags && post.tags.length > 0) {
      console.log(chalk.dim(`Tags: ${post.tags.join(", ")}`));
    }
    console.log(chalk.dim(`Created: ${post.created_at}`));
    console.log();
  }

  /**
   * Format and output one page of a post's replies
   */
  replies(page: RepliesResponse): void {
    if (this.jsonMode) {
      this.json(page);
      return;
    }

    if (page.data.length === 0) {
      console.log(chalk.dim("No replies yet"));
      return;
    }

    console.log(chalk.dim(`${page.meta.total} replies\n`));
    for (const reply of page.data) {
      const threaded = reply.parent_reply_id ? `  in reply to ${reply.parent_reply_id}` : "";
      console.log(
        chalk.bold(reply.id) +
          chalk.dim(`  ${reply.author_type} ${reply.author_id}  Score: ${reply.score}${threaded}`)
      );
      console.log(reply.body);
      console.log();
    }
    if (page.meta.has_more && page.meta.next_cursor) {
      console.log(chalk.dim(`More: solvr replies ${page.data[0].post_id} --cursor ${page.meta.next_cursor}`));
    }
  }

  /**
   * Format and output a created resource. JSON mode prints the API's answer.
   */
  created(type: string, response: ApiResponse<{ id: string }>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    this.success(`${type} created successfully`);
    console.log(chalk.dim(`ID: ${response.data.id}`));
  }

  /**
   * Format and output one reply and the ETag of its version
   */
  reply(response: ApiResponse<Reply>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const reply = response.data;
    console.log(chalk.bold(reply.id) + chalk.dim(`  ${reply.author_type} ${reply.author_id}  on post ${reply.post_id}`));
    console.log(reply.body);
    if (reply.etag) {
      console.log(chalk.dim(`ETag: ${reply.etag}  (edit with: solvr update-reply ${reply.id} --if-match '${reply.etag}' --body ...)`));
    }
  }

  /**
   * Output a failed call: the API's error answer in JSON mode, else its code, message and request id
   */
  apiError(err: ApiError): void {
    if (this.jsonMode) {
      console.error(JSON.stringify(err.toAnswer(), null, 2));
      return;
    }

    this.error(`${err.code}: ${err.message}`);
    if (err.requestId) {
      console.error(chalk.dim(`  request id: ${err.requestId}`));
    }
    if (err.details) {
      console.error(err.details);
    }
  }

  /**
   * Format and output a room
   */
  room(response: ApiResponse<Room>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const room = response.data;
    this.success(`Room ${room.slug} created${room.is_private ? " (private)" : ""}`);
    console.log(chalk.dim(`Join it with: solvr room join ${room.slug}`));
  }

  /**
   * Format and output the room token of a handshake
   */
  joined(response: ApiResponse<RoomHandshake>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const handshake = response.data;
    this.success(`Joined ${handshake.room_slug} as ${handshake.agent_id}${handshake.rotated ? " (other tokens rotated)" : ""}`);
    console.log(`Room token: ${handshake.room_token}`);
    console.log(chalk.dim(`Saved for: solvr room read|send|ticket|watch ${handshake.room_slug}`));
  }

  /**
   * Format and output one page of a room's timeline
   */
  roomEntries(slug: string, page: RoomEntriesResponse): void {
    if (this.jsonMode) {
      this.json(page);
      return;
    }

    if (page.data.length === 0) {
      console.log(chalk.dim("No entries yet"));
      return;
    }
    for (const entry of page.data) {
      const text = entry.kind === "event" ? chalk.dim(`[${entry.event_type ?? "event"}]`) : entry.body ?? "";
      console.log(chalk.dim(`#${entry.sequence}`) + " " + chalk.bold(entry.actor_label) + ": " + text);
    }
    if (page.meta.has_more && page.meta.next_cursor) {
      console.log(chalk.dim(`More: solvr room read ${slug} --cursor ${page.meta.next_cursor}`));
    }
  }

  /**
   * Format and output a sent room entry
   */
  roomEntry(response: RoomEntryResponse): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const entry = response.data;
    if (response.meta?.idempotent_replay) {
      this.info(`Message ${entry.id} was already sent (same client entry id)`);
    } else {
      this.success(`Message ${entry.id} sent`);
    }
  }

  /**
   * Format and output a stream ticket
   */
  ticket(response: ApiResponse<RoomStreamTicket>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const ticket = response.data;
    console.log(`Ticket: ${ticket.ticket}`);
    console.log(chalk.dim(`Opens ${ticket.stream} until ${ticket.expires_at}`));
  }

  /**
   * Output one room stream event: a JSON line in JSON mode
   */
  streamEvent(event: RoomStreamEvent): void {
    if (this.jsonMode) {
      console.log(JSON.stringify(event));
      return;
    }

    const frame = event.frame;
    if (frame.type === "message" && frame.payload) {
      const message = frame.payload as RoomStreamMessage;
      console.log(chalk.dim(`#${message.sequence_num}`) + " " + chalk.bold(message.agent_name) + ": " + message.content);
      return;
    }
    console.log(chalk.dim(`[${frame.event ?? frame.type}]${frame.agent_name ? ` ${frame.agent_name}` : ""}`));
  }
}
