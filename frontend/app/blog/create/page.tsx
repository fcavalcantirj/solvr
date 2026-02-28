"use client";

export const dynamic = 'force-dynamic';

import { useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import { CreateBlogPostData } from '@/lib/api-types';
import { Header } from '@/components/header';
import { Footer } from '@/components/footer';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { AlertCircle, X, Eye, Edit3 } from 'lucide-react';
import ReactMarkdown from 'react-markdown';

const MAX_TAGS = 10;

export default function CreateBlogPostPage() {
  const router = useRouter();
  const { isAuthenticated, isLoading: authLoading } = useAuth();

  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [coverImageUrl, setCoverImageUrl] = useState('');
  const [excerpt, setExcerpt] = useState('');
  const [metaDescription, setMetaDescription] = useState('');
  const [status, setStatus] = useState<'published' | 'draft'>('published');
  const [showPreview, setShowPreview] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const addTag = useCallback(() => {
    const tag = tagInput.trim().toLowerCase();
    if (tag && !tags.includes(tag) && tags.length < MAX_TAGS) {
      setTags((prev) => [...prev, tag]);
      setTagInput('');
    }
  }, [tagInput, tags]);

  const removeTag = useCallback((tagToRemove: string) => {
    setTags((prev) => prev.filter((t) => t !== tagToRemove));
  }, []);

  const handleTagKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag();
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!title || title.length < 10) {
      setError('Title must be at least 10 characters');
      return;
    }

    if (!body || body.length < 50) {
      setError('Body must be at least 50 characters');
      return;
    }

    setIsSubmitting(true);
    try {
      const data: CreateBlogPostData = {
        title,
        body,
        status,
        tags: tags.length > 0 ? tags : undefined,
        cover_image_url: coverImageUrl || undefined,
        excerpt: excerpt || undefined,
        meta_description: metaDescription || undefined,
      };

      const response = await api.createBlogPost(data);
      router.push(`/blog/${response.data.slug}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create blog post';
      setError(message);
    } finally {
      setIsSubmitting(false);
    }
  };

  if (authLoading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="flex items-center justify-center py-20">
          <Spinner className="w-8 h-8" />
        </div>
        <Footer />
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <div className="py-20 text-center">
          <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
            <AlertCircle className="w-8 h-8 text-muted-foreground" />
          </div>
          <h2 className="font-mono text-lg mb-2">Authentication Required</h2>
          <p className="text-muted-foreground font-mono text-sm mb-6">
            You need to sign in to create a blog post.
          </p>
          <button
            onClick={() => router.push('/login')}
            className="px-5 py-2.5 bg-foreground text-background font-mono text-xs tracking-wider hover:bg-foreground/90 transition-colors"
          >
            SIGN IN
          </button>
        </div>
        <Footer />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <div className="max-w-2xl mx-auto px-6 lg:px-12 py-12">
        <p className="font-mono text-[10px] tracking-[0.3em] text-muted-foreground mb-3">
          NEW BLOG POST
        </p>
        <h1 className="text-3xl font-light tracking-tight mb-8">
          Create Blog Post
        </h1>

        <form onSubmit={handleSubmit} className="space-y-6">
          {error && (
            <div className="p-4 border border-red-500/30 bg-red-500/10 text-red-500 font-mono text-sm">
              {error}
            </div>
          )}

          {/* Title */}
          <div className="space-y-2">
            <label htmlFor="title" className="font-mono text-xs tracking-wider text-muted-foreground">
              TITLE
            </label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Enter a clear, descriptive title (min 10 chars)"
              maxLength={300}
              className="font-mono"
            />
            <p className="font-mono text-xs text-muted-foreground">
              {title.length}/300 characters
            </p>
          </div>

          {/* Body with preview toggle */}
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label htmlFor="body" className="font-mono text-xs tracking-wider text-muted-foreground">
                BODY
              </label>
              <div className="flex gap-1">
                <button
                  type="button"
                  onClick={() => setShowPreview(false)}
                  className={`px-3 py-1 font-mono text-xs transition-colors ${
                    !showPreview ? 'bg-foreground/10 text-foreground' : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <Edit3 className="w-3 h-3 inline mr-1" />
                  Edit
                </button>
                <button
                  type="button"
                  onClick={() => setShowPreview(true)}
                  className={`px-3 py-1 font-mono text-xs transition-colors ${
                    showPreview ? 'bg-foreground/10 text-foreground' : 'text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <Eye className="w-3 h-3 inline mr-1" />
                  Preview
                </button>
              </div>
            </div>
            {showPreview ? (
              <div className="min-h-[200px] p-4 border border-border prose prose-invert max-w-none font-mono text-sm">
                <ReactMarkdown>{body}</ReactMarkdown>
              </div>
            ) : (
              <textarea
                id="body"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                placeholder="Write your blog post content in markdown (min 50 chars)"
                rows={12}
                className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm rounded-md focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none resize-none"
              />
            )}
            <p className="font-mono text-xs text-muted-foreground">
              {body.length} characters (min 50)
            </p>
          </div>

          {/* Tags */}
          <div className="space-y-2">
            <label htmlFor="tags" className="font-mono text-xs tracking-wider text-muted-foreground">
              TAGS (optional, max {MAX_TAGS})
            </label>
            <div className="flex gap-2">
              <Input
                id="tags"
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
                      data-testid={`remove-tag-${tag}`}
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

          {/* Cover Image URL */}
          <div className="space-y-2">
            <label htmlFor="cover-image" className="font-mono text-xs tracking-wider text-muted-foreground">
              COVER IMAGE URL (optional)
            </label>
            <Input
              id="cover-image"
              value={coverImageUrl}
              onChange={(e) => setCoverImageUrl(e.target.value)}
              placeholder="https://example.com/image.jpg"
              className="font-mono"
            />
          </div>

          {/* Excerpt */}
          <div className="space-y-2">
            <label htmlFor="excerpt" className="font-mono text-xs tracking-wider text-muted-foreground">
              EXCERPT (optional)
            </label>
            <textarea
              id="excerpt"
              value={excerpt}
              onChange={(e) => setExcerpt(e.target.value)}
              placeholder="Auto-generated from body if left empty"
              rows={3}
              className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm rounded-md focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none resize-none"
            />
          </div>

          {/* Meta Description */}
          <div className="space-y-2">
            <label htmlFor="meta-description" className="font-mono text-xs tracking-wider text-muted-foreground">
              META DESCRIPTION (optional)
            </label>
            <textarea
              id="meta-description"
              value={metaDescription}
              onChange={(e) => setMetaDescription(e.target.value.slice(0, 160))}
              placeholder="SEO description for search engines (max 160 chars)"
              rows={2}
              maxLength={160}
              className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm rounded-md focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none resize-none"
            />
            <p className="font-mono text-xs text-muted-foreground">
              {metaDescription.length}/160 characters
            </p>
          </div>

          {/* Status toggle */}
          <div className="space-y-2">
            <label className="font-mono text-xs tracking-wider text-muted-foreground">STATUS</label>
            <div className="flex gap-3">
              <button
                type="button"
                onClick={() => setStatus('published')}
                className={`px-4 py-2 border font-mono text-xs transition-colors ${
                  status === 'published'
                    ? 'border-foreground bg-foreground/5'
                    : 'border-border hover:border-foreground/50'
                }`}
              >
                Publish
              </button>
              <button
                type="button"
                onClick={() => setStatus('draft')}
                className={`px-4 py-2 border font-mono text-xs transition-colors ${
                  status === 'draft'
                    ? 'border-foreground bg-foreground/5'
                    : 'border-border hover:border-foreground/50'
                }`}
              >
                Draft
              </button>
            </div>
          </div>

          {/* Submit */}
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
                'CREATE BLOG POST'
              )}
            </button>
          </div>
        </form>
      </div>
      <Footer />
    </div>
  );
}
