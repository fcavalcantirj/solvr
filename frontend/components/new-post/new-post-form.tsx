"use client";

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { useCreatePost } from '@/hooks/use-create-post';
import { useAuth } from '@/hooks/use-auth';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { AlertCircle, X } from 'lucide-react';

const POST_TYPES = [
  { value: 'question', label: 'Question', description: 'Ask a specific question to get answers' },
  { value: 'problem', label: 'Problem', description: 'Describe a problem to explore approaches' },
  { value: 'idea', label: 'Idea', description: 'Share an idea for discussion and feedback' },
] as const;

export function NewPostForm() {
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const { form, updateForm, isSubmitting, error, submit } = useCreatePost();
  const [tagInput, setTagInput] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const post = await submit();
    if (post) {
      const route = form.type === 'problem' ? 'problems' : form.type === 'idea' ? 'ideas' : 'questions';
      router.push(`/${route}/${post.id}`);
    }
  };

  const addTag = () => {
    const tag = tagInput.trim().toLowerCase();
    if (tag && !form.tags.includes(tag) && form.tags.length < 5) {
      updateForm({ tags: [...form.tags, tag] });
      setTagInput('');
    }
  };

  const removeTag = (tagToRemove: string) => {
    updateForm({ tags: form.tags.filter((t) => t !== tagToRemove) });
  };

  const handleTagKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag();
    }
  };

  if (authLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="py-20 text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <AlertCircle className="w-8 h-8 text-muted-foreground" />
        </div>
        <h2 className="font-mono text-lg mb-2">Authentication Required</h2>
        <p className="text-muted-foreground font-mono text-sm mb-6">
          You need to sign in to create a post.
        </p>
        <button
          onClick={() => router.push('/login')}
          className="px-5 py-2.5 bg-foreground text-background font-mono text-xs tracking-wider hover:bg-foreground/90 transition-colors"
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
        <label className="font-mono text-xs tracking-wider text-muted-foreground">TYPE</label>
        <div className="grid grid-cols-3 gap-3">
          {POST_TYPES.map((type) => (
            <button
              key={type.value}
              type="button"
              onClick={() => updateForm({ type: type.value })}
              className={`p-4 border text-left transition-colors ${
                form.type === type.value
                  ? 'border-foreground bg-foreground/5'
                  : 'border-border hover:border-foreground/50'
              }`}
            >
              <span className="font-mono text-sm block mb-1">{type.label}</span>
              <span className="font-mono text-xs text-muted-foreground">{type.description}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="space-y-2">
        <label htmlFor="title" className="font-mono text-xs tracking-wider text-muted-foreground">
          TITLE
        </label>
        <Input
          id="title"
          value={form.title}
          onChange={(e) => updateForm({ title: e.target.value })}
          placeholder="Enter a clear, descriptive title (min 10 chars)"
          maxLength={200}
          className="font-mono"
        />
        <p className="font-mono text-xs text-muted-foreground">
          {form.title.length}/200 characters
        </p>
      </div>

      <div className="space-y-2">
        <label htmlFor="description" className="font-mono text-xs tracking-wider text-muted-foreground">
          DESCRIPTION
        </label>
        <textarea
          id="description"
          value={form.description}
          onChange={(e) => updateForm({ description: e.target.value })}
          placeholder="Provide detailed context and information (min 50 chars)"
          rows={8}
          className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm rounded-md focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none resize-none"
        />
        <p className="font-mono text-xs text-muted-foreground">
          {form.description.length} characters (min 50)
        </p>
      </div>

      <div className="space-y-2">
        <label htmlFor="tags" className="font-mono text-xs tracking-wider text-muted-foreground">
          TAGS (optional, max 5)
        </label>
        <div className="flex gap-2">
          <Input
            id="tags"
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
            onKeyDown={handleTagKeyDown}
            placeholder="Add a tag and press Enter"
            className="font-mono flex-1"
            disabled={form.tags.length >= 5}
          />
          <button
            type="button"
            onClick={addTag}
            disabled={!tagInput.trim() || form.tags.length >= 5}
            className="px-4 py-2 border border-border font-mono text-xs hover:bg-foreground/5 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            ADD
          </button>
        </div>
        {form.tags.length > 0 && (
          <div className="flex flex-wrap gap-2 mt-2">
            {form.tags.map((tag) => (
              <span
                key={tag}
                className="inline-flex items-center gap-1 px-2 py-1 bg-foreground/5 border border-border font-mono text-xs"
              >
                {tag}
                <button
                  type="button"
                  onClick={() => removeTag(tag)}
                  className="hover:text-red-500 transition-colors"
                >
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
          disabled={isSubmitting}
          className="w-full py-3 bg-foreground text-background font-mono text-sm tracking-wider hover:bg-foreground/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
        >
          {isSubmitting ? (
            <>
              <Spinner className="w-4 h-4" />
              CREATING...
            </>
          ) : (
            'CREATE POST'
          )}
        </button>
      </div>
    </form>
  );
}
