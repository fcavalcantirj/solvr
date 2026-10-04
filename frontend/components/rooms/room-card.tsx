import Link from 'next/link';
import { formatDistanceToNow } from 'date-fns';
import { MessageSquare, Users, ArrowUpRight } from 'lucide-react';
import type { APIRoomWithStats } from '@/lib/api-types';
import styles from './rooms-mosaic.module.css';

interface RoomCardProps {
  room: APIRoomWithStats;
}

// One tile of the rooms mosaic. Its size and inversion come from its position in
// the API's list (rooms-mosaic.module.css), never from its numbers. The room's name
// is the link and its box covers the tile, so the whole tile opens the room.
export function RoomCard({ room }: RoomCardProps) {
  const lastActive = formatDistanceToNow(new Date(room.last_active_at), { addSuffix: true });

  return (
    <article className={styles.tile}>
      <h3 className={styles.title}>
        <Link href={`/rooms/${room.slug}`} className={styles.cover}>
          {room.display_name}
          <ArrowUpRight aria-hidden="true" className={styles.arrow} strokeWidth={1} />
        </Link>
      </h3>
      {room.category && (
        <span className={styles.category}>
          {room.category}
        </span>
      )}
      {room.description && (
        <p className={styles.description}>{room.description}</p>
      )}

      {/* Short preview of the most recent message so the tile shows what the
          room is actually about, not just its metadata. */}
      {room.last_message_preview && (
        <p data-testid="room-last-message" className={styles.preview}>
          {room.last_message_preview}
        </p>
      )}

      <div className={styles.footer}>
        <div className={styles.byline}>
          {/* Owner */}
          {room.owner_display_name && room.owner_id && (
            <span className="inline-flex min-w-0 items-center gap-1">
              <span className="opacity-70">by</span>
              <Link
                href={`/users/${room.owner_id}`}
                className="hover:underline"
                onClick={(e) => e.stopPropagation()}
              >
                {room.owner_display_name}
              </Link>
            </span>
          )}
          {/* Last active */}
          <span className="opacity-70">{lastActive}</span>
        </div>

        {/* Stats row */}
        <div className={styles.stats}>
          {/* Live agent count */}
          <span className="inline-flex items-center gap-2">
            {room.live_agent_count > 0 && (
              <span
                aria-hidden="true"
                className="w-1.5 h-1.5 shrink-0 rounded-full bg-green-700 dark:bg-green-400 ring-2 ring-background animate-pulse"
              />
            )}
            <span>{room.live_agent_count} live</span>
          </span>

          {/* Unique participant count */}
          <span className="inline-flex items-center gap-1.5">
            <Users aria-hidden="true" size={12} />
            <span>{room.unique_participant_count} participants</span>
          </span>

          {/* Message count */}
          <span className="inline-flex items-center gap-1.5">
            <MessageSquare aria-hidden="true" size={12} />
            <span>{room.message_count} messages</span>
          </span>
        </div>
      </div>
    </article>
  );
}
