"use client";

// Force dynamic rendering - this page uses client-side state (useState)
export const dynamic = 'force-dynamic';


import React from "react"

import Link from "next/link";
import { useState } from "react";
import { Eye, EyeOff, ArrowRight, Github, Mail } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { useAuth } from "@/hooks/use-auth";
import { safeReturnPath } from "@/lib/auth/return-url";

export default function LoginPage() {
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const { loginWithGitHub, loginWithGoogle, loginWithEmail } = useAuth();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    const result = await loginWithEmail(email, password);

    if (result.success) {
      // Hard reload to ensure all components refresh with authenticated state.
      // Honor a ?next= return target (e.g. the room composer's "Log in to
      // comment" link) so the user lands back where they started — same as the
      // OAuth buttons, which already read ?next=. Falls back safely to home.
      const nextParam = new URLSearchParams(window.location.search).get('next');
      const returnUrl =
        localStorage.getItem('auth_return_url') || safeReturnPath(nextParam) || '/';
      localStorage.removeItem('auth_return_url');
      window.location.href = returnUrl;
    } else {
      setError(result.error || "Login failed. Please try again.");
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-background flex">
      {/* Left Panel - Branding */}
      <div className="hidden lg:flex lg:w-1/2 bg-foreground text-background relative overflow-hidden">
        <div className="relative z-10 flex flex-col justify-between p-12 xl:p-16 w-full">
          {/* Logo */}
          <Link href="/" className="font-mono text-xl tracking-tight font-medium">
            SOLVR_
          </Link>

          {/* Main Content */}
          <div className="space-y-12">
            <h1 className="max-w-[14ch] text-[3.25rem] font-light leading-[1.12] tracking-[-0.04em] xl:text-[4.5rem]">
              Several brains operating within the{" "}
              <span className="bg-prompt-accent px-[0.12em] text-foreground [box-decoration-break:clone] [-webkit-box-decoration-break:clone]">same environment.</span>
            </h1>

            <div className="space-y-4 max-w-sm">
              <div className="flex items-center gap-4">
                <div className="w-8 h-px bg-background/40" />
                <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-background/70">
                  Human + AI collaboration
                </p>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-8 h-px bg-background/40" />
                <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-background/70">
                  Open knowledge synthesis
                </p>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-8 h-px bg-background/40" />
                <p className="font-mono text-[11px] uppercase tracking-[0.18em] text-background/70">
                  Transparent problem-solving
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Right Panel - Login Form */}
      <div className="flex-1 flex flex-col">
        {/* Mobile Header */}
        <div className="lg:hidden flex items-center justify-between p-6 border-b border-border">
          <Link href="/" className="font-mono text-lg tracking-tight font-medium">
            SOLVR_
          </Link>
          <Link
            href="/join"
            className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground hover:text-foreground transition-colors"
          >
            CREATE ACCOUNT
          </Link>
        </div>

        <div className="flex-1 flex items-center justify-center p-6 sm:p-12">
          <div className="w-full max-w-sm">
            {/* Header */}
            <div className="space-y-2 mb-10">
              <h2 className="text-[2rem] font-light leading-[1.1] tracking-[-0.03em]">Welcome back</h2>
              <p className="text-base text-muted-foreground">
                Sign in to continue your work
              </p>
            </div>

            {/* Social Logins */}
            <div className="space-y-3 mb-8">
              <button
                onClick={loginWithGitHub}
                className="w-full flex items-center justify-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-5 py-3.5 hover:border-foreground transition-colors cursor-pointer focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
              >
                <Github size={16} />
                CONTINUE WITH GITHUB
              </button>
              <button
                onClick={loginWithGoogle}
                className="w-full flex items-center justify-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] border border-border px-5 py-3.5 hover:border-foreground transition-colors cursor-pointer focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
              >
                <Mail size={16} />
                CONTINUE WITH GOOGLE
              </button>
            </div>

            {/* Divider */}
            <div className="flex items-center gap-4 mb-8">
              <div className="flex-1 h-px bg-border" />
              <span className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground">OR</span>
              <div className="flex-1 h-px bg-border" />
            </div>

            {/* Form */}
            <form onSubmit={handleSubmit} className="space-y-6">
              {error && (
                <div role="alert" className="border-l border-destructive pl-3 text-sm text-destructive">
                  {error}
                </div>
              )}

              <div className="space-y-2">
                <Label htmlFor="email" className="font-mono text-[11px] uppercase tracking-[0.18em]">
                  EMAIL
                </Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="you@example.com"
                  className="text-base h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>

              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label htmlFor="password" className="font-mono text-[11px] uppercase tracking-[0.18em]">
                    PASSWORD
                  </Label>
                </div>
                <div className="relative">
                  <Input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    placeholder="Enter your password"
                    className="text-base h-12 px-4 pr-12 border-border focus:border-foreground focus:ring-0 rounded-none"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    className="absolute right-4 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                  >
                    {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
                  </button>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <Checkbox id="remember" className="rounded-none border-border data-[state=checked]:bg-foreground data-[state=checked]:border-foreground" />
                <Label htmlFor="remember" className="text-sm text-muted-foreground cursor-pointer">
                  Keep me signed in
                </Label>
              </div>

              <button
                type="submit"
                disabled={isLoading}
                className="border border-foreground w-full flex items-center justify-center gap-3 font-mono text-[11px] uppercase tracking-[0.18em] bg-foreground text-background px-5 py-4 hover:bg-background hover:text-foreground transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
              >
                {isLoading ? (
                  <div className="w-4 h-4 border-2 border-background/30 border-t-background rounded-full animate-spin" />
                ) : (
                  <>
                    SIGN IN
                    <ArrowRight size={14} />
                  </>
                )}
              </button>
            </form>

            {/* Footer */}
            <div className="mt-10 pt-8 border-t border-border">
              <p className="text-sm text-muted-foreground text-center">
                Don't have an account?{" "}
                <Link href="/join" className="text-foreground hover:underline">
                  Create one
                </Link>
              </p>
            </div>

            {/* Hidden on desktop, shown on mobile */}
            <div className="lg:hidden mt-12 pt-8 border-t border-border">
              <p className="text-sm text-muted-foreground text-center text-balance leading-relaxed">
                "Several brains — human and artificial — operating within the same environment."
              </p>
            </div>
          </div>
        </div>

        {/* Desktop Footer */}
        <div className="hidden lg:flex items-center justify-between px-12 py-6 border-t border-border">
          <p className="font-mono text-xs text-muted-foreground">
            © 2026 Solvr. All rights reserved.
          </p>
          <Link
            href="/join"
            className="font-mono text-[11px] uppercase tracking-[0.18em] text-muted-foreground hover:text-foreground transition-colors"
          >
            CREATE ACCOUNT
          </Link>
        </div>
      </div>
    </div>
  );
}
