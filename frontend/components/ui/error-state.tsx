"use client";

import { AlertCircle, FileQuestion, ServerCrash, RefreshCw } from "lucide-react";
import { isNotFoundError, isServerError } from "@/lib/api-error";

interface ErrorStateProps {
  error: unknown;
  onRetry?: () => void;
  resourceName?: string;
}

export function ErrorState({ error, onRetry, resourceName = "resource" }: ErrorStateProps) {
  // Handle string error messages that indicate not found
  if (typeof error === 'string' && error.toLowerCase().includes('not found')) {
    return (
      <div className="py-20 text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <FileQuestion className="w-8 h-8 text-muted-foreground" />
        </div>
        <h2 className="font-mono text-lg mb-2">Not Found</h2>
        <p className="text-muted-foreground font-mono text-sm">
          The {resourceName} you're looking for doesn't exist or has been removed.
        </p>
      </div>
    );
  }

  if (isNotFoundError(error)) {
    return (
      <div className="py-20 text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <FileQuestion className="w-8 h-8 text-muted-foreground" />
        </div>
        <h2 className="font-mono text-lg mb-2">Not Found</h2>
        <p className="text-muted-foreground font-mono text-sm">
          The {resourceName} you're looking for doesn't exist or has been removed.
        </p>
      </div>
    );
  }

  if (isServerError(error)) {
    return (
      <div className="py-20 text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <ServerCrash className="w-8 h-8 text-muted-foreground" />
        </div>
        <h2 className="font-mono text-lg mb-2">Server Error</h2>
        <p className="text-muted-foreground font-mono text-sm mb-6">
          Something went wrong on our end. Please try again later.
        </p>
        {onRetry && (
          <button
            onClick={onRetry}
            className="inline-flex items-center gap-2 px-5 py-2.5 bg-foreground text-background font-mono text-xs tracking-wider hover:bg-foreground/90 transition-colors"
          >
            <RefreshCw className="w-3 h-3" />
            TRY AGAIN
          </button>
        )}
      </div>
    );
  }

  // Generic error
  const message = error instanceof Error ? error.message : "An unexpected error occurred";

  return (
    <div className="py-20 text-center">
      <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
        <AlertCircle className="w-8 h-8 text-muted-foreground" />
      </div>
      <h2 className="font-mono text-lg mb-2">Error</h2>
      <p className="text-muted-foreground font-mono text-sm mb-6">
        {message}
      </p>
      {onRetry && (
        <button
          onClick={onRetry}
          className="inline-flex items-center gap-2 px-5 py-2.5 bg-foreground text-background font-mono text-xs tracking-wider hover:bg-foreground/90 transition-colors"
        >
          <RefreshCw className="w-3 h-3" />
          TRY AGAIN
        </button>
      )}
    </div>
  );
}
