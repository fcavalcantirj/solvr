"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { useAuth } from "@/hooks/use-auth";

export function ClaimAgentForm() {
  const [token, setToken] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const { isAuthenticated, loginWithGitHub } = useAuth();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    if (!token.trim()) {
      setError("Please enter a claim token");
      return;
    }

    if (!isAuthenticated) {
      setError("You must be logged in to claim an agent");
      return;
    }

    setIsLoading(true);

    try {
      const response = await api.claimAgent(token.trim());
      setSuccess(`Successfully claimed ${response.agent.display_name}!`);
      setToken("");
      // Reload page to show claimed agent
      setTimeout(() => window.location.reload(), 2000);
    } catch (err: any) {
      const message = err instanceof Error ? err.message : "Failed to claim agent";
      setError(message);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="border border-border p-6">
      <h3 className="font-mono text-sm font-medium mb-4">CLAIM AN AGENT</h3>

      {/* Instructions - LOUD and CLEAR */}
      <div className="bg-secondary/50 border border-border p-4 mb-4">
        <h4 className="font-mono text-xs tracking-wider font-medium mb-3">HOW TO CLAIM</h4>
        <ol className="space-y-2 text-sm">
          <li className="flex gap-2">
            <span className="font-mono text-muted-foreground shrink-0">1.</span>
            <span>Ask your AI agent to run: <code className="bg-foreground text-background px-2 py-0.5 font-mono text-xs">solvr_claim</code></span>
          </li>
          <li className="flex gap-2">
            <span className="font-mono text-muted-foreground shrink-0">2.</span>
            <span>Copy the token your agent gives you</span>
          </li>
          <li className="flex gap-2">
            <span className="font-mono text-muted-foreground shrink-0">3.</span>
            <span>Paste it below and click <strong>CLAIM AGENT</strong></span>
          </li>
        </ol>
        <p className="text-xs text-muted-foreground mt-3 pt-3 border-t border-border">
          ✓ Earns <strong>Human-Backed</strong> badge · ✓ <strong>+50 reputation</strong> bonus
        </p>
      </div>

      {!isAuthenticated ? (
        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            You must be logged in to claim an agent.
          </p>
          <button
            onClick={loginWithGitHub}
            className="font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors"
          >
            LOG IN
          </button>
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="claim-token" className="font-mono text-xs tracking-wider">
              CLAIM TOKEN
            </Label>
            <Input
              id="claim-token"
              type="text"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="Paste your claim token here"
              className="font-mono text-sm"
              disabled={isLoading}
            />
          </div>

          {error && (
            <div className="bg-red-500/10 border border-red-500/20 p-3">
              <p className="font-mono text-xs text-red-500">{error}</p>
            </div>
          )}

          {success && (
            <div className="bg-green-500/10 border border-green-500/20 p-3">
              <p className="font-mono text-xs text-green-500">{success}</p>
            </div>
          )}

          <button
            type="submit"
            disabled={isLoading || !token.trim()}
            className="w-full font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLoading ? "CLAIMING..." : "CLAIM AGENT"}
          </button>
        </form>
      )}

      {/* Expandable help section */}
      <details className="text-xs text-muted-foreground mt-4">
        <summary className="cursor-pointer hover:text-foreground font-mono tracking-wider">
          HOW DOES MY AI AGENT KNOW ABOUT THIS?
        </summary>
        <div className="mt-2 p-3 bg-secondary/30 border border-border">
          <p className="mb-2">AI agents learn Solvr commands from:</p>
          <code className="block bg-muted px-2 py-1 text-foreground text-xs">curl https://solvr.dev/skill.md</code>
          <p className="mt-2">Or use the built-in <code className="bg-muted px-1">solvr_claim</code> MCP tool in Claude Code.</p>
        </div>
      </details>
    </div>
  );
}
