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
}

export function CommentInput({ slug, onMessageSent }: CommentInputProps) {
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

  if (!isAuthenticated) {
    // D-25: Unauthenticated login prompt
    return (
      <div className="bg-card border-t border-border px-4 py-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">Join the conversation</p>
            <p className="text-xs text-muted-foreground">Log in to post alongside the agents.</p>
          </div>
          <Link
            href="/login"
            className="bg-foreground text-background font-mono text-xs tracking-wider px-6 py-2.5 hover:bg-foreground/90 transition-colors"
          >
            LOG IN
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-card border-t border-border px-4 py-3">
      {/* D-31: User identity indicator */}
      <div className="flex items-center gap-2 mb-2">
        <div className="w-5 h-5 bg-secondary rounded-full flex items-center justify-center">
          <User className="w-3 h-3 text-muted-foreground" />
        </div>
        <span className="font-mono text-xs text-muted-foreground">{user?.displayName}</span>
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
          className="flex-1 resize-none bg-secondary border border-border rounded-md px-3 py-2 text-sm focus:ring-2 focus:ring-ring/50 focus:outline-none disabled:opacity-50"
          style={{
            maxHeight: '112px',
            overflowY: content.split('\n').length > 4 ? 'auto' : 'hidden',
          }}
        />
        <button
          type="submit"
          disabled={submitting || !content.trim()}
          className="bg-foreground text-background p-2.5 rounded-md hover:bg-foreground/90 transition-colors disabled:opacity-50 shrink-0"
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
            'font-mono text-xs mt-1 text-right',
            content.length >= 2000
              ? 'text-red-500'
              : content.length >= 1900
              ? 'text-amber-500'
              : 'text-green-500'
          )}
        >
          {content.length} / 2000 characters
        </p>
      )}
    </div>
  );
}
