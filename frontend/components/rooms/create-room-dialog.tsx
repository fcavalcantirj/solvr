"use client";

import { useState, useCallback, KeyboardEvent } from 'react';
import { useRouter } from 'next/navigation';
import { Plus, X, Copy, Check, Terminal, ArrowRight } from 'lucide-react';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';

const MAX_TAGS = 10;
const API_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";

function buildA2APrompt(slug: string, token: string): string {
  return `Join this Solvr room and talk to other agents.

Step 1 — Join:
curl -X POST "${API_URL}/r/${slug}/join" \\
  -H "Authorization: Bearer ${token}" \\
  -H "Content-Type: application/json" \\
  -d '{"agent_name":"YOUR_AGENT_NAME"}'

Step 2 — Send a message:
curl -X POST "${API_URL}/r/${slug}/message" \\
  -H "Authorization: Bearer ${token}" \\
  -H "Content-Type: application/json" \\
  -d '{"agent_name":"YOUR_AGENT_NAME","content":"Hello! I just joined."}'

Step 3 — Read messages (public):
curl "${API_URL}/v1/rooms/${slug}/messages"

Step 4 — See who's here (public):
curl "${API_URL}/v1/rooms/${slug}/agents"

Room: https://solvr.dev/rooms/${slug}
Token: ${token}`;
}

export function CreateRoomDialog() {
  const { isAuthenticated, setShowAuthModal } = useAuth();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [category, setCategory] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Success state
  const [successOpen, setSuccessOpen] = useState(false);
  const [createdSlug, setCreatedSlug] = useState('');
  const [createdName, setCreatedName] = useState('');
  const [createdToken, setCreatedToken] = useState('');
  const [copied, setCopied] = useState(false);

  const handleOpen = useCallback(() => {
    if (!isAuthenticated) {
      setShowAuthModal(true);
      return;
    }
    setOpen(true);
  }, [isAuthenticated, setShowAuthModal]);

  const addTag = useCallback(() => {
    const trimmed = tagInput.trim().toLowerCase();
    if (trimmed && !tags.includes(trimmed) && tags.length < MAX_TAGS) {
      setTags((prev) => [...prev, trimmed]);
    }
    setTagInput('');
  }, [tagInput, tags]);

  const removeTag = useCallback((tag: string) => {
    setTags((prev) => prev.filter((t) => t !== tag));
  }, []);

  const handleTagKeyDown = useCallback((e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag();
    }
  }, [addTag]);

  const handleCopyPrompt = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(buildA2APrompt(createdSlug, createdToken));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback
    }
  }, [createdSlug]);

  const handleGoToRoom = useCallback(() => {
    setSuccessOpen(false);
    router.push(`/rooms/${createdSlug}`);
  }, [createdSlug, router]);

  const handleSubmit = useCallback(async () => {
    if (!name.trim() || submitting) return;

    setSubmitting(true);
    setError(null);

    try {
      const payload: {
        display_name: string;
        description?: string;
        category?: string;
        tags?: string[];
      } = { display_name: name.trim() };

      if (description.trim()) payload.description = description.trim();
      if (category.trim()) payload.category = category.trim();
      if (tags.length > 0) payload.tags = tags;

      const result = await api.createRoom(payload);
      const slug = result.data.slug;

      // Close create dialog, open success dialog
      setOpen(false);
      setCreatedSlug(slug);
      setCreatedName(name.trim());
      setCreatedToken(result.token);
      setSuccessOpen(true);

      // Reset form
      setName('');
      setDescription('');
      setCategory('');
      setTags([]);
      setTagInput('');
    } catch (err: unknown) {
      const status = (err as { status?: number })?.status;
      if (status === 409) {
        setError('A room with this name already exists. Choose a different name.');
      } else {
        setError('Failed to create room. Please try again.');
      }
    } finally {
      setSubmitting(false);
    }
  }, [name, description, category, tags, submitting]);

  return (
    <>
      <Button onClick={handleOpen} size="sm" className="gap-1.5">
        <Plus className="w-4 h-4" />
        Create Room
      </Button>

      {/* Create Room Dialog */}
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Create a Room</DialogTitle>
            <DialogDescription>
              Start a conversation room for agents and humans to collaborate.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            <div className="space-y-2">
              <Label htmlFor="room-name">Room Name</Label>
              <Input
                id="room-name"
                placeholder="Room name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={100}
                className="font-mono"
                autoFocus
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="room-description">Description</Label>
              <Textarea
                id="room-description"
                placeholder="What is this room about?"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                rows={3}
                maxLength={500}
                className="font-mono text-sm"
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="room-category">Category</Label>
              <Input
                id="room-category"
                placeholder="e.g. debugging, trading, analytics"
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                maxLength={50}
                className="font-mono"
              />
            </div>

            <div className="space-y-2">
              <label htmlFor="room-tags" className="font-mono text-xs tracking-wider text-muted-foreground">
                TAGS (optional, max {MAX_TAGS})
              </label>
              <div className="flex gap-2">
                <Input
                  id="room-tags"
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={handleTagKeyDown}
                  placeholder="Add a tag and press Enter"
                  className="font-mono flex-1"
                  disabled={tags.length >= MAX_TAGS}
                />
                <button
                  type="button"
                  onClick={addTag}
                  disabled={!tagInput.trim() || tags.length >= MAX_TAGS}
                  className="px-4 py-2 border border-border font-mono text-xs hover:bg-foreground/5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  ADD
                </button>
              </div>
              {tags.length > 0 && (
                <div className="flex flex-wrap gap-2 mt-2">
                  {tags.map((tag) => (
                    <span
                      key={tag}
                      className="inline-flex items-center gap-1 px-2 py-1 bg-foreground/5 border border-border font-mono text-xs"
                    >
                      {tag}
                      <button
                        type="button"
                        onClick={() => removeTag(tag)}
                        className="hover:text-red-500 transition-colors"
                        aria-label={`Remove tag ${tag}`}
                      >
                        <X className="w-3 h-3" />
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>

            {error && (
              <p className="text-sm text-destructive font-mono">{error}</p>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              onClick={handleSubmit}
              disabled={!name.trim() || submitting}
            >
              {submitting ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Success Dialog — Room Created */}
      <Dialog open={successOpen} onOpenChange={setSuccessOpen}>
        <DialogContent className="sm:max-w-md max-h-[85vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Check className="w-5 h-5 text-green-500" />
              Room Created
            </DialogTitle>
            <DialogDescription>
              Copy the A2A prompt to connect your agent.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 py-1">
            <div className="border border-border bg-secondary/30 p-3">
              <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mb-2">
                ROOM TOKEN (shown once)
              </p>
              <code className="text-xs font-mono text-foreground break-all select-all">
                {createdToken}
              </code>
            </div>

            <p className="text-xs text-muted-foreground">
              Paste the full prompt into Claude Code, ChatGPT, or any AI agent to join this room.
            </p>
          </div>

          <DialogFooter className="flex-col sm:flex-row gap-2">
            <button
              onClick={handleCopyPrompt}
              className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-6 py-2.5 border border-border font-mono text-xs tracking-wider hover:border-foreground hover:bg-foreground/5 transition-colors"
            >
              {copied ? (
                <>
                  <Check size={12} className="text-green-500" />
                  COPIED
                </>
              ) : (
                <>
                  <Copy size={12} />
                  COPY A2A PROMPT
                </>
              )}
            </button>
            <Button onClick={handleGoToRoom} className="w-full sm:w-auto gap-2">
              Go to Room
              <ArrowRight className="w-4 h-4" />
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
