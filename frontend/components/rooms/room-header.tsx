import Link from "next/link";
import { formatDistanceToNow } from "date-fns";
import { Badge } from "@/components/ui/badge";
import { RoomHeaderActions } from "./room-header-actions";
import type { APIRoom } from "@/lib/api-types";

interface RoomHeaderProps {
  room: APIRoom;
  ownerDisplayName?: string;
  // Live participant count from server-confirmed presence (never recomputed here).
  onlineCount?: number;
}

export function RoomHeader({ room, ownerDisplayName, onlineCount }: RoomHeaderProps) {
  return (
    <div className="mb-4 lg:mb-8">
      {/* Eyebrow + live/finished status + visibility. An archived room is labeled
          Finished rather than Live; the transcript stays readable at this URL. */}
      <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-2 lg:mb-4 flex items-center gap-2">
        <span>ROOM</span>
        {room.archived_at ? (
          <span data-testid="room-status" className="text-muted-foreground">· FINISHED</span>
        ) : (
          <span data-testid="room-status" className="text-green-700 dark:text-green-400">· LIVE</span>
        )}
        <span data-testid="room-visibility" className="text-muted-foreground">
          · {room.is_private ? "PRIVATE" : "PUBLIC"}
        </span>
      </p>
      {/* Room name + header actions (Share, Connect an agent) */}
      <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 mb-2 lg:mb-4">
        <h1 className="text-2xl sm:text-3xl md:text-4xl lg:text-5xl font-normal tracking-tight break-words">
          {room.display_name}
        </h1>
        <div className="shrink-0">
          <RoomHeaderActions slug={room.slug} displayName={room.display_name} />
        </div>
      </div>
      {/* Description */}
      {room.description && (
        <p className="text-sm lg:text-base text-muted-foreground leading-relaxed mb-2 lg:mb-4 max-w-3xl">
          {room.description}
        </p>
      )}
      {/* Meta row */}
      <div className="flex flex-wrap items-center gap-3">
        {room.category && (
          <Badge
            variant="secondary"
            className="font-mono text-[10px] tracking-wider"
          >
            {room.category}
          </Badge>
        )}
        {room.tags?.map((tag) => (
          <Badge
            key={tag}
            variant="outline"
            className="font-mono text-[10px] tracking-wider"
          >
            {tag}
          </Badge>
        ))}
        {ownerDisplayName && room.owner_id && (
          <Link
            href={`/users/${room.owner_id}`}
            className="text-xs text-muted-foreground hover:underline"
          >
            by {ownerDisplayName}
          </Link>
        )}
        {onlineCount != null && (
          <span
            data-testid="room-participants"
            className="font-mono text-xs text-muted-foreground flex items-center gap-1.5"
          >
            <span
              className={
                onlineCount > 0
                  ? "inline-block w-1.5 h-1.5 rounded-full bg-green-600 dark:bg-green-400"
                  : "inline-block w-1.5 h-1.5 rounded-full bg-muted-foreground/40"
              }
              aria-hidden="true"
            />
            {onlineCount} online
          </span>
        )}
        <span className="font-mono text-xs text-muted-foreground">
          {room.message_count} messages
        </span>
        <span className="font-mono text-xs text-muted-foreground">
          Created{" "}
          {formatDistanceToNow(new Date(room.created_at), { addSuffix: true })}
        </span>
      </div>
    </div>
  );
}
