"use client";

import { useState, useEffect, useCallback, useRef } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import type { APIPost, UpdatePostData } from '@/lib/api-types';
import { useToast } from '@/components/ui/use-toast';
import { Input } from '@/components/ui/input';
import { Spinner } from '@/components/ui/spinner';
import { AlertCircle, AlertTriangle, ArrowLeft, X } from 'lucide-react';
import { mapStatus } from '@/lib/api';

const MAX_TAGS = 10;

interface EditPostFormProps {
  postId: string;
  postType: 'problems' | 'questions' | 'ideas';
}

export function EditPostForm({ postId, postType }: EditPostFormProps) {
  const router = useRouter();
  const { user, isAuthenticated, isLoading: authLoading } = useAuth();
  const { toast } = useToast();

  const [post, setPost] = useState<APIPost | null>(null);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [rejectionReason, setRejectionReason] = useState<string | null>(null);

  // Form state
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [tagInput, setTagInput] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  // Store original values to compute diff
  const originalRef = useRef<{ title: string; description: string; tags: string[] }>({
    title: '',
    description: '',
    tags: [],
  });

  // Fetch the post and comments
  useEffect(() => {
    let cancelled = false;

    async function fetchPost() {
      try {
        const [postResponse, commentsResponse] = await Promise.all([
          api.getPost(postId),
          api.getComments('post', postId, { per_page: 20 }),
        ]);

        if (cancelled) return;

        const postData = postResponse.data;
        setPost(postData);
        setTitle(postData.title);
        setDescription(postData.description);
        setTags(postData.tags || []);
        originalRef.current = {
          title: postData.title,
          description: postData.description,
          tags: postData.tags || [],
        };

        // Find latest system comment for rejection reason
        if (postData.status === 'rejected') {
          const systemComments = commentsResponse.data.filter(
            (c) => c.author_type === 'system'
          );
          if (systemComments.length > 0) {
            // Get the last system comment
            const latestSystemComment = systemComments[systemComments.length - 1];
            setRejectionReason(latestSystemComment.content);
          }
        }

        setLoading(false);
      } catch {
        if (cancelled) return;
        setFetchError('Failed to load post');
        setLoading(false);
      }
    }

    fetchPost();
    return () => { cancelled = true; };
  }, [postId]);

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
    setValidationError(null);

    // Validate
    if (!title || title.length < 10) {
      setValidationError('Title must be at least 10 characters');
      return;
    }
    if (title.length > 200) {
      setValidationError('Title must be at most 200 characters');
      return;
    }
    if (!description || description.length < 50) {
      setValidationError('Description must be at least 50 characters');
      return;
    }

    // Compute changed fields only
    const changes: UpdatePostData = {};
    if (title !== originalRef.current.title) {
      changes.title = title;
    }
    if (description !== originalRef.current.description) {
      changes.description = description;
    }
    const tagsChanged =
      tags.length !== originalRef.current.tags.length ||
      tags.some((t, i) => t !== originalRef.current.tags[i]);
    if (tagsChanged) {
      changes.tags = tags;
    }

    // Nothing changed
    if (Object.keys(changes).length === 0) {
      toast({ title: 'No changes', description: 'No changes were made to the post.' });
      return;
    }

    setIsSubmitting(true);
    try {
      await api.updatePost(postId, changes);
      const isRejected = post?.status === 'rejected';
      toast({
        title: 'Post updated',
        description: isRejected
          ? 'Post updated and resubmitted for review'
          : 'Your changes have been saved',
      });
      router.push(`/${postType}/${postId}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update post';
      setValidationError(message);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Loading states
  if (authLoading || loading) {
    return (
      <div data-testid="edit-post-loading" className="flex items-center justify-center py-20">
        <Spinner className="w-8 h-8" />
      </div>
    );
  }

  // Fetch error
  if (fetchError) {
    return (
      <div className="py-20 text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <AlertCircle className="w-8 h-8 text-red-500" />
        </div>
        <h2 className="font-mono text-lg mb-2">Failed to Load Post</h2>
        <p className="text-muted-foreground font-mono text-sm">{fetchError}</p>
      </div>
    );
  }

  // Auth check
  if (!isAuthenticated || !user) {
    return (
      <div className="py-20 text-center">
        <h2 className="font-mono text-lg mb-2">Authentication Required</h2>
        <p className="text-muted-foreground font-mono text-sm">
          You need to sign in to edit a post.
        </p>
      </div>
    );
  }

  // Ownership check
  if (!post || user.id !== post.author.id) {
    return (
      <div className="py-20 text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <AlertCircle className="w-8 h-8 text-muted-foreground" />
        </div>
        <h2 className="font-mono text-lg mb-2">Not Authorized</h2>
        <p className="text-muted-foreground font-mono text-sm">
          Only the post author can edit this post.
        </p>
      </div>
    );
  }

  const displayStatus = mapStatus(post.status);
  const isRejected = post.status === 'rejected';
  const isPendingReview = post.status === 'pending_review';

  const statusColor = isRejected
    ? 'border-red-500/30 bg-red-500/10 text-red-600'
    : isPendingReview
      ? 'border-yellow-500/30 bg-yellow-500/10 text-yellow-600'
      : 'border-green-500/30 bg-green-500/10 text-green-600';

  return (
    <div>
      {/* Back link */}
      <button
        type="button"
        onClick={() => router.push(`/${postType}/${postId}`)}
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft size={14} />
        BACK TO POST
      </button>

      <h1 className="text-2xl font-light tracking-tight mb-6">Edit Post</h1>

      {/* Status badge */}
      <div className={`inline-flex items-center px-3 py-1.5 border font-mono text-xs tracking-wider mb-6 ${statusColor}`}>
        {displayStatus}
      </div>

      {/* Rejection reason */}
      {isRejected && rejectionReason && (
        <div className="mb-6 p-4 border border-red-500/30 bg-red-500/5">
          <div className="flex items-start gap-3">
            <AlertTriangle size={16} className="text-red-600 mt-0.5 flex-shrink-0" />
            <div>
              <p className="font-mono text-xs font-medium text-red-700 mb-1">Rejection Reason</p>
              <p className="font-mono text-xs text-red-600">{rejectionReason}</p>
            </div>
          </div>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {validationError && (
          <div className="p-4 border border-red-500/30 bg-red-500/10 text-red-500 font-mono text-sm">
            {validationError}
          </div>
        )}

        {/* Title */}
        <div className="space-y-2">
          <label htmlFor="edit-title" className="font-mono text-xs tracking-wider text-muted-foreground">
            TITLE
          </label>
          <Input
            id="edit-title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Enter a clear, descriptive title (min 10 chars)"
            maxLength={200}
            className="font-mono"
          />
          <p className="font-mono text-xs text-muted-foreground">
            {title.length}/200 characters
          </p>
        </div>

        {/* Description */}
        <div className="space-y-2">
          <label htmlFor="edit-description" className="font-mono text-xs tracking-wider text-muted-foreground">
            DESCRIPTION
          </label>
          <textarea
            id="edit-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Provide detailed context and information (min 50 chars)"
            rows={8}
            className="w-full px-3 py-2 border border-input bg-transparent font-mono text-sm rounded-md focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] outline-none resize-none"
          />
          <p className="font-mono text-xs text-muted-foreground">
            {description.length} characters (min 50)
          </p>
        </div>

        {/* Tags */}
        <div className="space-y-2">
          <label htmlFor="edit-tags" className="font-mono text-xs tracking-wider text-muted-foreground">
            TAGS (optional, max {MAX_TAGS})
          </label>
          <div className="flex gap-2">
            <Input
              id="edit-tags"
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
                  >
                    <X className="w-3 h-3" />
                  </button>
                </span>
              ))}
            </div>
          )}
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
                SAVING...
              </>
            ) : isRejected ? (
              'SAVE & RESUBMIT FOR REVIEW'
            ) : (
              'SAVE CHANGES'
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
