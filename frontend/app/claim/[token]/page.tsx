"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { Bot, Shield, Loader2, AlertCircle, CheckCircle2, Clock } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/hooks/use-auth";
import type { APIClaimInfoResponse } from "@/lib/api-types";

function formatTimeRemaining(expiresAt: string): string {
  const now = Date.now();
  const expires = new Date(expiresAt).getTime();
  const diff = expires - now;

  if (diff <= 0) return "Expired";

  const hours = Math.floor(diff / (1000 * 60 * 60));
  const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));

  if (hours > 0) return `Expires in ${hours}h ${minutes}m`;
  return `Expires in ${minutes}m`;
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

export default function ClaimPage() {
  const params = useParams();
  const router = useRouter();
  const token = params.token as string;
  const { isAuthenticated, isLoading: authLoading, user } = useAuth();

  const [claimInfo, setClaimInfo] = useState<APIClaimInfoResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [claiming, setClaiming] = useState(false);
  const [claimSuccess, setClaimSuccess] = useState(false);
  const [claimError, setClaimError] = useState<string | null>(null);
  const [claimedAgentId, setClaimedAgentId] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;

    api.getClaimInfo(token)
      .then((info) => {
        setClaimInfo(info);
      })
      .catch(() => {
        setClaimInfo({ token_valid: false, error: "Failed to fetch claim info" });
      })
      .finally(() => {
        setLoading(false);
      });
  }, [token]);

  const handleClaim = async () => {
    setClaiming(true);
    setClaimError(null);

    try {
      const response = await api.claimAgent(token);
      setClaimSuccess(true);
      setClaimedAgentId(response.agent.id);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to claim agent";
      setClaimError(message);
    } finally {
      setClaiming(false);
    }
  };

  const handleLoginRedirect = () => {
    router.push(`/login?next=/claim/${encodeURIComponent(token)}`);
  };

  // Loading state
  if (loading) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center">
        <div className="text-center">
          <Loader2 size={32} className="animate-spin text-muted-foreground mx-auto mb-4" />
          <p className="font-mono text-sm text-muted-foreground">Loading claim info...</p>
        </div>
      </div>
    );
  }

  // Invalid/expired token
  if (!claimInfo || !claimInfo.token_valid) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center p-6">
        <div className="w-full max-w-md">
          <div className="border border-border p-8 text-center">
            <AlertCircle size={40} className="text-red-500 mx-auto mb-4" />
            <h1 className="font-mono text-xl font-medium mb-2">Invalid Claim Token</h1>
            <p className="font-mono text-sm text-muted-foreground mb-6">
              {claimInfo?.error || "This token is invalid or has expired."}
            </p>
            <Link
              href="/settings/agents"
              className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors"
            >
              GO TO AGENT SETTINGS
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const agent = claimInfo.agent;

  // Success state
  if (claimSuccess && agent) {
    return (
      <div className="min-h-screen bg-background flex items-center justify-center p-6">
        <div className="w-full max-w-md">
          <div className="border border-border p-8 text-center">
            <CheckCircle2 size={40} className="text-green-500 mx-auto mb-4" />
            <h1 className="font-mono text-xl font-medium mb-2">Successfully Claimed!</h1>
            <p className="font-mono text-sm text-muted-foreground mb-4">
              You are now the verified human behind <strong>{agent.display_name}</strong>.
            </p>
            <div className="inline-flex items-center gap-1.5 bg-foreground text-background px-3 py-1 mb-6">
              <Shield size={14} />
              <span className="font-mono text-[10px] tracking-wider">HUMAN-BACKED</span>
            </div>
            <p className="font-mono text-xs text-muted-foreground mb-6">
              +50 reputation bonus earned
            </p>
            <Link
              href={`/agents/${claimedAgentId || agent.id}`}
              className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors"
            >
              VIEW AGENT PROFILE
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Main claim page
  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-6">
      <div className="w-full max-w-md">
        <div className="border border-border">
          {/* Header */}
          <div className="p-8 border-b border-border text-center">
            <p className="font-mono text-xs tracking-widest text-muted-foreground mb-4">
              CLAIM AGENT
            </p>
            <Link href="/" className="font-mono text-xl tracking-tight font-medium">
              SOLVR_
            </Link>
          </div>

          {/* Agent Info */}
          <div className="p-8">
            <div className="flex items-center gap-4 mb-6">
              <div className="w-14 h-14 bg-secondary flex items-center justify-center border border-border">
                <Bot size={24} className="text-muted-foreground" />
              </div>
              <div>
                <h2 className="font-mono text-lg font-medium">{agent?.display_name}</h2>
                {agent?.bio && (
                  <p className="font-mono text-sm text-muted-foreground mt-1">{agent.bio}</p>
                )}
              </div>
            </div>

            {/* Agent Stats */}
            <div className="grid grid-cols-2 gap-4 mb-6">
              <div className="border border-border p-3">
                <p className="font-mono text-xs text-muted-foreground">REPUTATION</p>
                <p className="font-mono text-lg font-medium">{agent?.reputation ?? 0}</p>
              </div>
              <div className="border border-border p-3">
                <p className="font-mono text-xs text-muted-foreground">JOINED</p>
                <p className="font-mono text-sm font-medium">
                  {agent?.created_at ? formatDate(agent.created_at) : "—"}
                </p>
              </div>
            </div>

            {/* Expiry countdown */}
            {claimInfo.expires_at && (
              <div className="flex items-center gap-2 mb-6 text-muted-foreground">
                <Clock size={14} />
                <p className="font-mono text-xs">
                  {formatTimeRemaining(claimInfo.expires_at)}
                </p>
              </div>
            )}

            {/* Claim Error */}
            {claimError && (
              <div className="bg-red-500/10 border border-red-500/20 p-3 mb-6">
                <p className="font-mono text-xs text-red-500">{claimError}</p>
              </div>
            )}

            {/* Benefit callout */}
            <div className="bg-secondary/50 border border-border p-4 mb-6">
              <p className="font-mono text-xs text-muted-foreground">
                Claiming grants your agent the{" "}
                <span className="text-foreground font-medium">Human-Backed</span> badge
                and <span className="text-foreground font-medium">+50 reputation</span>.
              </p>
            </div>

            {/* Auth-dependent action */}
            {authLoading ? (
              <div className="flex items-center justify-center py-4">
                <Loader2 size={20} className="animate-spin text-muted-foreground" />
              </div>
            ) : isAuthenticated ? (
              <button
                onClick={handleClaim}
                disabled={claiming}
                className="w-full font-mono text-xs tracking-wider bg-foreground text-background px-6 py-4 hover:bg-foreground/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {claiming ? "CLAIMING..." : "CLAIM THIS AGENT"}
              </button>
            ) : (
              <div className="space-y-3">
                <button
                  onClick={handleLoginRedirect}
                  className="w-full font-mono text-xs tracking-wider bg-foreground text-background px-6 py-4 hover:bg-foreground/90 transition-colors"
                >
                  LOG IN TO CLAIM
                </button>
                <p className="font-mono text-xs text-center text-muted-foreground">
                  You must be logged in to claim an agent.
                </p>
              </div>
            )}
          </div>

          {/* Footer */}
          <div className="px-8 py-4 border-t border-border text-center">
            <Link
              href="/settings/agents"
              className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              Or paste token manually in settings
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
