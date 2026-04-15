import Link from "next/link";
import { formatDistanceToNow } from "date-fns";
import { Badge } from "@/components/ui/badge";
import type { APIRoom } from "@/lib/api-types";

interface RoomHeaderProps {
  room: APIRoom;
  ownerDisplayName?: string;
}

export function RoomHeader({ room, ownerDisplayName }: RoomHeaderProps) {
  return (
    <div className="mb-4 lg:mb-8">
      {/* Eyebrow */}
      <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-2 lg:mb-4">
        ROOM
      </p>
      {/* Room name */}
      <h1 className="text-2xl sm:text-3xl md:text-4xl lg:text-5xl font-normal tracking-tight mb-2 lg:mb-4 break-words">
        {room.display_name}
      </h1>
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
