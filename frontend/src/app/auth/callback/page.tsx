'use client';

/**
 * Auth Callback Page
 * Per SPEC.md Part 5.2 Authentication:
 *   - Parse tokens from URL
 *   - Store in localStorage
 *   - Redirect to dashboard (or redirect_to param)
 */

import { useEffect, useState, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

const AUTH_TOKEN_KEY = 'solvr_auth_token';

/**
 * Validate and sanitize redirect URL to prevent open redirect attacks
 * Only allows internal paths starting with /
 */
function sanitizeRedirectUrl(url: string | null): string {
  const defaultPath = '/dashboard';

  if (!url) return defaultPath;

  // Trim whitespace
  const trimmed = url.trim();

  // Must start with exactly one / and not be //
  if (!trimmed.startsWith('/') || trimmed.startsWith('//')) {
    return defaultPath;
  }

  // Must not contain protocol indicators
  if (trimmed.includes(':') || trimmed.includes('//')) {
    return defaultPath;
  }

  return trimmed;
}

function CallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [status, setStatus] = useState<'processing' | 'success' | 'error'>(
    'processing'
  );

  useEffect(() => {
    // Process auth callback asynchronously to avoid cascading renders
    const processAuth = async () => {
      const token = searchParams.get('token');
      const error = searchParams.get('error');
      const redirectTo = searchParams.get('redirect_to');

      // Small delay to ensure state updates happen outside the initial render
      await new Promise((resolve) => setTimeout(resolve, 0));

      // Handle error from OAuth provider
      if (error) {
        setStatus('error');
        // Use setTimeout to defer navigation
        setTimeout(() => {
          router.replace(`/login?error=${encodeURIComponent(error)}`);
        }, 100);
        return;
      }

      // Handle missing token
      if (!token) {
        setStatus('error');
        setTimeout(() => {
          router.replace('/login?error=missing_token');
        }, 100);
        return;
      }

      // Store token in localStorage
      try {
        localStorage.setItem(AUTH_TOKEN_KEY, token);
        setStatus('success');

        // Redirect to intended destination (sanitized)
        const destination = sanitizeRedirectUrl(redirectTo);
        setTimeout(() => {
          router.replace(destination);
        }, 100);
      } catch (err) {
        console.error('Failed to store auth token:', err);
        setStatus('error');
        setTimeout(() => {
          router.replace('/login?error=storage_error');
        }, 100);
      }
    };

    processAuth();
  }, [router, searchParams]);

  return (
    <div className="flex flex-col items-center gap-4">
      {/* Animated spinner */}
      <div className="relative w-12 h-12">
        <div className="absolute inset-0 border-4 border-gray-200 rounded-full" />
        <div
          className="absolute inset-0 border-4 border-t-blue-500 rounded-full animate-spin"
          aria-hidden="true"
        />
      </div>

      {/* Status message */}
      <p
        role="status"
        className="text-lg text-gray-600"
        aria-live="polite"
      >
        {status === 'processing' && 'Completing sign in...'}
        {status === 'success' && 'Success! Redirecting...'}
        {status === 'error' && 'Something went wrong. Redirecting...'}
      </p>
    </div>
  );
}

function CallbackLoading() {
  return (
    <div className="flex flex-col items-center gap-4">
      <div className="relative w-12 h-12">
        <div className="absolute inset-0 border-4 border-gray-200 rounded-full" />
        <div
          className="absolute inset-0 border-4 border-t-blue-500 rounded-full animate-spin"
          aria-hidden="true"
        />
      </div>
      <p role="status" className="text-lg text-gray-600">
        Loading...
      </p>
    </div>
  );
}

export default function CallbackPage() {
  return (
    <main className="min-h-screen flex items-center justify-center bg-gray-50 px-4">
      <div className="text-center">
        {/* Logo */}
        <div
          className="mb-8 text-3xl font-bold"
          style={{ color: 'var(--color-primary)' }}
        >
          Solvr
        </div>

        {/* Content with Suspense for useSearchParams */}
        <Suspense fallback={<CallbackLoading />}>
          <CallbackContent />
        </Suspense>
      </div>
    </main>
  );
}
