/**
 * The room tools: create, join, members, add_member, read, send, ticket and watch. A join keeps
 * the room token the API issued; read, send, ticket and watch present that token (never the API
 * key), or the room_token argument when one is given. Create, join, members and add_member
 * present the API key.
 */

import { SolvrApiClient } from './api.js';
import type { RoomStream } from './stream.js';
import type {
  AddRoomMemberInput,
  CreateRoomEntryInput,
  CreateRoomInput,
  HandshakeRoomInput,
  ListRoomEntriesOptions,
  RoomEntry,
  RoomMember,
  RoomRole,
  RoomStreamEvent,
  RoomStreamMessage,
} from './room-types.js';
import {
  ToolDefinition,
  ToolInputError,
  ToolResult,
  failureText,
  optionalList,
  optionalNumber,
  optionalString,
  requireString,
  textResult,
} from './tool-kit.js';

/** How long solvr_room_watch waits by default, and at most, before it answers what it has. */
const WATCH_WAIT_SECONDS = 30;
const WATCH_WAIT_MAX_SECONDS = 120;

const SLUG = { type: 'string', description: 'The room slug' };
const ROOM_TOKEN = {
  type: 'string',
  description: 'Optional: the room token to present (default: the one solvr_room_join kept for this room)',
};

export const ROOM_TOOL_DEFINITIONS: ToolDefinition[] = [
  {
    name: 'solvr_room_create',
    description: 'Create a room where independently running agents work together (a planner, executors, a reviewer). Each agent then joins it with solvr_room_join.',
    inputSchema: {
      type: 'object',
      properties: {
        display_name: { type: 'string', description: 'The room name' },
        slug: { type: 'string', description: 'Optional: the room slug (default: derived from display_name)' },
        description: { type: 'string', description: 'Optional: what the room is for' },
        tags: { type: 'array', description: 'Optional: tags', items: { type: 'string' } },
        is_private: { type: 'boolean', description: 'Optional: true for a room only its members can read' },
      },
      required: ['display_name'],
    },
  },
  {
    name: 'solvr_room_join',
    description: 'Join a room. The API issues this agent a room token; this server keeps it for solvr_room_read, solvr_room_send, solvr_room_ticket and solvr_room_watch on that room.',
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
        rotate: { type: 'boolean', description: "Optional: true replaces this agent's other live tokens for the room" },
        ttl_seconds: { type: 'number', description: 'Optional: the token lifetime in seconds (default: no expiry)' },
      },
      required: ['slug'],
    },
  },
  {
    name: 'solvr_room_members',
    description: "List a room's participants and their roles, owners first (owner only; presents the API key). Their agent ids are what addressed_member_ids names.",
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
      },
      required: ['slug'],
    },
  },
  {
    name: 'solvr_room_add_member',
    description: 'Admit a third or any later agent to the same room (owner only; presents the API key). The admitted agent then joins it with solvr_room_join and its own API key; no new room is needed.',
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
        agent_id: { type: 'string', description: 'The agent to admit' },
        role: { type: 'string', description: 'Optional: owner or member (default: a new participant is a member, an existing one keeps its role)', enum: ['owner', 'member'] },
      },
      required: ['slug', 'agent_id'],
    },
  },
  {
    name: 'solvr_room_read',
    description: "Read a room's timeline (messages and typed events), oldest first, one page at a time.",
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
        limit: { type: 'number', description: 'Optional: page size' },
        cursor: { type: 'string', description: 'Optional: the cursor of the next page, from a previous read' },
        kind: { type: 'string', description: 'Optional: only messages or only events', enum: ['message', 'event'] },
        issue: { type: 'string', description: 'Optional: only the typed events of this issue' },
        room_token: ROOM_TOKEN,
      },
      required: ['slug'],
    },
  },
  {
    name: 'solvr_room_send',
    description: 'Send a message to a room. Set reply_to_entry_id to answer an entry and addressed_member_ids to address members; a repeated client_entry_id is sent once.',
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
        body: { type: 'string', description: 'The message (Markdown)' },
        client_entry_id: { type: 'string', description: 'Optional: your id for this message; resending it does not send it twice' },
        reply_to_entry_id: { type: 'number', description: 'Optional: the id of the entry this message answers' },
        addressed_member_ids: { type: 'array', description: 'Optional: the agent ids this message is addressed to', items: { type: 'string' } },
        room_token: ROOM_TOKEN,
      },
      required: ['slug', 'body'],
    },
  },
  {
    name: 'solvr_room_ticket',
    description: "Mint a short-lived ticket that opens the room's stream without a credential (for a watcher that holds no room token).",
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
        room_token: ROOM_TOKEN,
      },
      required: ['slug'],
    },
  },
  {
    name: 'solvr_room_watch',
    description: `Wait for a room's next events (live stream). Answers after max_events events (default 1) or wait_seconds (default ${WATCH_WAIT_SECONDS}), with the last event id to continue from.`,
    inputSchema: {
      type: 'object',
      properties: {
        slug: SLUG,
        last_event_id: { type: 'string', description: 'Optional: the last event id received; the stream replays what came after it' },
        ticket: { type: 'string', description: 'Optional: a stream ticket (solvr_room_ticket): watches without a credential' },
        event_type: { type: 'string', description: "Optional: only frames of this type or typed event name (e.g. message); the stream's type filter" },
        issue: { type: 'string', description: 'Optional: only the typed events of this issue' },
        max_events: { type: 'number', description: 'Optional: answer after this many events (default 1)', default: 1 },
        wait_seconds: { type: 'number', description: `Optional: answer after this many seconds (default ${WATCH_WAIT_SECONDS}, at most ${WATCH_WAIT_MAX_SECONDS})`, default: WATCH_WAIT_SECONDS },
        room_token: ROOM_TOKEN,
      },
      required: ['slug'],
    },
  },
];

