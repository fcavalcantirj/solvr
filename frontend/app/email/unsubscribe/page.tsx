"use client";

import { useSearchParams } from "next/navigation";
import { useState, Suspense } from "react";
import { CheckCircle2, XCircle, Loader2 } from "lucide-react";

// The page an email's footer links (SPEC.md 16.6). The API puts the address, its token and the
// address masked for display in the link. Opening the page changes nothing: one button POSTs the
// RFC 8058 one-click body to the same URL a mail client uses.
function UnsubscribeContent() {
  const searchParams = useSearchParams();
  const email = searchParams.get("email");
  const token = searchParams.get("token");
  const masked = searchParams.get("masked");
  const [status, setStatus] = useState<"ready" | "working" | "success" | "error">(
    email && token ? "ready" : "error",
  );
  const [message, setMessage] = useState(email && token ? "" : "Invalid unsubscribe link. Missing email or token.");

  const unsubscribe = () => {
    if (!email || !token) return;
    setStatus("working");
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";
    fetch(`${apiUrl}/v1/email/unsubscribe?email=${encodeURIComponent(email)}&token=${encodeURIComponent(token)}`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: "List-Unsubscribe=One-Click",
    })
      .then(async (res) => {
        const data = await res.json();
        if (res.ok) {
          setStatus("success");
          setMessage(data.message || "You have been unsubscribed.");
        } else {
          setStatus("error");
          setMessage(data.error?.message || "Failed to unsubscribe. The link may be invalid.");
        }
      })
      .catch(() => {
        setStatus("error");
        setMessage("Network error. Please try again later.");
      });
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="max-w-md w-full mx-4 text-center">
        <h1 className="font-mono text-lg tracking-tight font-medium mb-2">SOLVR_</h1>

        {(status === "ready" || status === "working") && (
          <div className="mt-8 border border-border p-8">
            <h2 className="mb-2 text-2xl font-light tracking-[-0.02em]">Unsubscribe</h2>
            <p className="text-sm text-muted-foreground">
              Stop Solvr&apos;s broadcast emails{masked ? ` to ${masked}` : ""}?
            </p>
            <button
              type="button"
              onClick={unsubscribe}
              disabled={status === "working"}
              className="mt-6 w-full font-mono text-[11px] uppercase tracking-[0.18em] py-2.5 border border-border hover:border-foreground transition-colors flex items-center justify-center gap-2 disabled:opacity-60"
            >
              {status === "working" && <Loader2 className="w-3 h-3 animate-spin" />}
              Unsubscribe
            </button>
          </div>
        )}

        {status === "success" && (
          <div className="mt-8 border border-border p-8">
            <CheckCircle2 strokeWidth={1} className="w-12 h-12 mx-auto text-green-700 dark:text-green-400 mb-4" />
            <h2 className="mb-2 text-2xl font-light tracking-[-0.02em]">Unsubscribed</h2>
            <p className="text-sm text-muted-foreground">{message}</p>
            <p className="text-xs text-muted-foreground mt-4">
              You will no longer receive broadcast emails from Solvr.
            </p>
          </div>
        )}

        {status === "error" && (
          <div className="mt-8 border border-border p-8">
            <XCircle className="w-12 h-12 mx-auto text-destructive mb-4" />
            <h2 className="mb-2 text-2xl font-light tracking-[-0.02em]">Error</h2>
            <p className="text-sm text-muted-foreground">{message}</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default function UnsubscribePage() {
  return (
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center bg-background">
        <Loader2 className="w-8 h-8 animate-spin text-muted-foreground" />
      </div>
    }>
      <UnsubscribeContent />
    </Suspense>
  );
}
