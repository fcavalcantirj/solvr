"use client";

export const dynamic = 'force-dynamic';

import { Suspense } from "react";
import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";

function AuthCallbackContent() {
  const searchParams = useSearchParams();
  const { setToken } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(true);

  useEffect(() => {
    const handleCallback = async () => {
      const token = searchParams.get("token");
      const errorParam = searchParams.get("error");

      if (errorParam) {
        setError(errorParam);
        setIsProcessing(false);
        return;
      }

      if (!token) {
        setError("No authentication token received");
        setIsProcessing(false);
        return;
      }

      try {
        // Store token and fetch user info
        await setToken(token);

        // Get return URL from localStorage or default to /feed
        const returnUrl = localStorage.getItem("auth_return_url") || "/feed";
        localStorage.removeItem("auth_return_url");

        // Hard reload to ensure all components refresh with authenticated state
        window.location.href = returnUrl;
      } catch (err) {
        setError("Failed to authenticate. Please try again.");
        setIsProcessing(false);
      }
    };

    handleCallback();
  }, [searchParams, setToken]);

  if (error) {
    return (
      <main className="min-h-screen bg-background flex items-center justify-center px-6">
        <div className="max-w-md w-full text-center">
          <div className="w-16 h-16 mx-auto mb-6 bg-red-500/10 border border-red-500/20 flex items-center justify-center">
            <span className="text-2xl">!</span>
          </div>
          <h1 className="font-mono text-xl tracking-tight mb-4">
            AUTHENTICATION_FAILED
          </h1>
          <p className="text-muted-foreground mb-8">{error}</p>
          <a
            href="/login"
            className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-8 py-3 hover:bg-foreground/90 transition-colors"
          >
            TRY AGAIN
          </a>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-background flex items-center justify-center px-6">
      <div className="max-w-md w-full text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <div className="w-6 h-6 border-2 border-muted-foreground/30 border-t-foreground rounded-full animate-spin" />
        </div>
        <h1 className="font-mono text-xl tracking-tight mb-2">
          AUTHENTICATING...
        </h1>
        <p className="text-muted-foreground text-sm">
          Please wait while we complete your login
        </p>
      </div>
    </main>
  );
}

function LoadingFallback() {
  return (
    <main className="min-h-screen bg-background flex items-center justify-center px-6">
      <div className="max-w-md w-full text-center">
        <div className="w-16 h-16 mx-auto mb-6 border border-border flex items-center justify-center">
          <div className="w-6 h-6 border-2 border-muted-foreground/30 border-t-foreground rounded-full animate-spin" />
        </div>
        <h1 className="font-mono text-xl tracking-tight mb-2">
          LOADING...
        </h1>
      </div>
    </main>
  );
}

export default function AuthCallbackPage() {
  return (
    <Suspense fallback={<LoadingFallback />}>
      <AuthCallbackContent />
    </Suspense>
  );
}