const ROOM_TOOL_NAMES = new Set(ROOM_TOOL_DEFINITIONS.map((tool) => tool.name));

export class RoomTools {
  /** The room token each solvr_room_join issued, per room slug, for the life of this server process. */
  private readonly tokens = new Map<string, string>();

  /** client presents the API key (create, join); room clients are made per room token. */
  constructor(private readonly client: SolvrApiClient, private readonly apiUrl: string) {}

  handles(name: string): boolean {
    return ROOM_TOOL_NAMES.has(name);
  }

  async execute(name: string, args: Record<string, unknown>): Promise<ToolResult> {
    switch (name) {
      case 'solvr_room_create':
        return this.create(args);
      case 'solvr_room_join':
        return this.join(args);
      case 'solvr_room_members':
        return this.members(args);
      case 'solvr_room_add_member':
        return this.addMember(args);
      case 'solvr_room_read':
        return this.read(args);
      case 'solvr_room_send':
        return this.send(args);
      case 'solvr_room_ticket':
        return this.ticket(args);
      default:
        return this.watch(args);
    }
  }

  /** A client presenting the room_token argument, else the token a join kept for the room. */
  private roomClient(slug: string, args: Record<string, unknown>): SolvrApiClient {
    const token = optionalString(args, 'room_token') ?? this.tokens.get(slug);
    if (!token) {
      throw new ToolInputError(`No room token for ${slug}. Call solvr_room_join with slug "${slug}" first, or pass room_token.`);
    }
    return new SolvrApiClient(token, this.apiUrl);
  }

  private async create(args: Record<string, unknown>): Promise<ToolResult> {
    const input: CreateRoomInput = { display_name: requireString(args, 'display_name') };
    const slug = optionalString(args, 'slug');
    if (slug !== undefined) input.slug = slug;
    const description = optionalString(args, 'description');
    if (description !== undefined) input.description = description;
    const tags = optionalList(args, 'tags');
    if (tags !== undefined) input.tags = tags;
    if (args.is_private !== undefined) input.is_private = args.is_private === true;

    const room = (await this.client.createRoom(input)).data;
    return textResult([
      `Room ${room.slug} created${room.is_private ? ' (private)' : ''}: ${room.display_name}`,
      `ID: ${room.id}`,
      `Join it with solvr_room_join (slug ${room.slug}); every agent that joins the same slug works in this room.`,
    ]);
  }

  private async join(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const input: HandshakeRoomInput = {};
    if (args.rotate !== undefined) input.rotate = args.rotate === true;
    const ttl = optionalNumber(args, 'ttl_seconds');
    if (ttl !== undefined) input.ttl_seconds = ttl;

    const handshake = (await this.client.handshakeRoom(slug, input)).data;
    this.tokens.set(slug, handshake.room_token);
    return textResult([
      `Joined ${handshake.room_slug} as ${handshake.agent_id}${handshake.rotated ? ' (your other tokens for this room were rotated)' : ''}`,
      `Room token: ${handshake.room_token}`,
      `Kept for solvr_room_read, solvr_room_send, solvr_room_ticket and solvr_room_watch on ${slug}; after a restart of this server pass it as room_token.`,
    ]);
  }

  private async members(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const participants = (await this.client.listRoomMembers(slug)).data;
    return textResult([`${participants.length} participants of ${slug}:`, ...participants.map(memberLine)]);
  }

  private async addMember(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const input: AddRoomMemberInput = { agent_id: requireString(args, 'agent_id') };
    const role = optionalString(args, 'role');
    if (role !== undefined) input.role = role as RoomRole;

    const member = (await this.client.addRoomMember(slug, input)).data;
    return textResult([
      `${member.agent_id} is in ${slug} as ${member.role} (added by ${member.added_by})`,
      `It joins with its own API key: solvr_room_join with slug ${slug}.`,
    ]);
  }

  private async read(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const client = this.roomClient(slug, args);
    const options: ListRoomEntriesOptions = {
      cursor: optionalString(args, 'cursor'),
      limit: optionalNumber(args, 'limit'),
      kind: optionalString(args, 'kind'),
      issue: optionalString(args, 'issue'),
    };

    const page = await client.listRoomEntries(slug, options);
    if (page.data.length === 0) {
      return textResult(['No entries yet.']);
    }
    const lines = page.data.map(entryLine);
    if (page.meta.has_more && page.meta.next_cursor) {
      lines.push('', `More: call solvr_room_read with slug ${slug} and cursor ${page.meta.next_cursor}`);
    }
    return textResult(lines);
  }

