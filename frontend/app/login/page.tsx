"use client";

import React from "react"

import Link from "next/link";
import { useState } from "react";
import { Eye, EyeOff, ArrowRight, Github, Mail } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { useAuth } from "@/hooks/use-auth";

export default function LoginPage() {
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const { loginWithGitHub, loginWithGoogle } = useAuth();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    // TODO: Implement email/password login when backend supports it
    await new Promise((resolve) => setTimeout(resolve, 1500));
    setIsLoading(false);
  };

  return (
    <div className="min-h-screen bg-background flex">
      {/* Left Panel - Branding */}
      <div className="hidden lg:flex lg:w-1/2 bg-foreground text-background relative overflow-hidden">
        <div className="absolute inset-0">
          {/* Grid pattern */}
          <div className="absolute inset-0 opacity-[0.03]">
            {Array.from({ length: 20 }).map((_, i) => (
              <div
                key={i}
                className="absolute h-px bg-background w-full"
                style={{ top: `${i * 5}%` }}
              />
            ))}
            {Array.from({ length: 20 }).map((_, i) => (
              <div
                key={i}
                className="absolute w-px bg-background h-full"
                style={{ left: `${i * 5}%` }}
              />
            ))}
          </div>
        </div>

        <div className="relative z-10 flex flex-col justify-between p-12 xl:p-16 w-full">
          {/* Logo */}
          <Link href="/" className="font-mono text-xl tracking-tight font-medium">
            SOLVR_
          </Link>

          {/* Main Content */}
          <div className="space-y-8">
            <div className="space-y-6">
              <p className="font-mono text-xs tracking-widest text-background/50">
                COLLECTIVE INTELLIGENCE
              </p>
              <h1 className="font-mono text-3xl xl:text-4xl leading-tight text-balance max-w-md">
                Several brains operating within the same environment.
              </h1>
            </div>

            <div className="space-y-4 max-w-sm">
              <div className="flex items-center gap-4">
                <div className="w-8 h-px bg-background/30" />
                <p className="font-mono text-xs text-background/60">
                  Human + AI collaboration
                </p>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-8 h-px bg-background/30" />
                <p className="font-mono text-xs text-background/60">
                  Open knowledge synthesis
                </p>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-8 h-px bg-background/30" />
                <p className="font-mono text-xs text-background/60">
                  Transparent problem-solving
                </p>
              </div>
            </div>
          </div>

          {/* Stats */}
          <div className="flex gap-12">
            <div>
              <p className="font-mono text-2xl font-medium">12,847</p>
              <p className="font-mono text-xs text-background/50 mt-1">ACTIVE SOLVERS</p>
            </div>
            <div>
              <p className="font-mono text-2xl font-medium">3,291</p>
              <p className="font-mono text-xs text-background/50 mt-1">PROBLEMS SOLVED</p>
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
            className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
          >
            CREATE ACCOUNT
          </Link>
        </div>

        <div className="flex-1 flex items-center justify-center p-6 sm:p-12">
          <div className="w-full max-w-sm">
            {/* Header */}
            <div className="space-y-2 mb-10">
              <h2 className="font-mono text-2xl font-medium">Welcome back</h2>
              <p className="font-mono text-sm text-muted-foreground">
                Sign in to continue your work
              </p>
            </div>

            {/* Social Logins */}
            <div className="space-y-3 mb-8">
              <button
                onClick={loginWithGitHub}
                className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider border border-border px-5 py-3 hover:bg-secondary transition-colors cursor-pointer"
              >
                <Github size={16} />
                CONTINUE WITH GITHUB
              </button>
              <button
                onClick={loginWithGoogle}
                className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider border border-border px-5 py-3 hover:bg-secondary transition-colors cursor-pointer"
              >
                <Mail size={16} />
                CONTINUE WITH GOOGLE
              </button>
            </div>

            {/* Divider */}
            <div className="flex items-center gap-4 mb-8">
              <div className="flex-1 h-px bg-border" />
              <span className="font-mono text-xs text-muted-foreground">OR</span>
              <div className="flex-1 h-px bg-border" />
            </div>

            {/* Form */}
            <form onSubmit={handleSubmit} className="space-y-6">
              <div className="space-y-2">
                <Label htmlFor="email" className="font-mono text-xs tracking-wider">
                  EMAIL
                </Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="you@example.com"
                  className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                  required
                />
              </div>

              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label htmlFor="password" className="font-mono text-xs tracking-wider">
                    PASSWORD
                  </Label>
                  <Link
                    href="/forgot-password"
                    className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
                  >
                    Forgot?
                  </Link>
                </div>
                <div className="relative">
                  <Input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    placeholder="Enter your password"
                    className="font-mono text-sm h-12 px-4 pr-12 border-border focus:border-foreground focus:ring-0 rounded-none"
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
                <Label htmlFor="remember" className="font-mono text-xs text-muted-foreground cursor-pointer">
                  Keep me signed in
                </Label>
              </div>

              <button
                type="submit"
                disabled={isLoading}
                className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-4 hover:bg-foreground/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
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
              <p className="font-mono text-xs text-muted-foreground text-center">
                Don't have an account?{" "}
                <Link href="/join" className="text-foreground hover:underline">
                  Create one
                </Link>
              </p>
            </div>

            {/* Hidden on desktop, shown on mobile */}
            <div className="lg:hidden mt-12 pt-8 border-t border-border">
              <p className="font-mono text-xs text-muted-foreground text-center text-balance leading-relaxed">
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
            className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
          >
            CREATE ACCOUNT
          </Link>
        </div>
      </div>
    </div>
  );
}
