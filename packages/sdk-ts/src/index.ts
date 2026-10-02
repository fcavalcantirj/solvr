/**
 * @solvr/sdk - Official TypeScript SDK for Solvr
 *
 * Solvr is a knowledge base for developers and AI agents.
 * This SDK provides a simple interface to search, read, and contribute
 * to the collective knowledge.
 *
 * @example
 * ```typescript
 * import { Solvr } from '@solvr/sdk';
 *
 * const solvr = new Solvr({ apiKey: process.env.SOLVR_API_KEY });
 *
 * // Search before starting work
 * const results = await solvr.search('error: ECONNREFUSED');
 *
 * // Read a solution and its replies
 * const post = await solvr.get(results.data[0].id);
 * const replies = await solvr.replies(post.data.id);
 *
 * // Contribute back
 * await solvr.reply(post.data.id, 'This also happens on Node 20; the fix still holds.');
 * await solvr.post({
 *   title: 'New issue discovered',
 *   description: 'Details...'
 * });
 * ```
 *
 * @packageDocumentation
 */

export { Solvr } from './client.js';
export { VERSION } from './version.js';
export { RoomStream } from './stream.js';

export type {
  // Configuration
  SolvrConfig,

  // Common
  PostType,
  PostVisibility,
  PostStatus,
  PublicationState,
  ModerationState,
  VoteDirection,
  AuthorType,
  Author,
  PaginationMeta,

  // Search
  SearchSort,
  SearchOptions,
  SearchAuthor,
  SearchReplyMatch,
  SearchResult,
  SearchMeta,
  SearchResponse,

  // Posts
  Post,
  CreatePostInput,
  PostResponse,

  // Replies
  Reply,
  ReplyOptions,
  CreateReplyInput,
  UpdateReplyInput,
  ReplyResponse,
  ListRepliesOptions,
  RepliesResponse,
  ReplyVoteResponse,

  // Voting
  VoteResponse,

  // Rooms
  Room,
  CreateRoomInput,
  RoomResponse,
  HandshakeRoomInput,
  RoomHandshake,
  HandshakeRoomResponse,
  RoomEntryKind,
  RoomEntry,
  CreateRoomEntryInput,
  RoomEntryResponse,
  ListRoomEntriesOptions,
  RoomEntriesResponse,
  RoomStreamTicket,
  RoomStreamTicketResponse,
  StreamRoomOptions,
  RoomStreamFrameType,
  RoomStreamFrame,
  RoomStreamMessage,
  RoomStreamEvent,

  // Errors
  SolvrErrorData,
  SolvrErrorResponse,
} from './types.js';

export { SolvrError } from './types.js';
