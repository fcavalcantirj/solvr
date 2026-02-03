'use client';

/**
 * New Idea Page
 * Per SPEC.md Part 4.8 (New Post Pages) and PRD line 484:
 *   - Form with title, description, tags
 *   - Preview tab for markdown rendering
 *   - Submit to POST /v1/ideas
 *   - Redirect to created post
 */

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { api, ApiError } from '@/lib/api';
import { trackEvent } from '@/lib/analytics';

// Markdown rendering for preview
import { marked } from 'marked';

interface FormData {
  title: string;
  description: string;
  tags: string;
}

interface FormErrors {
  title?: string;
  description?: string;
  tags?: string;
  general?: string;
}

interface CreatedIdea {
  id: string;
  type: string;
  title: string;
}

export default function NewIdeaPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<'write' | 'preview'>('write');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errors, setErrors] = useState<FormErrors>({});

  const [formData, setFormData] = useState<FormData>({
    title: '',
    description: '',
    tags: '',
  });

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!authLoading && !user) {
      router.push('/login?redirect=/new/idea');
    }
  }, [authLoading, user, router]);

  // Show loading while checking auth
  if (authLoading) {
    return (
      <main className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mx-auto mb-4"></div>
          <p className="text-muted">Loading...</p>
        </div>
      </main>
    );
  }

  // Don't render form if not authenticated (will redirect)
  if (!user) {
    return null;
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: value,
    }));

    // Clear field error on change
    if (errors[name as keyof FormErrors]) {
      setErrors((prev) => ({ ...prev, [name]: undefined }));
    }
  };

  const validateForm = (): boolean => {
    const newErrors: FormErrors = {};

    // Title validation
    if (formData.title.length < 10) {
      newErrors.title = 'Title must be at least 10 characters';
    }

    // Description validation
    if (formData.description.length < 50) {
      newErrors.description = 'Description must be at least 50 characters';
    }

    // Tags validation (max 5)
    const tags = parseTags(formData.tags);
    if (tags.length > 5) {
      newErrors.tags = 'Maximum 5 tags allowed';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const parseTags = (tagsString: string): string[] => {
    return tagsString
      .split(',')
      .map((tag) => tag.trim())
      .filter((tag) => tag.length > 0);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Clear previous errors
    setErrors({});

    // Validate
    if (!validateForm()) {
      return;
    }

    setIsSubmitting(true);

    try {
      const payload = {
        type: 'idea',
        title: formData.title,
        description: formData.description,
        tags: parseTags(formData.tags),
        status: 'open',
      };

      const result = await api.post<CreatedIdea>('/v1/ideas', payload);

      // Track PostCreated event (per SPEC.md Part 19.3)
      trackEvent('PostCreated', {
        type: 'idea',
        author_type: 'human',
      });

      router.push(`/posts/${result.id}`);
    } catch (error) {
      if (error instanceof ApiError) {
        setErrors({ general: error.message });
      } else {
        setErrors({ general: 'Failed to submit idea. Please try again.' });
      }
      setIsSubmitting(false);
    }
  };

  const handleCancel = () => {
    router.back();
  };

  const dismissError = () => {
    setErrors((prev) => ({ ...prev, general: undefined }));
  };

  // Render markdown for preview
  const renderMarkdown = (text: string): string => {
    try {
      return marked(text) as string;
    } catch {
      return text;
    }
  };

  return (
    <main className="min-h-screen py-8 px-4">
      <div className="max-w-3xl mx-auto" data-testid="new-idea-container">
        <h1 className="text-2xl font-bold mb-6">New Idea</h1>

        {/* Error Alert */}
        {errors.general && (
          <div
            role="alert"
            className="mb-6 p-4 bg-red-50 border border-red-200 rounded-lg flex items-start justify-between"
          >
            <p className="text-red-700">{errors.general}</p>
            <button
              onClick={dismissError}
              className="text-red-500 hover:text-red-700 ml-4"
              aria-label="Dismiss"
            >
              ×
            </button>
          </div>
        )}

        {/* Tab Buttons */}
        <div className="flex border-b border-gray-200 mb-6" role="tablist">
          <button
            role="tab"
            aria-selected={activeTab === 'write'}
            onClick={() => setActiveTab('write')}
            className={`px-4 py-2 font-medium -mb-px ${
              activeTab === 'write'
                ? 'border-b-2 border-primary text-primary'
                : 'text-muted hover:text-foreground'
            }`}
          >
            Write
          </button>
          <button
            role="tab"
            aria-selected={activeTab === 'preview'}
            onClick={() => setActiveTab('preview')}
            className={`px-4 py-2 font-medium -mb-px ${
              activeTab === 'preview'
                ? 'border-b-2 border-primary text-primary'
                : 'text-muted hover:text-foreground'
            }`}
          >
            Preview
          </button>
        </div>

        {/* Preview Panel */}
        {activeTab === 'preview' && (
          <div
            data-testid="preview-area"
            className="min-h-[300px] p-4 border rounded-lg bg-gray-50 mb-6 prose prose-sm max-w-none"
          >
            <h2 className="text-xl font-semibold mb-2">{formData.title || 'Untitled Idea'}</h2>
            <div
              dangerouslySetInnerHTML={{
                __html: renderMarkdown(formData.description || '*No description yet*'),
              }}
            />
            {formData.tags && (
              <div className="mt-4">
                <span className="font-medium">Tags: </span>
                {parseTags(formData.tags).map((tag, i) => (
                  <span key={i} className="inline-block bg-gray-200 px-2 py-1 rounded mr-2 text-sm">
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </div>
        )}

        {/* Form */}
        {activeTab === 'write' && (
          <form role="form" onSubmit={handleSubmit} className="space-y-6">
            {/* Title */}
            <div>
              <label htmlFor="title" className="block font-medium mb-1">
                Title
              </label>
              <input
                type="text"
                id="title"
                name="title"
                value={formData.title}
                onChange={handleInputChange}
                required
                maxLength={200}
                className={`w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent ${
                  errors.title ? 'border-red-500' : 'border-gray-300'
                }`}
                placeholder="Share your idea or observation"
              />
              <p className="text-sm text-muted mt-1">Maximum 200 characters</p>
              {errors.title && <p className="text-sm text-red-600 mt-1">{errors.title}</p>}
            </div>

            {/* Description */}
            <div>
              <label htmlFor="description" className="block font-medium mb-1">
                Description
              </label>
              <textarea
                id="description"
                name="description"
                value={formData.description}
                onChange={handleInputChange}
                required
                rows={10}
                className={`w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent font-mono text-sm ${
                  errors.description ? 'border-red-500' : 'border-gray-300'
                }`}
                placeholder="Describe your idea in detail. Share your thoughts, observations, or suggestions. You can use Markdown for formatting."
              />
              <p className="text-sm text-muted mt-1">Supports Markdown. Minimum 50 characters.</p>
              {errors.description && (
                <p className="text-sm text-red-600 mt-1">{errors.description}</p>
              )}
            </div>

            {/* Tags */}
            <div>
              <label htmlFor="tags" className="block font-medium mb-1">
                Tags
              </label>
              <input
                type="text"
                id="tags"
                name="tags"
                value={formData.tags}
                onChange={handleInputChange}
                className={`w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent ${
                  errors.tags ? 'border-red-500' : 'border-gray-300'
                }`}
                placeholder="innovation, design, patterns"
              />
              <p className="text-sm text-muted mt-1">Comma-separated, up to 5 tags</p>
              {errors.tags && <p className="text-sm text-red-600 mt-1">{errors.tags}</p>}
            </div>

            {/* Buttons */}
            <div className="flex gap-4 pt-4">
              <button
                type="submit"
                disabled={isSubmitting}
                className="px-6 py-2 bg-primary text-white rounded-lg hover:bg-primary-dark disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSubmitting ? 'Sharing...' : 'Share Idea'}
              </button>
              <button
                type="button"
                onClick={handleCancel}
                className="px-6 py-2 border border-gray-300 rounded-lg hover:bg-gray-50"
              >
                Cancel
              </button>
            </div>
          </form>
        )}
      </div>
    </main>
  );
}
