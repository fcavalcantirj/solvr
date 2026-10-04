import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import type {
  APIOverviewPreviewMessage,
  APIOverviewPreviews,
  APIOverviewRoomPreview,
} from '@/lib/api-types';
import { SectionHeading } from './metric';

// The operator's featured rooms (SPEC Part 26, "Featured rooms"). Which rooms
// appear, the day they rotate, what each one is quoted by (the ask and the
// outcome), how much of each quote is shown and how the ages read are all the
// API's decisions. One full-width row per room: the room, what it set out to
// do, what came out of it.

const CAPTION = 'font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground';
const META = 'font-mono text-[11px] tracking-[0.06em] text-muted-foreground';

function Quote({ label, quote }: { label: string; quote: APIOverviewPreviewMessage }) {
  return (
    <figure className="min-w-0">
      <figcaption className={CAPTION}>{label}</figcaption>
      <blockquote className="mt-4 text-lg font-light leading-relaxed [overflow-wrap:anywhere] sm:text-xl">
        {quote.excerpt}
      </blockquote>
      <p className={`mt-4 ${META} [overflow-wrap:anywhere]`}>
        {quote.author} · {quote.author_role}
      </p>
      <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1">
        {quote.is_excerpt && quote.excerpt_note ? (
          <span className={META}>{quote.excerpt_note}</span>
        ) : null}
        {quote.message_url ? (
          <Link
            href={quote.message_url}
            className="font-mono text-[11px] tracking-[0.06em] underline decoration-border underline-offset-4 transition-colors hover:decoration-foreground"
          >
            Open the original message
          </Link>
        ) : null}
      </div>
    </figure>
  );
}

function FeaturedRoom({ room, data }: { room: APIOverviewRoomPreview; data: APIOverviewPreviews }) {
  return (
    <li
      data-testid="featured-room"
      className="grid gap-8 border-b border-border py-10 lg:grid-cols-12 lg:gap-12 lg:py-14"
    >
      <div className="min-w-0 lg:col-span-4">
        <Link
          href={room.url}
          className="group inline-flex items-start gap-2 text-[1.75rem] font-light leading-[1.1] tracking-[-0.03em] [overflow-wrap:anywhere] sm:text-[2.25rem]"
        >
          <span className="underline decoration-transparent decoration-1 underline-offset-[6px] transition-colors group-hover:decoration-current">
            {room.display_name}
          </span>
          <ArrowUpRight
            size={18}
            aria-hidden="true"
            className="mt-2 shrink-0 transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5"
          />
        </Link>

        {room.purpose ? (
          <p className="mt-4 max-w-[40ch] text-base leading-relaxed text-muted-foreground">
            {room.purpose}
          </p>
        ) : null}

        {room.participants.length > 0 ? (
          <ul className="mt-6 flex flex-wrap gap-x-4 gap-y-1">
            {room.participants.map((p) => (
              <li key={p.name} className="font-mono text-[11px] tracking-[0.06em] [overflow-wrap:anywhere]">
                {p.name}
              </li>
            ))}
            {room.more_participants_label ? (
              <li className={META}>{room.more_participants_label}</li>
            ) : null}
          </ul>
        ) : null}

        <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
          <span className={META}>{room.message_count_label}</span>
          <span className={META}>{room.last_activity_label}</span>
        </p>
      </div>

      {room.ask ? (
        <div className="min-w-0 lg:col-span-4">
          <Quote label={data.ask_label} quote={room.ask} />
        </div>
      ) : null}

      {room.outcome ? (
        <div className="min-w-0 border-t border-border pt-8 lg:col-span-4 lg:border-l lg:border-t-0 lg:pl-12 lg:pt-0">
          <Quote label={data.outcome_label} quote={room.outcome} />
        </div>
      ) : null}
    </li>
  );
}

export function RoomPreviewsSection({ data }: { data: APIOverviewPreviews }) {
  return (
    <section
      data-testid="overview-section-previews"
      className="px-4 sm:px-6 lg:px-12 py-12 lg:py-16 border-t border-border"
    >
      <div className="mx-auto max-w-[78rem]">
        <SectionHeading heading={data.heading} intro={data.intro} />

        <p className="mt-4 max-w-[68ch] text-[0.8125rem] leading-relaxed text-muted-foreground">
          {data.note}
        </p>

        <ol className="mt-12 border-t border-foreground">
          {data.rooms.map((room) => (
            <FeaturedRoom key={room.slug} room={room} data={data} />
          ))}
        </ol>
      </div>
    </section>
  );
}
