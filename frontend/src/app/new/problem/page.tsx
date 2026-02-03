'use client';

/**
 * New Problem Page
 * Per SPEC.md Part 4.8 (New Post Pages) and PRD lines 479-482:
 *   - Form with title, description, tags, success_criteria, weight
 *   - Preview tab for markdown rendering
 *   - Submit to POST /v1/problems
 *   - Redirect to created post
 */

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';
import { api, ApiError } from '@/lib/api';

// Markdown rendering for preview
import { marked } from 'marked';

interface FormData {
  title: string;
  description: string;
  tags: string;
  successCriteria: string;
  weight: number;
}

interface FormErrors {
  title?: string;
  description?: string;
  tags?: string;
  successCriteria?: string;
  general?: string;
}

interface CreatedProblem {
  id: string;
  type: string;
  title: string;
}

export default function NewProblemPage() {
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const [activeTab, setActiveTab] = useState<'write' | 'preview'>('write');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errors, setErrors] = useState<FormErrors>({});

  const [formData, setFormData] = useState<FormData>({
    title: '',
    description: '',
    tags: '',
    successCriteria: '',
    weight: 3,
  });

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!authLoading && !user) {
      router.push('/login?redirect=/new/problem');
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

  const handleInputChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>
  ) => {
    const { name, value } = e.target;
    setFormData((prev) => ({
      ...prev,
      [name]: name === 'weight' ? parseInt(value, 10) : value,
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

  const parseSuccessCriteria = (criteriaString: string): string[] => {
    return criteriaString
      .split('\n')
      .map((criterion) => criterion.trim())
      .filter((criterion) => criterion.length > 0);
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
        type: 'problem',
        title: formData.title,
        description: formData.description,
        tags: parseTags(formData.tags),
        success_criteria: parseSuccessCriteria(formData.successCriteria),
        weight: formData.weight,
        status: 'open',
      };

      const result = await api.post<CreatedProblem>('/v1/problems', payload);
      router.push(`/posts/${result.id}`);
    } catch (error) {
      if (error instanceof ApiError) {
        setErrors({ general: error.message });
      } else {
        setErrors({ general: 'Failed to create problem. Please try again.' });
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
      <div
        className="max-w-3xl mx-auto"
        data-testid="new-problem-container"
      >
        <h1 className="text-2xl font-bold mb-6">New Problem</h1>

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
            <h2 className="text-xl font-semibold mb-2">
              {formData.title || 'Untitled Problem'}
            </h2>
            <div
              dangerouslySetInnerHTML={{
                __html: renderMarkdown(formData.description || '*No description yet*'),
              }}
            />
            {formData.tags && (
              <div className="mt-4">
                <span className="font-medium">Tags: </span>
                {parseTags(formData.tags).map((tag, i) => (
                  <span
                    key={i}
                    className="inline-block bg-gray-200 px-2 py-1 rounded mr-2 text-sm"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
            {formData.successCriteria && (
              <div className="mt-4">
                <span className="font-medium">Success Criteria:</span>
                <ul className="list-disc pl-5 mt-1">
                  {parseSuccessCriteria(formData.successCriteria).map((c, i) => (
                    <li key={i}>{c}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {/* Form */}
        {activeTab === 'write' && (
          <form
            role="form"
            onSubmit={handleSubmit}
            className="space-y-6"
          >
            {/* Title */}
            <div>
              <label
                htmlFor="title"
                className="block font-medium mb-1"
              >
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
                placeholder="A clear, concise title for your problem"
              />
              <p className="text-sm text-muted mt-1">
                Maximum 200 characters
              </p>
              {errors.title && (
                <p className="text-sm text-red-600 mt-1">{errors.title}</p>
              )}
            </div>

            {/* Description */}
            <div>
              <label
                htmlFor="description"
                className="block font-medium mb-1"
              >
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
                placeholder="Describe the problem in detail. You can use Markdown for formatting."
              />
              <p className="text-sm text-muted mt-1">
                Supports Markdown. Minimum 50 characters.
              </p>
              {errors.description && (
                <p className="text-sm text-red-600 mt-1">{errors.description}</p>
              )}
            </div>

            {/* Tags */}
            <div>
              <label
                htmlFor="tags"
                className="block font-medium mb-1"
              >
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
                placeholder="golang, postgresql, async"
              />
              <p className="text-sm text-muted mt-1">
                Comma-separated, up to 5 tags
              </p>
              {errors.tags && (
                <p className="text-sm text-red-600 mt-1">{errors.tags}</p>
              )}
            </div>

            {/* Success Criteria */}
            <div>
              <label
                htmlFor="successCriteria"
                className="block font-medium mb-1"
              >
                Success Criteria
              </label>
              <textarea
                id="successCriteria"
                name="successCriteria"
                value={formData.successCriteria}
                onChange={handleInputChange}
                rows={4}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent"
                placeholder="Define what success looks like (one per line)"
              />
              <p className="text-sm text-muted mt-1">
                One criterion per line. These help others know when the problem is solved.
              </p>
            </div>

            {/* Difficulty/Weight */}
            <div>
              <label
                htmlFor="weight"
                className="block font-medium mb-1"
              >
                Difficulty
              </label>
              <select
                id="weight"
                name="weight"
                value={formData.weight}
                onChange={handleInputChange}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary focus:border-transparent"
              >
                <option value={1}>1 - Easy</option>
                <option value={2}>2 - Moderate</option>
                <option value={3}>3 - Medium</option>
                <option value={4}>4 - Hard</option>
                <option value={5}>5 - Very Hard</option>
              </select>
              <p className="text-sm text-muted mt-1">
                Estimated difficulty level (1-5)
              </p>
            </div>

            {/* Buttons */}
            <div className="flex gap-4 pt-4">
              <button
                type="submit"
                disabled={isSubmitting}
                className="px-6 py-2 bg-primary text-white rounded-lg hover:bg-primary-dark disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSubmitting ? 'Creating...' : 'Create Problem'}
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
