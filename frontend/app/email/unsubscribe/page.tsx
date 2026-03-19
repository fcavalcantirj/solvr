"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useState, Suspense } from "react";
import { CheckCircle2, XCircle, Loader2 } from "lucide-react";

function UnsubscribeContent() {
  const searchParams = useSearchParams();
  const [status, setStatus] = useState<"loading" | "success" | "error">("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    const email = searchParams.get("email");
    const token = searchParams.get("token");

    if (!email || !token) {
      setStatus("error");
      setMessage("Invalid unsubscribe link. Missing email or token.");
      return;
    }

    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "https://api.solvr.dev";
    fetch(`${apiUrl}/v1/email/unsubscribe?email=${encodeURIComponent(email)}&token=${encodeURIComponent(token)}`)
      .then(async (res) => {
        const data = await res.json();
        if (res.ok) {
          setStatus("success");
          setMessage(data.message || "You have been unsubscribed.");
        } else {
          setStatus("error");
          setMessage(data.message || "Failed to unsubscribe. The link may be invalid.");
        }
      })
      .catch(() => {
        setStatus("error");
        setMessage("Network error. Please try again later.");
      });
  }, [searchParams]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="max-w-md w-full mx-4 text-center">
        <h1 className="font-mono text-2xl tracking-tight mb-2">SOLVR_</h1>

        {status === "loading" && (
          <div className="mt-8">
            <Loader2 className="w-8 h-8 animate-spin mx-auto text-muted-foreground" />
            <p className="font-mono text-sm text-muted-foreground mt-4">Processing...</p>
          </div>
        )}

        {status === "success" && (
          <div className="mt-8 border border-border p-8">
            <CheckCircle2 className="w-12 h-12 mx-auto text-green-500 mb-4" />
            <h2 className="font-mono text-lg mb-2">Unsubscribed</h2>
            <p className="text-sm text-muted-foreground">{message}</p>
            <p className="text-xs text-muted-foreground mt-4">
              You will no longer receive broadcast emails from Solvr.
            </p>
          </div>
        )}

        {status === "error" && (
          <div className="mt-8 border border-border p-8">
            <XCircle className="w-12 h-12 mx-auto text-destructive mb-4" />
            <h2 className="font-mono text-lg mb-2">Error</h2>
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
