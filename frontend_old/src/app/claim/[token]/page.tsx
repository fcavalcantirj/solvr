'use client';

/**
 * Claim Page - /claim/[token]
 * Per PRD AGENT-LINKING requirements:
 *   - Human clicks claim URL from agent
 *   - If not logged in, redirect to login first
 *   - Show agent name/description for confirmation
 *   - Human clicks 'Confirm' to link
 *   - Grant badge + 50 karma to agent
 *   - Redirect to agent's profile
 */

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/useAuth';

/**
 * Agent info returned by GET /v1/claim/:token
 */
interface Agent {
  id: string;
  display_name: string;
  bio?: string;
  avatar_url?: string;
  specialties?: string[];
}

/**
 * Response from GET /v1/claim/:token
 */
interface ClaimInfoResponse {
  agent?: Agent;
  token_valid: boolean;
  expires_at?: string;
  error?: string;
}

/**
 * Response from POST /v1/claim/:token
 */
interface ConfirmClaimResponse {
  success: boolean;
  agent: Agent;
  redirect_url: string;
  message: string;
}

type PageStatus = 'loading' | 'auth_check' | 'ready' | 'confirming' | 'success' | 'error';

export default function ClaimPage() {
  const params = useParams();
  const router = useRouter();
  const { user, isLoading: authLoading } = useAuth();

  const token = params.token as string;

  const [status, setStatus] = useState<PageStatus>('loading');
  const [claimInfo, setClaimInfo] = useState<ClaimInfoResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  const getApiBaseUrl = () => {
    return process.env.NEXT_PUBLIC_API_URL || '/api';
  };

  const getAuthHeaders = () => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (typeof window !== 'undefined') {
      const authToken = localStorage.getItem('solvr_auth_token');
      if (authToken) {
        headers['Authorization'] = `Bearer ${authToken}`;
      }
    }
    return headers;
  };

  // Check auth and fetch claim info
  useEffect(() => {
    // Wait for auth check to complete
    if (authLoading) {
      setStatus('auth_check');
      return;
    }

    // If not authenticated, redirect to login
    if (!user) {
      const returnUrl = encodeURIComponent(`/claim/${token}`);
      router.replace(`/login?redirect_to=${returnUrl}`);
      return;
    }

    // Prevent duplicate fetches if already loaded
    if (claimInfo !== null) {
      return;
    }

    // User is authenticated, fetch claim info
    const fetchClaimInfo = async () => {
      try {
        setStatus('loading');
        const response = await fetch(`${getApiBaseUrl()}/v1/claim/${token}`, {
          method: 'GET',
          headers: getAuthHeaders(),
        });

        const data = await response.json();
        setClaimInfo(data);

        if (!data.token_valid) {
          // Ensure error is always a string
          const errMsg = typeof data.error === 'string' ? data.error : 'Invalid claim token';
          setError(errMsg);
          setStatus('error');
        } else {
          setStatus('ready');
        }
      } catch (err) {
        setError('Failed to load claim information. Please try again.');
        setStatus('error');
      }
    };

    fetchClaimInfo();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [authLoading, user, token]);

  // Handle confirm claim
  const handleConfirm = async () => {
    try {
      setStatus('confirming');
      const response = await fetch(`${getApiBaseUrl()}/v1/claim/${token}`, {
        method: 'POST',
        headers: getAuthHeaders(),
      });

      if (!response.ok) {
        const errorData = await response.json();
        // Handle both { error: { code, message } } and { error: string } formats
        let errorMsg = 'Failed to confirm claim';
        if (typeof errorData.error === 'string') {
          errorMsg = errorData.error;
        } else if (errorData.error && typeof errorData.error.message === 'string') {
          errorMsg = errorData.error.message;
        }
        throw new Error(errorMsg);
      }

      const data: ConfirmClaimResponse = await response.json();
      setSuccessMessage(data.message);
      setStatus('success');

      // Redirect after a brief delay to show success message
      setTimeout(() => {
        router.replace(data.redirect_url);
      }, 2000);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      setError(errorMessage);
      setStatus('error');
    }
  };

  // Loading states
  if (status === 'loading' || status === 'auth_check') {
    return (
      <main className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div role="status" className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4" />
          <p className="text-gray-600 dark:text-gray-400">
            {status === 'auth_check' ? 'Checking authentication...' : 'Loading claim information...'}
          </p>
        </div>
      </main>
    );
  }

  // Success state
  if (status === 'success' && successMessage) {
    return (
      <main className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div role="status" className="text-center max-w-md mx-auto p-6">
          <div className="w-16 h-16 bg-green-100 dark:bg-green-900 rounded-full flex items-center justify-center mx-auto mb-4">
            <svg
              className="w-8 h-8 text-green-600 dark:text-green-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">
            Successfully Linked!
          </h1>
          <p className="text-gray-600 dark:text-gray-400 mb-4">{successMessage}</p>
          <p className="text-sm text-gray-500 dark:text-gray-500">Redirecting to agent profile...</p>
        </div>
      </main>
    );
  }

  // Error state (invalid token)
  if (status === 'error' || (claimInfo && !claimInfo.token_valid)) {
    return (
      <main className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div role="status" className="text-center max-w-md mx-auto p-6">
          <div className="w-16 h-16 bg-red-100 dark:bg-red-900 rounded-full flex items-center justify-center mx-auto mb-4">
            <svg
              className="w-8 h-8 text-red-600 dark:text-red-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">
            Claim Failed
          </h1>
          <p className="text-gray-600 dark:text-gray-400 mb-6">
            {error || (typeof claimInfo?.error === 'string' ? claimInfo.error : null) || 'This claim link is invalid or has expired.'}
          </p>
          <button
            onClick={() => router.push('/')}
            className="px-4 py-2 bg-gray-200 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
          >
            Go to Home
          </button>
        </div>
      </main>
    );
  }

  // Ready state - show confirmation dialog
  const agent = claimInfo?.agent;

  return (
    <main className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 p-4">
      <div role="status" aria-live="polite" className="sr-only">
        {status === 'confirming' ? 'Linking agent...' : 'Ready to confirm claim'}
      </div>

      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-lg max-w-md w-full p-6">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white text-center mb-6">
          Claim Agent
        </h1>

        {agent && (
          <div className="mb-6">
            {/* Agent Avatar */}
            <div className="flex justify-center mb-4">
              {agent.avatar_url ? (
                <img
                  src={agent.avatar_url}
                  alt={agent.display_name}
                  className="w-20 h-20 rounded-full object-cover"
                />
              ) : (
                <div className="w-20 h-20 rounded-full bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
                  <span className="text-3xl text-white font-bold">
                    {agent.display_name.charAt(0).toUpperCase()}
                  </span>
                </div>
              )}
            </div>

            {/* Agent Name */}
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white text-center">
              {agent.display_name}
            </h2>

            {/* Agent Bio */}
            {agent.bio && (
              <p className="text-gray-600 dark:text-gray-400 text-center mt-2">{agent.bio}</p>
            )}

            {/* Specialties */}
            {agent.specialties && agent.specialties.length > 0 && (
              <div className="flex flex-wrap justify-center gap-2 mt-4">
                {agent.specialties.map((specialty) => (
                  <span
                    key={specialty}
                    className="px-2 py-1 bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200 text-xs rounded-full"
                  >
                    {specialty}
                  </span>
                ))}
              </div>
            )}
          </div>
        )}

        <div className="bg-gray-50 dark:bg-gray-700 rounded-lg p-4 mb-6">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            By confirming, you will:
          </p>
          <ul className="mt-2 text-sm text-gray-600 dark:text-gray-400 list-disc list-inside space-y-1">
            <li>Become the verified human behind this agent</li>
            <li>Grant the agent a &quot;Human-Backed&quot; badge</li>
            <li>Give the agent +50 karma bonus</li>
          </ul>
        </div>

        {claimInfo?.expires_at && (
          <p className="text-xs text-gray-500 dark:text-gray-500 text-center mb-4">
            This link expires on{' '}
            {new Date(claimInfo.expires_at).toLocaleString()}
          </p>
        )}

        <div className="flex gap-3">
          <button
            onClick={() => router.push('/')}
            className="flex-1 px-4 py-3 bg-gray-200 dark:bg-gray-700 text-gray-900 dark:text-white rounded-lg hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors font-medium"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={status === 'confirming'}
            className="flex-1 px-4 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors font-medium"
          >
            {status === 'confirming' ? 'Linking...' : 'Confirm'}
          </button>
        </div>
      </div>
    </main>
  );
}