  private async send(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const input: CreateRoomEntryInput = { body: requireString(args, 'body') };
    const client = this.roomClient(slug, args);
    const clientEntryId = optionalString(args, 'client_entry_id');
    if (clientEntryId !== undefined) input.client_entry_id = clientEntryId;
    const replyTo = optionalNumber(args, 'reply_to_entry_id');
    if (replyTo !== undefined) input.reply_to_entry_id = replyTo;
    const addressed = optionalList(args, 'addressed_member_ids');
    if (addressed !== undefined) input.addressed_member_ids = addressed;

    const answer = await client.createRoomEntry(slug, input);
    const entry = answer.data;
    return textResult([
      answer.meta?.idempotent_replay
        ? `Message ${entry.id} was already sent (same client_entry_id); not sent again.`
        : `Message ${entry.id} sent to ${slug} (#${entry.sequence}).`,
    ]);
  }

  private async ticket(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const ticket = (await this.roomClient(slug, args).createRoomStreamTicket(slug)).data;
    return textResult([
      `Ticket: ${ticket.ticket}`,
      `Opens ${ticket.stream} until ${ticket.expires_at} (${ticket.ttl_seconds} s) without a credential: solvr_room_watch with slug ${slug} and this ticket.`,
    ]);
  }

  /**
   * Reads the stream until max_events events arrived, the stream ended, or wait_seconds passed;
   * then closes it. A watch with a ticket is anonymous.
   */
  private async watch(args: Record<string, unknown>): Promise<ToolResult> {
    const slug = requireString(args, 'slug');
    const ticket = optionalString(args, 'ticket');
    const client = ticket ? new SolvrApiClient(null, this.apiUrl) : this.roomClient(slug, args);
    const lastEventId = optionalString(args, 'last_event_id');
    const maxEvents = Math.max(1, Math.floor(optionalNumber(args, 'max_events') ?? 1));
    const waitSeconds = Math.min(WATCH_WAIT_MAX_SECONDS, Math.max(1, optionalNumber(args, 'wait_seconds') ?? WATCH_WAIT_SECONDS));

    const events: RoomStreamEvent[] = [];
    const timeout = new AbortController();
    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      timeout.abort();
    }, waitSeconds * 1000);
    let failure: unknown;
    let stream: RoomStream | undefined;
    try {
      stream = await client.streamRoom(slug, { lastEventId, ticket, type: optionalString(args, 'event_type'), issue: optionalString(args, 'issue') }, timeout.signal);
      while (events.length < maxEvents) {
        const event = await stream.next();
        if (!event) break;
        events.push(event);
      }
    } catch (error) {
      if (!timedOut) {
        if (events.length === 0) throw error;
        failure = error;
      }
    } finally {
      clearTimeout(timer);
      await stream?.close();
      timeout.abort();
    }

    const lines = events.map(eventLine);
    if (events.length === 0) {
      lines.push(timedOut ? `No events within ${waitSeconds} s.` : 'No events: the stream ended.');
    }
    const lastId = [...events].reverse().find((event) => event.id)?.id ?? lastEventId;
    if (failure !== undefined) {
      lines.push('', failureText('solvr_room_watch', failure));
    } else if (lastId) {
      lines.push('', `To continue: solvr_room_watch with slug ${slug} and last_event_id ${lastId}`);
    }
    return textResult(lines, failure !== undefined);
  }
}

/** One participant: its agent id, role, who admitted it and since when. */
function memberLine(member: RoomMember): string {
  return `${member.agent_id} ${member.role} (added by ${member.added_by}, since ${member.created_at})`;
}

/** One timeline entry: its sequence and id, who wrote it, and its body or event. */
function entryLine(entry: RoomEntry): string {
  const head = `#${entry.sequence} (id ${entry.id}) ${entry.actor_label}`;
  const notes: string[] = [];
  if (entry.reply_to_entry_id) notes.push(`reply to ${entry.reply_to_entry_id}`);
  if (entry.addressed_member_ids?.length) notes.push(`to ${entry.addressed_member_ids.join(', ')}`);
  const note = notes.length > 0 ? ` (${notes.join('; ')})` : '';
  if (entry.kind === 'event') {
    return `${head} [${entry.event_type ?? 'event'}]${entry.issue ? ` issue ${entry.issue}` : ''}${note}`;
  }
  return `${head}${note}: ${entry.body ?? ''}`;
}

/** One stream event: a message with its author and content, else the frame's event name. */
function eventLine(event: RoomStreamEvent): string {
  const frame = event.frame;
  const id = event.id ? ` (id ${event.id})` : '';
  if (frame.type === 'message' && frame.payload) {
    const message = frame.payload as RoomStreamMessage;
    return `#${message.sequence_num}${id} ${message.agent_name}: ${message.content}`;
  }
  return `[${frame.event ?? frame.type}]${id}${frame.agent_name ? ` ${frame.agent_name}` : ''}`;
}
