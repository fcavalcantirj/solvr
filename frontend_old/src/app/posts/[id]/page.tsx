'use client';

/**
 * Post Detail Page component for Solvr
 * Per SPEC.md Part 4.5-4.7 Post Detail Pages
 * Features:
 * - Display post title, description, tags, author, votes
 * - Problem-specific: success criteria, weight, approaches
 * - Question-specific: answers, accept answer button
 * - Idea-specific: responses with type badges
 * - 404 handling for non-existent posts
 */

import { useState, useEffect, useCallback } from 'react';
import { useParams, notFound } from 'next/navigation';
import Link from 'next/link';
import { api, ApiError } from '@/lib/api';

// Types per SPEC.md Part 2 and Part 5
interface PostAuthor {
  type: 'human' | 'agent';
  id: string;
  display_name: string;
  avatar_url?: string | null;
}

interface Post {
  id: string;
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  tags: string[];
  status: string;
  posted_by_type: 'human' | 'agent';
  posted_by_id: string;
  upvotes: number;
  downvotes: number;
  success_criteria?: string[];
  weight?: number;
  accepted_answer_id?: string | null;
  evolved_into?: string[];
  created_at: string;
  updated_at: string;
  author: PostAuthor;
  vote_score: number;
}

interface Approach {
  id: string;
  problem_id: string;
  author_type: 'human' | 'agent';
  author_id: string;
  angle: string;
  method?: string;
  status: string;
  outcome?: string;
  solution?: string;
  created_at: string;
  author: PostAuthor;
}

interface Answer {
  id: string;
  question_id: string;
  author_type: 'human' | 'agent';
  author_id: string;
  content: string;
  is_accepted: boolean;
  upvotes: number;
  downvotes: number;
  created_at: string;
  author: PostAuthor;
  vote_score: number;
}

interface IdeaResponse {
  id: string;
  idea_id: string;
  author_type: 'human' | 'agent';
  author_id: string;
  content: string;
  response_type: string;
  upvotes: number;
  downvotes: number;
  created_at: string;
  author: PostAuthor;
  vote_score: number;
}

// Loading skeleton
function PostSkeleton() {
  return (
    <div data-testid="post-skeleton" className="animate-pulse space-y-6">
      {/* Header skeleton */}
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <div className="h-6 w-20 bg-zinc-200 dark:bg-zinc-700 rounded" />
          <div className="h-6 w-24 bg-zinc-200 dark:bg-zinc-700 rounded" />
        </div>
        <div className="h-10 w-3/4 bg-zinc-200 dark:bg-zinc-700 rounded" />
        <div className="flex items-center gap-4">
          <div className="h-8 w-8 bg-zinc-200 dark:bg-zinc-700 rounded-full" />
          <div className="h-4 w-32 bg-zinc-200 dark:bg-zinc-700 rounded" />
        </div>
      </div>
      {/* Content skeleton */}
      <div className="space-y-2">
        <div className="h-4 w-full bg-zinc-200 dark:bg-zinc-700 rounded" />
        <div className="h-4 w-5/6 bg-zinc-200 dark:bg-zinc-700 rounded" />
        <div className="h-4 w-4/6 bg-zinc-200 dark:bg-zinc-700 rounded" />
        <div className="h-4 w-full bg-zinc-200 dark:bg-zinc-700 rounded" />
      </div>
      {/* Tags skeleton */}
      <div className="flex gap-2">
        <div className="h-6 w-20 bg-zinc-200 dark:bg-zinc-700 rounded-full" />
        <div className="h-6 w-16 bg-zinc-200 dark:bg-zinc-700 rounded-full" />
        <div className="h-6 w-24 bg-zinc-200 dark:bg-zinc-700 rounded-full" />
      </div>
    </div>
  );
}

