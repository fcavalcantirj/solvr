"use client";

import { useState, useRef, useCallback } from 'react';
import Link from 'next/link';
import { User, Send, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';
import type { APIRoomMessage } from '@/lib/api-types';

interface CommentInputProps {
  slug: string;
  onMessageSent: (msg: APIRoomMessage) => void;
  /** When true the room is Finished: the composer is replaced by a read-only
   * notice offering to start a fresh collaboration (the API refuses new posts). */
  archived?: boolean;
}

export function CommentInput({ slug, onMessageSent, archived = false }: CommentInputProps) {
  const { user, isAuthenticated } = useAuth();
  const [content, setContent] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setContent(e.target.value);
    // Auto-expand textarea height (D-30): max 4 lines (~112px), then internal scroll
    const textarea = e.target;
    textarea.style.height = 'auto';
    textarea.style.height = Math.min(textarea.scrollHeight, 112) + 'px';
  }, []);

  const handleSubmit = useCallback(async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const trimmed = content.trim();
    if (!trimmed || submitting) return;

    setSubmitting(true);
    try {
      // D-28: Wait for server confirmation before showing message (no optimistic UI)
      const response = await api.postRoomMessage(slug, trimmed);
      onMessageSent(response.data);
      setContent('');
      // Reset textarea height
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto';
      }
    } catch (err: unknown) {
      const status = (err as { status?: number })?.status;
      if (status === 429) {
        // D-32: Rate limit toast
        toast.error('Slow down — try again in a few seconds');
      } else {
        toast.error('Failed to post — please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }, [content, submitting, slug, onMessageSent]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // D-27: Enter sends; Shift+Enter inserts newline
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      void handleSubmit();
    }
  }, [handleSubmit]);

  if (archived) {
    // The room is Finished: no new messages are accepted. Offer a clean restart
    // that reuses the collaboration's instructions in a brand-new room.
    return (
      <div className="bg-background border-t border-border py-6">
        <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-4">
          <div className="min-w-0">
            <p className="text-2xl font-light tracking-[-0.025em]">This room is finished</p>
            <p className="mt-2 text-sm text-muted-foreground">
              New messages are closed. Start a new room to continue the work.
            </p>
          </div>
          <Link
            href="/connect"
            className="border border-foreground bg-foreground text-background font-mono text-[11px] uppercase tracking-[0.18em] px-6 py-3 hover:bg-background hover:text-foreground transition-colors whitespace-nowrap"
          >
            START A NEW ROOM
          </Link>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    // Task 39, step 2: a QUIET "Log in to comment" action at the composer — a
    // secondary text link, not a solid CTA — so the login prompt never competes
    // with or blocks the open transcript above it. Step 3: it carries the room
    // path as ?next= so login returns the reader to this same room.
    const next = encodeURIComponent(`/rooms/${slug}`);
    return (
      <div className="bg-background border-t border-border py-5">
        <p className="text-sm text-muted-foreground">
          <Link
            href={`/login?next=${next}`}
            className="font-mono text-[11px] uppercase tracking-[0.18em] underline underline-offset-4 hover:text-foreground transition-colors"
          >
            Log in to comment
          </Link>{' '}
          — reading stays open to everyone.
        </p>
      </div>
    );
  }

  return (
    <div className="bg-background border-t border-border py-5">
      {/* D-31: User identity indicator */}
      <div className="flex items-center gap-2 mb-3">
        <div className="w-5 h-5 bg-secondary rounded-full flex items-center justify-center">
          <User aria-hidden="true" className="w-3 h-3 text-muted-foreground" />
        </div>
        <span className="text-sm text-muted-foreground">{user?.displayName}</span>
      </div>
      <form onSubmit={handleSubmit} className="flex items-end gap-2">
        <textarea
          ref={textareaRef}
          value={content}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder="Type a message..."
          rows={1}
          disabled={submitting}
          className="min-w-0 flex-1 resize-none bg-background border border-border rounded-none px-4 py-3 text-[0.9375rem] leading-relaxed placeholder:text-muted-foreground focus:border-foreground focus:outline-none focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground disabled:opacity-50"
          style={{
            maxHeight: '112px',
            overflowY: content.split('\n').length > 4 ? 'auto' : 'hidden',
          }}
        />
        <button
          type="submit"
          disabled={submitting || !content.trim()}
          className="border border-foreground flex items-center justify-center bg-foreground text-background p-2.5 rounded-none hover:bg-background hover:text-foreground transition-colors disabled:opacity-50 shrink-0 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-foreground"
          style={{ minHeight: '44px', minWidth: '44px' }}
        >
          {submitting ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Send className="w-4 h-4" />
          )}
        </button>
      </form>
      {/* D-29: Character limit indicator — only shown near limit */}
      {content.length >= 1800 && (
        <p
          className={cn(
            'font-mono text-[11px] mt-1 text-right',
            content.length >= 2000
              ? 'text-red-700 dark:text-red-400'
              : content.length >= 1900
              ? 'text-amber-700 dark:text-amber-400'
              : 'text-green-700 dark:text-green-400'
          )}
        >
          {content.length} / 2000 characters
        </p>
      )}
    </div>
  );
}
