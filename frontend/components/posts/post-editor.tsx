"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { X } from "lucide-react";
import { api } from "@/lib/api";

const MAX_TAGS = 10;

// Edit updates only the content the author owns — title, body, tags. It never
// sends author, timestamps, votes, or source attribution, so the server-owned
// identity and history of the post are preserved across an edit.
export function PostEditor({ postId }: { postId: string }) {
  const router = useRouter();

  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  // The version this form loaded: the edit sends it back as If-Match, so it can never
  // overwrite a newer edit (the API answers 412 with its own message instead).
  const [version, setVersion] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadError(false);
    try {
      const res = await api.getPost(postId);
      setTitle(res.data.title);
      setBody(res.data.description);
      setTags(res.data.tags ?? []);
      setVersion(res.etag ?? null);
    } catch {
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  }, [postId]);

  useEffect(() => {
    load();
  }, [load]);

  const addTag = () => {
    const tag = tagInput.trim().toLowerCase();
    if (tag && !tags.includes(tag) && tags.length < MAX_TAGS) {
      setTags([...tags, tag]);
      setTagInput("");
    }
  };

  const removeTag = (t: string) => setTags(tags.filter((x) => x !== t));

  const handleTagKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      addTag();
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSaving(true);
    try {
      await api.updatePost(postId, { title, description: body, tags }, version);
      router.push(`/posts/${postId}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save your changes. Please try again.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <div className="py-20 text-center font-mono text-sm text-muted-foreground">Loading…</div>;
  }

  if (loadError) {
    return (
      <div className="py-20 text-center space-y-4">
        <p className="font-mono text-sm text-muted-foreground">Could not load this post.</p>
        <button
          type="button"
          onClick={load}
          className="px-4 py-2 border border-border font-mono text-xs hover:border-foreground transition-colors"
        >
          RETRY
        </button>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {error && (
        <div className="p-4 border border-red-500/30 bg-red-500/10 text-red-500 font-mono text-sm">
          {error}
        </div>
      )}

      <div className="space-y-2">
        <label htmlFor="title" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
          TITLE
        </label>
        <input
          id="title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          maxLength={200}
          className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm outline-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
        />
      </div>

      <div className="space-y-2">
        <label htmlFor="body" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
          BODY (Markdown)
        </label>
        <textarea
          id="body"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={10}
          className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm outline-none resize-y focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px]"
        />
      </div>

      <div className="space-y-2">
        <label htmlFor="tags" className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
          TAGS (optional, max {MAX_TAGS})
        </label>
        <div className="flex gap-2">
          <input
            id="tags"
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
            onKeyDown={handleTagKeyDown}
            placeholder="Add a tag and press Enter"
            disabled={tags.length >= MAX_TAGS}
            className="flex-1 px-3 py-2 border border-input bg-transparent font-mono text-sm outline-none focus-visible:border-ring"
          />
          <button
            type="button"
            onClick={addTag}
            disabled={!tagInput.trim() || tags.length >= MAX_TAGS}
            className="px-4 py-2.5 border border-border font-mono text-[11px] uppercase tracking-[0.18em] hover:border-foreground transition-colors disabled:opacity-50"
          >
            ADD
          </button>
        </div>
        {tags.length > 0 && (
          <div className="flex flex-wrap gap-2 mt-2">
            {tags.map((t) => (
              <span
                key={t}
                className="inline-flex items-center gap-1 px-2 py-1 bg-foreground/5 border border-border font-mono text-xs"
              >
                {t}
                <button type="button" onClick={() => removeTag(t)} aria-label={`remove ${t}`} className="hover:text-red-500">
                  <X className="w-3 h-3" />
                </button>
              </span>
            ))}
          </div>
        )}
      </div>

      <div className="pt-4 border-t border-border">
        <button
          type="submit"
          disabled={saving}
          className="border border-foreground w-full py-3.5 bg-foreground text-background font-mono text-[11px] uppercase tracking-[0.18em] hover:bg-background hover:text-foreground transition-colors disabled:opacity-50 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
        >
          {saving ? "SAVING…" : "SAVE CHANGES"}
        </button>
      </div>
    </form>
  );
}
