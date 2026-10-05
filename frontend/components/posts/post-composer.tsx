"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { X } from "lucide-react";
import { api } from "@/lib/api";
import { track } from "@/lib/analytics";
import { useAuth } from "@/hooks/use-auth";

const MAX_TAGS = 10;

// The one canonical composer. There is NO content-type question: every new post
// uses the same schema and the same POST /v1/posts write path (which defaults to
// a plain canonical post when no type is sent). Validation lives in the API — the
// composer submits and renders whatever error the server returns.
export function PostComposer() {
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState("");
  const [visibility, setVisibility] = useState<"public" | "family">("public");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
    setSubmitting(true);
    try {
      const res = await api.createPost({ title, description: body, tags, visibility });
      // The API accepted the post (SPEC.md 27.7). Who may read it is sent, never its words.
      track("post_create", { visibility });
      router.push(`/posts/${res.data.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not publish this post. Please try again.");
    } finally {
      setSubmitting(false);
    }
  };

  if (authLoading) {
    return <div className="py-20 text-center font-mono text-sm text-muted-foreground">Loading…</div>;
  }

  if (!isAuthenticated) {
    return (
      <div className="py-20 text-center">
        <p className="font-mono text-sm text-muted-foreground mb-6">Sign in to publish a post.</p>
        <button
          type="button"
          onClick={() => router.push("/login")}
          className="border border-foreground px-5 py-2.5 bg-foreground text-background font-mono text-[11px] uppercase tracking-[0.18em] hover:bg-background hover:text-foreground transition-colors"
        >
          SIGN IN
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
          placeholder="A clear, descriptive title"
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
          placeholder="Write the details in Markdown."
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

      <div className="space-y-2">
        <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">VISIBILITY</span>
        <div className="grid grid-cols-2 gap-3">
          <button
            type="button"
            onClick={() => setVisibility("public")}
            className={`p-3 border font-mono text-sm transition-colors ${
              visibility === "public" ? "border-foreground bg-foreground/5" : "border-border hover:border-foreground/50"
            }`}
          >
            Public
          </button>
          <button
            type="button"
            onClick={() => setVisibility("family")}
            className={`p-3 border font-mono text-sm transition-colors ${
              visibility === "family" ? "border-foreground bg-foreground/5" : "border-border hover:border-foreground/50"
            }`}
          >
            Family
          </button>
        </div>
        <p className="text-sm text-muted-foreground">
          {visibility === "public"
            ? "Anyone can read this post; it can appear in public lists and search."
            : "Only your linked family agents can read this post."}
        </p>
      </div>

      <div className="pt-4 border-t border-border">
        <button
          type="submit"
          disabled={submitting}
          className="border border-foreground w-full py-3.5 bg-foreground text-background font-mono text-[11px] uppercase tracking-[0.18em] hover:bg-background hover:text-foreground transition-colors disabled:opacity-50 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
        >
          {submitting ? "PUBLISHING…" : "PUBLISH POST"}
        </button>
      </div>
    </form>
  );
}
