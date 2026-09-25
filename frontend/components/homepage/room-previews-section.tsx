import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import type { APIOverviewPreviews, APIOverviewRoomPreview } from '@/lib/api-types';
import { SectionHeading } from './metric';

// Editorially selected room previews. Which rooms appear, why each one is
// here, how much of the exchange is shown and how the ages read are all the
// API's decisions — including the rule that a busy room is not promoted.

function Preview({ room }: { room: APIOverviewRoomPreview }) {
  return (
    <article className="border border-border bg-background p-5 sm:p-6 flex flex-col">
      <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-3">
        {room.selected_reason}
      </p>

      <Link
        href={room.url}
        className="font-mono text-sm tracking-wider underline underline-offset-4 hover:no-underline inline-flex items-start gap-1"
      >
        {room.display_name}
        <ArrowUpRight size={12} className="mt-0.5 shrink-0" />
      </Link>

      {room.purpose ? (
        <p className="text-sm text-muted-foreground leading-relaxed mt-3">
          {room.purpose}
        </p>
      ) : null}

      {room.participants.length > 0 ? (
        <ul className="mt-4 border-y border-border divide-y divide-border">
          {room.participants.map((p) => (
            <li
              key={p.name}
              className="py-2 flex flex-wrap items-baseline justify-between gap-x-3"
            >
              <span className="font-mono text-[11px] tracking-wider break-all">
                {p.name}
              </span>
              <span className="font-mono text-[10px] tracking-wider text-muted-foreground shrink-0">
                {p.message_label}
              </span>
            </li>
          ))}
          {room.more_participants_label ? (
            <li className="py-2 font-mono text-[10px] tracking-wider text-muted-foreground">
              {room.more_participants_label}
            </li>
          ) : null}
        </ul>
      ) : null}

      {room.exchange.length > 0 ? (
        <ol className="mt-4 space-y-4">
          {room.exchange.map((message) => (
            <li key={message.message_url ?? message.excerpt}>
              <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-1">
                {message.author} · {message.author_role}
              </p>
              <p className="text-sm leading-relaxed whitespace-pre-wrap">
                {message.excerpt}
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
                {message.is_excerpt && message.excerpt_note ? (
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {message.excerpt_note}
                  </span>
                ) : null}
                {message.message_url ? (
                  <Link
                    href={message.message_url}
                    className="font-mono text-[10px] tracking-wider underline underline-offset-4 hover:no-underline"
                  >
                    Open the original message
                  </Link>
                ) : null}
              </div>
            </li>
          ))}
        </ol>
      ) : null}

      <div className="mt-auto pt-5 flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
          {room.message_count_label}
        </span>
        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
          {room.last_activity_label}
        </span>
      </div>
    </article>
  );
}

export function RoomPreviewsSection({ data }: { data: APIOverviewPreviews }) {
  return (
    <section
      data-testid="overview-section-previews"
      className="px-4 sm:px-6 lg:px-12 py-24 lg:py-32 border-t border-border"
    >
      <div className="max-w-7xl mx-auto">
        <SectionHeading eyebrow="SELECTED" heading={data.heading} intro={data.intro} />

        <p className="mt-4 font-mono text-[10px] tracking-wider text-muted-foreground max-w-3xl">
          {data.note}
        </p>

        {data.rooms.length === 0 ? (
          <p className="mt-12 text-sm text-muted-foreground">{data.empty_note}</p>
        ) : (
          <div className="mt-12 grid md:grid-cols-2 lg:grid-cols-3 gap-6 lg:gap-8 items-stretch">
            {data.rooms.map((room) => (
              <Preview key={room.slug} room={room} />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
