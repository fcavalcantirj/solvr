import Link from "next/link";
import { formatDistanceToNow } from "date-fns";
import { Caption, CAPTION } from "@/components/page/caption";
import { RoomHeaderActions } from "./room-header-actions";
import type { APIRoom } from "@/lib/api-types";

interface RoomHeaderProps {
  room: APIRoom;
  ownerDisplayName?: string;
  // Live participant count from server-confirmed presence (never recomputed here).
  onlineCount?: number;
  // The API's "Try this workflow" link (null for a private room).
  tryWorkflowUrl?: string | null;
  // The conversation's live state (connection progress, and the transport when it is
  // not live), set in the facts strip so the room says its state once.
  liveStatus?: React.ReactNode;
}

export function RoomHeader({ room, ownerDisplayName, onlineCount, tryWorkflowUrl, liveStatus }: RoomHeaderProps) {
  return (
    <div>
      {/* The room's name opens the page, set big and quiet; its actions (Share,
          Connect an agent) stand beside it. */}
      <div className="grid gap-6 pb-8 pt-8 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end lg:gap-16 lg:pb-10 lg:pt-12">
        <div className="min-w-0">
          <h1 className="max-w-[24ch] text-[clamp(2.25rem,4.5vw,4.25rem)] font-light leading-[1.02] tracking-[-0.045em] [overflow-wrap:anywhere]">
            {room.display_name}
          </h1>
          {/* Description */}
          {room.description && (
            <p className="mt-5 max-w-[60ch] text-lg font-light leading-snug tracking-[-0.015em] text-muted-foreground">
              {room.description}
            </p>
          )}
        </div>
        <div className="min-w-0">
          <RoomHeaderActions slug={room.slug} displayName={room.display_name} isPrivate={room.is_private} tryWorkflowUrl={tryWorkflowUrl} />
        </div>
      </div>

      {/* The room's facts in one hairline strip: live/finished status (an archived
          room is labeled Finished rather than Live; the transcript stays readable
          at this URL), visibility, category, tags, owner and counts. */}
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-border py-4">
        <Caption as="span" className="inline-flex flex-wrap items-center gap-2">
          {room.archived_at ? (
            <span data-testid="room-status" className="text-muted-foreground">· FINISHED</span>
          ) : (
            <span data-testid="room-status" className="text-green-700 dark:text-green-400">· LIVE</span>
          )}
          <span data-testid="room-visibility" className="text-muted-foreground">
            · {room.is_private ? "PRIVATE" : "PUBLIC"}
          </span>
        </Caption>
        {room.category && (
          <Caption as="span" className="text-foreground">
            {room.category}
          </Caption>
        )}
        {room.tags?.map((tag) => (
          <Caption as="span" key={tag}>
            {tag}
          </Caption>
        ))}
        {ownerDisplayName && room.owner_id && (
          <Link
            href={`/users/${room.owner_id}`}
            className="text-xs text-muted-foreground hover:text-foreground hover:underline underline-offset-4"
          >
            by {ownerDisplayName}
          </Link>
        )}
        {onlineCount != null && (
          <span
            data-testid="room-participants"
            className={`${CAPTION} flex items-center gap-2`}
          >
            <span
              className={
                onlineCount > 0
                  ? "inline-block w-1.5 h-1.5 rounded-full bg-green-700 dark:bg-green-400"
                  : "inline-block w-1.5 h-1.5 rounded-full bg-muted-foreground/40"
              }
              aria-hidden="true"
            />
            {onlineCount} online
          </span>
        )}
        <span className={CAPTION}>
          {room.message_count} messages
        </span>
        <span className={CAPTION}>
          Created{" "}
          {formatDistanceToNow(new Date(room.created_at), { addSuffix: true })}
        </span>
        {liveStatus}
      </div>
    </div>
  );
}
