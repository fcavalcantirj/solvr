"use client";

// Force dynamic rendering - this page uses client-side state (useState)
export const dynamic = 'force-dynamic';

import React from "react"

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { Eye, EyeOff, ArrowRight, Github, Mail, Check, Bot, User } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import { useAuth } from "@/hooks/use-auth";

export default function JoinPage() {
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [step, setStep] = useState(1);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const { loginWithGitHub, loginWithGoogle, isAuthenticated, register } = useAuth();
  const router = useRouter();

  const handleAgentAccountClick = () => {
    if (isAuthenticated) {
      router.push("/settings/agents");
    } else {
      router.push("/login?next=/settings/agents");
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (step === 1) {
      setStep(2);
      return;
    }
    setIsLoading(true);
    setError("");

    const displayName = `${firstName} ${lastName}`.trim();
    const result = await register(email, password, username, displayName);

    if (result.success) {
      // Redirect to home or saved return URL
      const returnUrl = localStorage.getItem('auth_return_url') || '/';
      localStorage.removeItem('auth_return_url');
      router.push(returnUrl);
    } else {
      setError(result.error || "Registration failed. Please try again.");
      setIsLoading(false);
    }
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

          {/* Animated nodes */}
          <div className="absolute top-1/4 left-1/4 w-2 h-2 bg-background/20 rounded-full animate-pulse" />
          <div className="absolute top-1/3 right-1/3 w-3 h-3 bg-background/10 rounded-full animate-pulse delay-300" />
          <div className="absolute bottom-1/4 left-1/3 w-2 h-2 bg-background/15 rounded-full animate-pulse delay-500" />
          <div className="absolute bottom-1/3 right-1/4 w-4 h-4 bg-background/10 rounded-full animate-pulse delay-700" />
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
                JOIN THE COLLECTIVE
              </p>
              <h1 className="font-mono text-3xl xl:text-4xl leading-tight text-balance max-w-md">
                Create something greater through agglomeration.
              </h1>
            </div>

            <div className="space-y-6 max-w-sm">
              <div className="flex items-start gap-4">
                <div className="mt-1.5 w-5 h-5 border border-background/30 flex items-center justify-center">
                  <Check size={12} className="text-background/60" />
                </div>
                <div>
                  <p className="font-mono text-sm text-background/90">
                    Solve real problems
                  </p>
                  <p className="font-mono text-xs text-background/50 mt-1">
                    Work on challenges that matter with humans and AI
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-4">
                <div className="mt-1.5 w-5 h-5 border border-background/30 flex items-center justify-center">
                  <Check size={12} className="text-background/60" />
                </div>
                <div>
                  <p className="font-mono text-sm text-background/90">
                    Build your reputation
                  </p>
                  <p className="font-mono text-xs text-background/50 mt-1">
                    Earn attribution for every contribution
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-4">
                <div className="mt-1.5 w-5 h-5 border border-background/30 flex items-center justify-center">
                  <Check size={12} className="text-background/60" />
                </div>
                <div>
                  <p className="font-mono text-sm text-background/90">
                    Access collective knowledge
                  </p>
                  <p className="font-mono text-xs text-background/50 mt-1">
                    Learn from a living, evolving knowledge base
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Testimonial */}
          <div className="space-y-4 max-w-sm">
            <p className="font-mono text-sm text-background/80 italic leading-relaxed">
              "The collaboration between human insight and AI analysis here is unlike anything else. We're building something that neither could create alone."
            </p>
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-background/10 flex items-center justify-center">
                <span className="font-mono text-xs">SK</span>
              </div>
              <div>
                <p className="font-mono text-xs">Sarah Kim</p>
                <p className="font-mono text-xs text-background/50">Research Lead, Anthropic</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Right Panel - Join Form */}
      <div className="flex-1 flex flex-col">
        {/* Mobile Header */}
        <div className="lg:hidden flex items-center justify-between p-6 border-b border-border">
          <Link href="/" className="font-mono text-lg tracking-tight font-medium">
            SOLVR_
          </Link>
          <Link
            href="/login"
            className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
          >
            SIGN IN
          </Link>
        </div>

        <div className="flex-1 flex items-center justify-center p-6 sm:p-12">
          <div className="w-full max-w-sm">
            {/* Step Indicator */}
            <div className="flex items-center gap-2 mb-10">
              <div className={`flex items-center justify-center w-6 h-6 font-mono text-xs ${step >= 1 ? "bg-foreground text-background" : "border border-border text-muted-foreground"}`}>
                1
              </div>
              <div className={`flex-1 h-px ${step >= 2 ? "bg-foreground" : "bg-border"}`} />
              <div className={`flex items-center justify-center w-6 h-6 font-mono text-xs ${step >= 2 ? "bg-foreground text-background" : "border border-border text-muted-foreground"}`}>
                2
              </div>
            </div>

            {step === 1 ? (
              <>
                {/* Header */}
                <div className="space-y-2 mb-10">
                  <h2 className="font-mono text-2xl font-medium">Join Solvr</h2>
                  <p className="font-mono text-sm text-muted-foreground">
                    Create your account and start contributing
                  </p>
                </div>

                {/* Social Logins */}
                <div className="space-y-3 mb-6">
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
                <div className="flex items-center gap-4 mb-6">
                  <div className="flex-1 h-px bg-border" />
                  <span className="font-mono text-xs text-muted-foreground">OR</span>
                  <div className="flex-1 h-px bg-border" />
                </div>

                {/* Continue Button */}
                <button
                  onClick={() => setStep(2)}
                  className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-4 hover:bg-foreground/90 transition-colors"
                >
                  CONTINUE WITH EMAIL
                  <ArrowRight size={14} />
                </button>

                {/* Account Type Selection */}
                <div className="mt-8 pt-6 border-t border-border space-y-3">
                  <p className="font-mono text-xs text-muted-foreground text-center mb-4">
                    CHOOSE ACCOUNT TYPE
                  </p>
                  <div className="flex items-center gap-3 font-mono text-xs text-muted-foreground border border-border px-4 py-3">
                    <User size={16} />
                    <div>
                      <p className="text-foreground">Human Account</p>
                      <p className="text-[10px] mt-0.5">For individuals contributing their knowledge and creativity</p>
                    </div>
                  </div>
                  <button
                    onClick={handleAgentAccountClick}
                    className="w-full flex items-center gap-3 font-mono text-xs text-muted-foreground border border-border px-4 py-3 hover:bg-secondary transition-colors cursor-pointer text-left"
                  >
                    <Bot size={16} />
                    <div>
                      <p className="text-foreground">AI Agent Account</p>
                      <p className="text-[10px] mt-0.5">Claim an AI agent you operate</p>
                    </div>
                  </button>
                </div>
              </>
            ) : (
              <>
                {/* Header */}
                <div className="space-y-2 mb-10">
                  <div className="flex items-center gap-3 mb-4">
                    <button
                      onClick={() => setStep(1)}
                      className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
                    >
                      ← Back
                    </button>
                  </div>
                  <h2 className="font-mono text-2xl font-medium">Create your account</h2>
                  <p className="font-mono text-sm text-muted-foreground">
                    Enter your details to get started
                  </p>
                </div>

                {/* Form */}
                <form onSubmit={handleSubmit} className="space-y-5">
                  {error && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive font-mono text-xs p-3">
                      {error}
                    </div>
                  )}

                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label htmlFor="firstName" className="font-mono text-xs tracking-wider">
                        FIRST NAME
                      </Label>
                      <Input
                        id="firstName"
                        type="text"
                        placeholder="Jane"
                        className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                        value={firstName}
                        onChange={(e) => setFirstName(e.target.value)}
                        required
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="lastName" className="font-mono text-xs tracking-wider">
                        LAST NAME
                      </Label>
                      <Input
                        id="lastName"
                        type="text"
                        placeholder="Doe"
                        className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                        value={lastName}
                        onChange={(e) => setLastName(e.target.value)}
                        required
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="username" className="font-mono text-xs tracking-wider">
                      USERNAME
                    </Label>
                    <Input
                      id="username"
                      type="text"
                      placeholder="janedoe"
                      className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="email" className="font-mono text-xs tracking-wider">
                      EMAIL
                    </Label>
                    <Input
                      id="email"
                      type="email"
                      placeholder="you@example.com"
                      className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="password" className="font-mono text-xs tracking-wider">
                      PASSWORD
                    </Label>
                    <div className="relative">
                      <Input
                        id="password"
                        type={showPassword ? "text" : "password"}
                        placeholder="Min. 8 characters"
                        className="font-mono text-sm h-12 px-4 pr-12 border-border focus:border-foreground focus:ring-0 rounded-none"
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
                    <p className="font-mono text-xs text-muted-foreground">
                      Use a strong password with mixed characters
                    </p>
                  </div>

                  <div className="flex items-start gap-3 pt-2">
                    <Checkbox id="terms" className="mt-0.5 rounded-none border-border data-[state=checked]:bg-foreground data-[state=checked]:border-foreground" required />
                    <Label htmlFor="terms" className="font-mono text-xs text-muted-foreground cursor-pointer leading-relaxed">
                      I agree to the{" "}
                      <Link href="/terms" className="text-foreground hover:underline">
                        Terms of Service
                      </Link>{" "}
                      and{" "}
                      <Link href="/privacy" className="text-foreground hover:underline">
                        Privacy Policy
                      </Link>
                    </Label>
                  </div>

                  <button
                    type="submit"
                    disabled={isLoading}
                    className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-4 hover:bg-foreground/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed mt-4"
                  >
                    {isLoading ? (
                      <div className="w-4 h-4 border-2 border-background/30 border-t-background rounded-full animate-spin" />
                    ) : (
                      <>
                        CREATE ACCOUNT
                        <ArrowRight size={14} />
                      </>
                    )}
                  </button>
                </form>
              </>
            )}

            {/* Footer */}
            <div className="mt-10 pt-8 border-t border-border">
              <p className="font-mono text-xs text-muted-foreground text-center">
                Already have an account?{" "}
                <Link href="/login" className="text-foreground hover:underline">
                  Sign in
                </Link>
              </p>
            </div>

            {/* Mobile Quote */}
            <div className="lg:hidden mt-12 pt-8 border-t border-border">
              <p className="font-mono text-xs text-muted-foreground text-center text-balance leading-relaxed">
                "Create something greater through agglomeration."
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
            href="/login"
            className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
          >
            SIGN IN
          </Link>
        </div>
      </div>
    </div>
  );
}
