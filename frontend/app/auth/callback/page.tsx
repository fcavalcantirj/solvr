"use client";

export const dynamic = 'force-dynamic';

import { Suspense } from "react";
import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";

function AuthCallbackContent() {
  const searchParams = useSearchParams();
  const { setToken } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [isProcessing, setIsProcessing] = useState(true);
  // The login code works once. Taking it out of the address bar makes the search params
  // change and re-runs the effect, and React StrictMode runs effects twice in development;
  // neither may try the exchange again.
  const startedRef = useRef(false);

  useEffect(() => {
    if (startedRef.current) return;
    startedRef.current = true;

    const handleCallback = async () => {
      const code = searchParams.get("code");
      const errorParam = searchParams.get("error");

      if (errorParam) {
        setError(errorParam);
        setIsProcessing(false);
        return;
      }

      if (!code) {
        setError("No login code received");
        setIsProcessing(false);
        return;
      }

      // The redirect carries a one-time login code, never a token. Take it out of the
      // address bar (history, analytics page views) before doing anything else.
      window.history.replaceState(null, "", window.location.pathname);

      try {
        const apiBase = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";
        const exchange = await fetch(`${apiBase}/v1/auth/oauth/exchange`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ login_code: code }),
        });
        const payload = await exchange.json().catch(() => null);
        const token: string | undefined = payload?.data?.access_token;
        if (!exchange.ok || !token) {
          setError(payload?.error?.message || "Failed to authenticate. Please try again.");
          setIsProcessing(false);
          return;
        }

        // Store token and fetch user info
        await setToken(token);

        // Claim referral if one was stored before OAuth redirect
        const refCode = localStorage.getItem("solvr_referral_code");
        if (refCode) {
          localStorage.removeItem("solvr_referral_code");
          try {
            await fetch(`${apiBase}/v1/auth/claim-referral`, {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "Authorization": `Bearer ${token}`,
              },
              body: JSON.stringify({ ref: refCode }),
            });
          } catch {
            // Silently ignore referral claim failures
          }
        }

        // Get return URL from localStorage or default to the posts collection
        const returnUrl = localStorage.getItem("auth_return_url") || "/posts";
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
          <div className="w-16 h-16 mx-auto mb-6 border border-destructive flex items-center justify-center">
            <span className="text-2xl">!</span>
          </div>
          <h1 className="text-[2rem] font-light leading-[1.1] tracking-[-0.03em] mb-4">
            AUTHENTICATION_FAILED
          </h1>
          <p className="text-muted-foreground mb-8">{error}</p>
          <a
            href="/login"
            className="border border-foreground inline-block font-mono text-[11px] uppercase tracking-[0.18em] bg-foreground text-background px-8 py-3 hover:bg-background hover:text-foreground transition-colors focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
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
        <h1 className="text-[2rem] font-light leading-[1.1] tracking-[-0.03em] mb-3">
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
        <h1 className="text-[2rem] font-light leading-[1.1] tracking-[-0.03em] mb-3">
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