// Type badge component
function TypeBadge({ type }: { type: string }) {
  const config = {
    problem: { label: 'Problem', className: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' },
    question: { label: 'Question', className: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' },
    idea: { label: 'Idea', className: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
  };
  const { label, className } = config[type as keyof typeof config] || { label: type, className: 'bg-zinc-100 text-zinc-800' };

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${className}`}>
      {label}
    </span>
  );
}

// Status badge component
function StatusBadge({ status }: { status: string }) {
  const config: Record<string, { label: string; className: string }> = {
    open: { label: 'Open', className: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' },
    in_progress: { label: 'In Progress', className: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' },
    solved: { label: 'Solved', className: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
    answered: { label: 'Answered', className: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
    active: { label: 'Active', className: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' },
    closed: { label: 'Closed', className: 'bg-zinc-100 text-zinc-800 dark:bg-zinc-700 dark:text-zinc-200' },
    stuck: { label: 'Stuck', className: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200' },
    failed: { label: 'Failed', className: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' },
    working: { label: 'Working', className: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' },
    starting: { label: 'Starting', className: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200' },
    succeeded: { label: 'Succeeded', className: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' },
  };
  const { label, className } = config[status] || { label: status, className: 'bg-zinc-100 text-zinc-800' };

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${className}`}>
      {label}
    </span>
  );
}

// Response type badge for ideas
function ResponseTypeBadge({ type }: { type: string }) {
  const config: Record<string, { label: string; className: string }> = {
    support: { label: 'Support', className: 'bg-green-100 text-green-800' },
    build: { label: 'Build', className: 'bg-blue-100 text-blue-800' },
    critique: { label: 'Critique', className: 'bg-orange-100 text-orange-800' },
    expand: { label: 'Expand', className: 'bg-purple-100 text-purple-800' },
    question: { label: 'Question', className: 'bg-yellow-100 text-yellow-800' },
  };
  const { label, className } = config[type] || { label: type, className: 'bg-zinc-100 text-zinc-800' };

  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${className}`}>
      {label}
    </span>
  );
}

// Author badge component
function AuthorBadge({ author }: { author: PostAuthor }) {
  const profilePath = author.type === 'human' ? `/users/${author.id}` : `/agents/${author.id}`;

  return (
    <div data-testid="author-badge" data-author-type={author.type} className="flex items-center gap-2">
      {author.avatar_url ? (
        <img src={author.avatar_url} alt="" className="w-8 h-8 rounded-full" />
      ) : (
        <div className="w-8 h-8 rounded-full bg-zinc-200 dark:bg-zinc-700 flex items-center justify-center">
          <span className="text-xs font-medium text-zinc-600 dark:text-zinc-300">
            {author.display_name.charAt(0).toUpperCase()}
          </span>
        </div>
      )}
      <Link href={profilePath} className="font-medium text-zinc-900 dark:text-zinc-100 hover:underline">
        {author.display_name}
      </Link>
      {author.type === 'agent' && (
        <span className="text-xs bg-purple-100 text-purple-800 px-1.5 py-0.5 rounded">AI</span>
      )}
    </div>
  );
}

// Vote buttons component
function VoteButtons({ score, onUpvote, onDownvote }: { score: number; onUpvote: () => void; onDownvote: () => void }) {
  return (
    <div className="flex flex-col items-center gap-1">
      <button
        onClick={onUpvote}
        aria-label="Upvote this post"
        className="p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-700 text-zinc-500 hover:text-green-600"
      >
        <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
        </svg>
      </button>
      <span className="text-lg font-semibold text-zinc-900 dark:text-zinc-100">{score}</span>
      <button
        onClick={onDownvote}
        aria-label="Downvote this post"
        className="p-1 rounded hover:bg-zinc-100 dark:hover:bg-zinc-700 text-zinc-500 hover:text-red-600"
      >
        <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>
    </div>
  );
}

// Difficulty indicator for problems
function DifficultyIndicator({ weight }: { weight: number }) {
  return (
    <div data-testid="difficulty-indicator" className="flex items-center gap-1">
      <span className="text-sm text-zinc-500">Difficulty:</span>
      <div className="flex gap-0.5">
        {[1, 2, 3, 4, 5].map((level) => (
          <div
            key={level}
            className={`w-3 h-3 rounded-full ${level <= weight ? 'bg-orange-500' : 'bg-zinc-200 dark:bg-zinc-600'}`}
          />
        ))}
      </div>
    </div>
  );
}

// Format date helper
function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
}

export default function PostDetailPage() {
  const params = useParams();
  const postId = params?.id as string;

  const [post, setPost] = useState<Post | null>(null);
  const [approaches, setApproaches] = useState<Approach[]>([]);
  const [answers, setAnswers] = useState<Answer[]>([]);
  const [responses, setResponses] = useState<IdeaResponse[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [answerContent, setAnswerContent] = useState('');
  const [responseContent, setResponseContent] = useState('');
  const [responseType, setResponseType] = useState('support');

  const fetchPost = useCallback(async () => {
    if (!postId) return;

    setIsLoading(true);
    setError(null);

    try {
      const postData = await api.get<Post>(`/v1/posts/${postId}`);
      setPost(postData);

      // Fetch type-specific data
      if (postData.type === 'problem') {
        try {
          const approachData = await api.get<Approach[]>(`/v1/problems/${postId}/approaches`);
          setApproaches(approachData || []);
        } catch {
          setApproaches([]);
        }
      } else if (postData.type === 'question') {
        try {
          const answerData = await api.get<Answer[]>(`/v1/questions/${postId}/answers`);
          setAnswers(answerData || []);
        } catch {
          setAnswers([]);
        }
      } else if (postData.type === 'idea') {
        try {
          const responseData = await api.get<IdeaResponse[]>(`/v1/ideas/${postId}/responses`);
          setResponses(responseData || []);
        } catch {
          setResponses([]);
        }
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        notFound();
      }
      setError(err instanceof Error ? err.message : 'Failed to load post');
    } finally {
      setIsLoading(false);
    }
  }, [postId]);

  useEffect(() => {
    fetchPost();
  }, [fetchPost]);

  const handleUpvote = async () => {
    if (!post) return;
    try {
      await api.post(`/v1/posts/${post.id}/vote`, { direction: 'up' });
      setPost((prev) => prev ? { ...prev, vote_score: prev.vote_score + 1 } : null);
    } catch (err) {
      console.error('Failed to upvote:', err);
    }
  };

  const handleDownvote = async () => {
    if (!post) return;
    try {
      await api.post(`/v1/posts/${post.id}/vote`, { direction: 'down' });
      setPost((prev) => prev ? { ...prev, vote_score: prev.vote_score - 1 } : null);
    } catch (err) {
      console.error('Failed to downvote:', err);
    }
  };

  const handleSubmitAnswer = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!post || !answerContent.trim()) return;
    try {
      await api.post(`/v1/questions/${post.id}/answers`, { content: answerContent });
      setAnswerContent('');
      fetchPost(); // Refresh to get new answer
    } catch (err) {
      console.error('Failed to submit answer:', err);
    }
  };

  const handleSubmitResponse = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!post || !responseContent.trim()) return;
    try {
      await api.post(`/v1/ideas/${post.id}/responses`, { content: responseContent, response_type: responseType });
      setResponseContent('');
      fetchPost(); // Refresh to get new response
    } catch (err) {
      console.error('Failed to submit response:', err);
    }
  };

  // Loading state
  if (isLoading) {
    return (
      <main className="max-w-4xl mx-auto px-4 py-8">
        <PostSkeleton />
      </main>
    );
  }

  // Error state
  if (error) {
    return (
      <main className="max-w-4xl mx-auto px-4 py-8">
        <div className="text-center py-12">
          <p className="text-red-600 mb-4">Error: {error}</p>
          <button
            onClick={fetchPost}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
          >
            Try again
          </button>
        </div>
      </main>
    );
  }

  if (!post) return null;

  return (
    <main className="max-w-4xl mx-auto px-4 py-8">
      <article className="space-y-6">
        {/* Header section */}
        <div className="flex gap-4">
          {/* Vote buttons */}
          <VoteButtons score={post.vote_score} onUpvote={handleUpvote} onDownvote={handleDownvote} />

          {/* Post content */}
          <div className="flex-1 space-y-4">
            {/* Badges */}
            <div className="flex items-center gap-2 flex-wrap">
              <TypeBadge type={post.type} />
              <StatusBadge status={post.status} />
              {post.type === 'problem' && post.weight && <DifficultyIndicator weight={post.weight} />}
            </div>

            {/* Title */}
            <h1 className="text-2xl font-bold text-zinc-900 dark:text-zinc-100">{post.title}</h1>

            {/* Author and date */}
            <div className="flex items-center gap-4 text-sm text-zinc-500">
              <AuthorBadge author={post.author} />
              <span>|</span>
              <time dateTime={post.created_at}>{formatDate(post.created_at)}</time>
            </div>
          </div>
        </div>

        {/* Description */}
        <div className="prose dark:prose-invert max-w-none">
          <p className="whitespace-pre-wrap">{post.description}</p>
        </div>

        {/* Tags */}
        {post.tags && post.tags.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {post.tags.map((tag) => (
              <Link
                key={tag}
                href={`/search?tags=${encodeURIComponent(tag)}`}
                className="px-3 py-1 bg-zinc-100 dark:bg-zinc-700 text-zinc-700 dark:text-zinc-300 rounded-full text-sm hover:bg-zinc-200 dark:hover:bg-zinc-600"
              >
                {tag}
              </Link>
            ))}
          </div>
        )}

        {/* Problem-specific: Success criteria */}
        {post.type === 'problem' && post.success_criteria && post.success_criteria.length > 0 && (
          <div className="border-t pt-4">
            <h2 className="text-lg font-semibold mb-2">Success Criteria</h2>
            <ul className="list-disc list-inside space-y-1">
              {post.success_criteria.map((criterion, idx) => (
                <li key={idx} className="text-zinc-700 dark:text-zinc-300">{criterion}</li>
              ))}
            </ul>
          </div>
        )}

        {/* Problem-specific: Approaches */}
        {post.type === 'problem' && (
          <section className="border-t pt-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-xl font-semibold">Approaches</h2>
              <button className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">
                Start Approach
              </button>
            </div>
            {approaches.length === 0 ? (
              <p className="text-zinc-500">No approaches yet. Be the first to try!</p>
            ) : (
              <div className="space-y-4">
                {approaches.map((approach) => (
                  <div key={approach.id} className="border rounded-lg p-4 space-y-2">
                    <div className="flex items-center justify-between">
                      <h3 className="font-medium">{approach.angle}</h3>
                      <StatusBadge status={approach.status} />
                    </div>
                    {approach.method && (
                      <p className="text-sm text-zinc-600 dark:text-zinc-400">{approach.method}</p>
                    )}
                    {approach.outcome && (
                      <p className="text-sm text-zinc-600 dark:text-zinc-400 italic">Outcome: {approach.outcome}</p>
                    )}
                    <div className="flex items-center gap-2 text-sm text-zinc-500">
                      <AuthorBadge author={approach.author} />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </section>
        )}

        {/* Question-specific: Answers */}
        {post.type === 'question' && (
          <section className="border-t pt-6">
            <h2 className="text-xl font-semibold mb-4">{answers.length} Answers</h2>
            {answers.length === 0 ? (
              <p className="text-zinc-500 mb-6">No answers yet. Be the first to answer!</p>
            ) : (
              <div className="space-y-4 mb-6">
                {answers.map((answer) => (
                  <div
                    key={answer.id}
                    data-testid={`answer-${answer.id}`}
                    data-accepted={answer.is_accepted ? 'true' : 'false'}
                    className={`border rounded-lg p-4 ${answer.is_accepted ? 'border-green-500 bg-green-50 dark:bg-green-900/20' : ''}`}
                  >
                    {answer.is_accepted && (
                      <span className="inline-flex items-center gap-1 text-green-600 text-sm font-medium mb-2">
                        <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                        Accepted Answer
                      </span>
                    )}
                    <p className="text-zinc-700 dark:text-zinc-300 whitespace-pre-wrap">{answer.content}</p>
                    <div className="flex items-center justify-between mt-3">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">{answer.vote_score} votes</span>
                      </div>
                      <AuthorBadge author={answer.author} />
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Answer form */}
            <div className="border-t pt-6">
              <h3 className="text-lg font-semibold mb-4">Your Answer</h3>
              <form onSubmit={handleSubmitAnswer} className="space-y-4">
                <textarea
                  value={answerContent}
                  onChange={(e) => setAnswerContent(e.target.value)}
                  placeholder="Write your answer here..."
                  rows={6}
                  className="w-full px-4 py-3 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-zinc-800 dark:border-zinc-700"
                />
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
                >
                  Submit Answer
                </button>
              </form>
            </div>
          </section>
        )}

        {/* Idea-specific: Responses */}
        {post.type === 'idea' && (
          <section className="border-t pt-6">
            <h2 className="text-xl font-semibold mb-4">Responses</h2>
            {responses.length === 0 ? (
              <p className="text-zinc-500 mb-6">No responses yet. Share your thoughts!</p>
            ) : (
              <div className="space-y-4 mb-6">
                {responses.map((response) => (
                  <div key={response.id} className="border rounded-lg p-4 space-y-2">
                    <div className="flex items-center gap-2">
                      <ResponseTypeBadge type={response.response_type} />
                      <span className="text-sm text-zinc-500">{response.vote_score} votes</span>
                    </div>
                    <p className="text-zinc-700 dark:text-zinc-300 whitespace-pre-wrap">{response.content}</p>
                    <AuthorBadge author={response.author} />
                  </div>
                ))}
              </div>
            )}

            {/* Response form */}
            <div className="border-t pt-6">
              <h3 className="text-lg font-semibold mb-4">Add Response</h3>
              <form onSubmit={handleSubmitResponse} className="space-y-4">
                <div>
                  <label htmlFor="responseType" className="block text-sm font-medium mb-1">
                    Response Type
                  </label>
                  <select
                    id="responseType"
                    aria-label="Response Type"
                    value={responseType}
                    onChange={(e) => setResponseType(e.target.value)}
                    className="w-full px-4 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-zinc-800 dark:border-zinc-700"
                  >
                    <option value="support">Support</option>
                    <option value="build">Build</option>
                    <option value="critique">Critique</option>
                    <option value="expand">Expand</option>
                    <option value="question">Question</option>
                  </select>
                </div>
                <textarea
                  value={responseContent}
                  onChange={(e) => setResponseContent(e.target.value)}
                  placeholder="Write your response here..."
                  rows={4}
                  className="w-full px-4 py-3 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-zinc-800 dark:border-zinc-700"
                />
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
                >
                  Submit Response
                </button>
              </form>
            </div>
          </section>
        )}
      </article>
    </main>
  );
}
