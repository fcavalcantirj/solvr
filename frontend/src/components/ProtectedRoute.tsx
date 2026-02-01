'use client';

/**
 * ProtectedRoute Component
 * Per PRD requirements:
 *   - Redirect to /login if not authenticated
 *   - Show loading state while checking auth
 *   - Render children when authenticated
 */

import { useEffect, ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '../hooks/useAuth';

export interface ProtectedRouteProps {
  children: ReactNode;
  redirectTo?: string;
  fallback?: ReactNode;
}

/**
 * Default loading component
 */
function DefaultLoading() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="text-center">
        <div className="relative w-12 h-12 mx-auto mb-4">
          <div className="absolute inset-0 border-4 border-gray-200 rounded-full" />
          <div
            className="absolute inset-0 border-4 border-t-blue-500 rounded-full animate-spin"
            aria-hidden="true"
          />
        </div>
        <p role="status" className="text-gray-600">
          Loading...
        </p>
      </div>
    </div>
  );
}

/**
 * ProtectedRoute component
 * Wraps children that require authentication
 */
export function ProtectedRoute({
  children,
  redirectTo = '/login',
  fallback,
}: ProtectedRouteProps) {
  const router = useRouter();
  const { user, isLoading } = useAuth();

  useEffect(() => {
    // Don't redirect while still loading
    if (isLoading) return;

    // Redirect if not authenticated
    if (!user) {
      router.push(redirectTo);
    }
  }, [user, isLoading, router, redirectTo]);

  // Show loading state
  if (isLoading) {
    return fallback ?? <DefaultLoading />;
  }

  // Not authenticated - render nothing (redirect will happen)
  if (!user) {
    return null;
  }

  // Authenticated - render children
  return <>{children}</>;
}

export default ProtectedRoute;
