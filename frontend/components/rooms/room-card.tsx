import Link from 'next/link';
import { formatDistanceToNow } from 'date-fns';
import { MessageSquare, Users } from 'lucide-react';
import { Card, CardHeader, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import type { APIRoomWithStats } from '@/lib/api-types';

interface RoomCardProps {
  room: APIRoomWithStats;
}

export function RoomCard({ room }: RoomCardProps) {
  const lastActive = formatDistanceToNow(new Date(room.last_active_at), { addSuffix: true });

  return (
    <Link href={`/rooms/${room.slug}`} className="block">
      <Card className="bg-card border border-border hover:border-foreground hover:shadow-sm transition-all h-full rounded-none">
        <CardHeader className="pb-2">
          <div className="flex items-start justify-between gap-2">
            <h3 className="font-mono text-base tracking-tight leading-snug">
              {room.display_name}
            </h3>
            {room.category && (
              <Badge
                variant="secondary"
                className="font-mono text-[10px] tracking-wider shrink-0"
              >
                {room.category}
              </Badge>
            )}
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {room.description && (
            <p className="text-sm text-muted-foreground leading-relaxed line-clamp-2">
              {room.description}
            </p>
          )}

          {/* Stats row */}
          <div className="flex flex-wrap items-center gap-3">
            {/* Live agent count */}
            <div className="flex items-center gap-1.5">
              {room.live_agent_count > 0 && (
                <span className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
              )}
              <span className="font-mono text-xs text-muted-foreground">
                {room.live_agent_count} live
              </span>
            </div>

            {/* Unique participant count */}
            <div className="flex items-center gap-1">
              <Users size={12} className="text-muted-foreground" />
              <span className="font-mono text-xs text-muted-foreground">
                {room.unique_participant_count} participants
              </span>
            </div>

            {/* Message count */}
            <div className="flex items-center gap-1">
              <MessageSquare size={12} className="text-muted-foreground" />
              <span className="font-mono text-xs text-muted-foreground">
                {room.message_count} messages
              </span>
            </div>

            {/* Last active */}
            <span className="font-mono text-xs text-muted-foreground">
              {lastActive}
            </span>
          </div>

          {/* Owner */}
          {room.owner_display_name && room.owner_id && (
            <div className="flex items-center gap-1">
              <span className="text-xs text-muted-foreground">by</span>
              <Link
                href={`/users/${room.owner_id}`}
                className="text-xs text-muted-foreground hover:underline"
                onClick={(e) => e.stopPropagation()}
              >
                {room.owner_display_name}
              </Link>
            </div>
          )}
        </CardContent>
      </Card>
    </Link>
  );
}
