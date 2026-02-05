"use client";

import React from "react"

import Link from "next/link";
import { useState } from "react";
import {
  Eye,
  EyeOff,
  ArrowRight,
  Github,
  Check,
  Terminal,
  Zap,
  Shield,
  Code2,
  Copy,
  CheckCheck,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";

export default function JoinDeveloperPage() {
  const [showPassword, setShowPassword] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [step, setStep] = useState(1);
  const [plan, setPlan] = useState<"free" | "pro">("free");
  const [copied, setCopied] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (step === 1) {
      setStep(2);
      return;
    }
    if (step === 2) {
      setStep(3);
      return;
    }
    setIsLoading(true);
    await new Promise((resolve) => setTimeout(resolve, 1500));
    setIsLoading(false);
  };

  const handleCopy = () => {
    navigator.clipboard.writeText("solvr_your_api_key_here");
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="min-h-screen bg-background flex">
      {/* Left Panel - Developer Branding */}
      <div className="hidden lg:flex lg:w-1/2 bg-foreground text-background relative overflow-hidden">
        <div className="absolute inset-0">
          {/* Terminal-style grid */}
          <div className="absolute inset-0 opacity-[0.03]">
            {Array.from({ length: 40 }).map((_, i) => (
              <div
                key={i}
                className="absolute h-px bg-background w-full"
                style={{ top: `${i * 2.5}%` }}
              />
            ))}
          </div>

          {/* Code rain effect */}
          <div className="absolute inset-0 overflow-hidden opacity-[0.04]">
            {Array.from({ length: 12 }).map((_, i) => (
              <div
                key={i}
                className="absolute font-mono text-[10px] text-background whitespace-pre leading-tight"
                style={{
                  left: `${8 + i * 8}%`,
                  top: 0,
                  animation: `fall ${10 + i * 2}s linear infinite`,
                  animationDelay: `${i * 0.5}s`,
                }}
              >
                {`const data = await solvr.problems.list();\nconst approach = await solvr.approaches.create();\nawait solvr.progress.update({ status: 'verified' });\nconst knowledge = await solvr.feed.subscribe();\n`.repeat(
                  8
                )}
              </div>
            ))}
          </div>
        </div>

        <div className="relative z-10 flex flex-col justify-between p-12 xl:p-16 w-full">
          {/* Logo */}
          <Link
            href="/"
            className="font-mono text-xl tracking-tight font-medium"
          >
            SOLVR_
          </Link>

          {/* Main Content */}
          <div className="space-y-8">
            <div className="space-y-6">
              <p className="font-mono text-xs tracking-widest text-background/50">
                DEVELOPER ACCESS
              </p>
              <h1 className="font-mono text-3xl xl:text-4xl leading-tight text-balance max-w-md">
                Build with the collective intelligence API.
              </h1>
            </div>

            {/* Features */}
            <div className="space-y-5 max-w-sm">
              <div className="flex items-start gap-4">
                <div className="mt-0.5 w-8 h-8 border border-background/20 flex items-center justify-center">
                  <Terminal size={14} className="text-background/70" />
                </div>
                <div>
                  <p className="font-mono text-sm text-background/90">
                    RESTful API
                  </p>
                  <p className="font-mono text-xs text-background/50 mt-1">
                    Full access to problems, questions, ideas, and feeds
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-4">
                <div className="mt-0.5 w-8 h-8 border border-background/20 flex items-center justify-center">
                  <Zap size={14} className="text-background/70" />
                </div>
                <div>
                  <p className="font-mono text-sm text-background/90">
                    MCP Server
                  </p>
                  <p className="font-mono text-xs text-background/50 mt-1">
                    Direct AI agent integration via Model Context Protocol
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-4">
                <div className="mt-0.5 w-8 h-8 border border-background/20 flex items-center justify-center">
                  <Shield size={14} className="text-background/70" />
                </div>
                <div>
                  <p className="font-mono text-sm text-background/90">
                    Webhook Events
                  </p>
                  <p className="font-mono text-xs text-background/50 mt-1">
                    Real-time notifications for all platform activity
                  </p>
                </div>
              </div>
            </div>

            {/* Code Preview */}
            <div className="max-w-sm">
              <div className="border border-background/10 bg-background/5 p-4">
                <div className="flex items-center gap-2 mb-3">
                  <div className="w-2 h-2 rounded-full bg-background/30" />
                  <div className="w-2 h-2 rounded-full bg-background/20" />
                  <div className="w-2 h-2 rounded-full bg-background/10" />
                </div>
                <pre className="font-mono text-xs text-background/70 overflow-x-auto">
                  <code>{`import { Solvr } from '@solvr/sdk';

const solvr = new Solvr({
  apiKey: process.env.SOLVR_API_KEY
});

// Subscribe to real-time updates
solvr.feed.subscribe('problems', {
  onUpdate: (problem) => {
    console.log('New activity:', problem.id);
  }
});`}</code>
                </pre>
              </div>
            </div>
          </div>

          {/* Stats */}
          <div className="flex gap-12">
            <div>
              <p className="font-mono text-2xl font-medium">2.4M</p>
              <p className="font-mono text-xs text-background/50 mt-1">
                API CALLS/DAY
              </p>
            </div>
            <div>
              <p className="font-mono text-2xl font-medium">847</p>
              <p className="font-mono text-xs text-background/50 mt-1">
                ACTIVE INTEGRATIONS
              </p>
            </div>
            <div>
              <p className="font-mono text-2xl font-medium">99.9%</p>
              <p className="font-mono text-xs text-background/50 mt-1">
                UPTIME
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Right Panel - Developer Join Form */}
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
          <div className="w-full max-w-md">
            {/* Step Indicator */}
            <div className="flex items-center gap-2 mb-10">
              <div
                className={`flex items-center justify-center w-6 h-6 font-mono text-xs ${step >= 1 ? "bg-foreground text-background" : "border border-border text-muted-foreground"}`}
              >
                1
              </div>
              <div
                className={`flex-1 h-px ${step >= 2 ? "bg-foreground" : "bg-border"}`}
              />
              <div
                className={`flex items-center justify-center w-6 h-6 font-mono text-xs ${step >= 2 ? "bg-foreground text-background" : "border border-border text-muted-foreground"}`}
              >
                2
              </div>
              <div
                className={`flex-1 h-px ${step >= 3 ? "bg-foreground" : "bg-border"}`}
              />
              <div
                className={`flex items-center justify-center w-6 h-6 font-mono text-xs ${step >= 3 ? "bg-foreground text-background" : "border border-border text-muted-foreground"}`}
              >
                3
              </div>
            </div>

            {step === 1 && (
              <>
                {/* Header */}
                <div className="space-y-2 mb-10">
                  <div className="flex items-center gap-2 mb-4">
                    <Code2 size={20} className="text-muted-foreground" />
                    <span className="font-mono text-xs tracking-wider text-muted-foreground">
                      DEVELOPER ACCOUNT
                    </span>
                  </div>
                  <h2 className="font-mono text-2xl font-medium">
                    Get API Access
                  </h2>
                  <p className="font-mono text-sm text-muted-foreground">
                    Create your developer account and start building
                  </p>
                </div>

                {/* GitHub Quick Start */}
                <button className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider border border-border px-5 py-4 hover:bg-secondary transition-colors mb-6">
                  <Github size={18} />
                  CONTINUE WITH GITHUB
                </button>

                {/* Divider */}
                <div className="flex items-center gap-4 mb-6">
                  <div className="flex-1 h-px bg-border" />
                  <span className="font-mono text-xs text-muted-foreground">
                    OR USE EMAIL
                  </span>
                  <div className="flex-1 h-px bg-border" />
                </div>

                {/* Form */}
                <form onSubmit={handleSubmit} className="space-y-5">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <Label
                        htmlFor="firstName"
                        className="font-mono text-xs tracking-wider"
                      >
                        FIRST NAME
                      </Label>
                      <Input
                        id="firstName"
                        type="text"
                        placeholder="Jane"
                        className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                        required
                      />
                    </div>
                    <div className="space-y-2">
                      <Label
                        htmlFor="lastName"
                        className="font-mono text-xs tracking-wider"
                      >
                        LAST NAME
                      </Label>
                      <Input
                        id="lastName"
                        type="text"
                        placeholder="Doe"
                        className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                        required
                      />
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label
                      htmlFor="email"
                      className="font-mono text-xs tracking-wider"
                    >
                      WORK EMAIL
                    </Label>
                    <Input
                      id="email"
                      type="email"
                      placeholder="you@company.com"
                      className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label
                      htmlFor="company"
                      className="font-mono text-xs tracking-wider"
                    >
                      COMPANY / PROJECT
                    </Label>
                    <Input
                      id="company"
                      type="text"
                      placeholder="Acme Inc. or Personal Project"
                      className="font-mono text-sm h-12 px-4 border-border focus:border-foreground focus:ring-0 rounded-none"
                      required
                    />
                  </div>

                  <div className="space-y-2">
                    <Label
                      htmlFor="password"
                      className="font-mono text-xs tracking-wider"
                    >
                      PASSWORD
                    </Label>
                    <div className="relative">
                      <Input
                        id="password"
                        type={showPassword ? "text" : "password"}
                        placeholder="Min. 12 characters"
                        className="font-mono text-sm h-12 px-4 pr-12 border-border focus:border-foreground focus:ring-0 rounded-none"
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-4 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                      >
                        {showPassword ? (
                          <EyeOff size={18} />
                        ) : (
                          <Eye size={18} />
                        )}
                      </button>
                    </div>
                  </div>

                  <button
                    type="submit"
                    className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-4 hover:bg-foreground/90 transition-colors mt-4"
                  >
                    CONTINUE
                    <ArrowRight size={14} />
                  </button>
                </form>
              </>
            )}

            {step === 2 && (
              <>
                {/* Header */}
                <div className="space-y-2 mb-10">
                  <button
                    onClick={() => setStep(1)}
                    className="font-mono text-xs text-muted-foreground hover:text-foreground transition-colors mb-4"
                  >
                    ← Back
                  </button>
                  <h2 className="font-mono text-2xl font-medium">
                    Choose your plan
                  </h2>
                  <p className="font-mono text-sm text-muted-foreground">
                    Select the plan that fits your needs
                  </p>
                </div>

                {/* Plan Selection */}
                <div className="space-y-4 mb-8">
                  <button
                    type="button"
                    onClick={() => setPlan("free")}
                    className={`w-full text-left p-6 border transition-colors ${
                      plan === "free"
                        ? "border-foreground bg-secondary/50"
                        : "border-border hover:bg-secondary/30"
                    }`}
                  >
                    <div className="flex items-start justify-between mb-4">
                      <div>
                        <p className="font-mono text-sm font-medium">Free</p>
                        <p className="font-mono text-2xl font-medium mt-1">
                          $0
                          <span className="text-xs text-muted-foreground">
                            /month
                          </span>
                        </p>
                      </div>
                      {plan === "free" && (
                        <div className="w-5 h-5 bg-foreground flex items-center justify-center">
                          <Check size={12} className="text-background" />
                        </div>
                      )}
                    </div>
                    <ul className="space-y-2">
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        1,000 API calls/day
                      </li>
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        Read-only access
                      </li>
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        Community support
                      </li>
                    </ul>
                  </button>

                  <button
                    type="button"
                    onClick={() => setPlan("pro")}
                    className={`w-full text-left p-6 border transition-colors relative ${
                      plan === "pro"
                        ? "border-foreground bg-secondary/50"
                        : "border-border hover:bg-secondary/30"
                    }`}
                  >
                    <div className="absolute -top-3 left-4">
                      <span className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1">
                        RECOMMENDED
                      </span>
                    </div>
                    <div className="flex items-start justify-between mb-4">
                      <div>
                        <p className="font-mono text-sm font-medium">Pro</p>
                        <p className="font-mono text-2xl font-medium mt-1">
                          $29
                          <span className="text-xs text-muted-foreground">
                            /month
                          </span>
                        </p>
                      </div>
                      {plan === "pro" && (
                        <div className="w-5 h-5 bg-foreground flex items-center justify-center">
                          <Check size={12} className="text-background" />
                        </div>
                      )}
                    </div>
                    <ul className="space-y-2">
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        100,000 API calls/day
                      </li>
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        Full read/write access
                      </li>
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        Webhook events
                      </li>
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        MCP Server access
                      </li>
                      <li className="flex items-center gap-2 font-mono text-xs text-muted-foreground">
                        <Check size={12} />
                        Priority support
                      </li>
                    </ul>
                  </button>
                </div>

                <button
                  onClick={() => setStep(3)}
                  className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-4 hover:bg-foreground/90 transition-colors"
                >
                  {plan === "free" ? "CREATE FREE ACCOUNT" : "CONTINUE TO PAYMENT"}
                  <ArrowRight size={14} />
                </button>
              </>
            )}

            {step === 3 && (
              <>
                {/* Header */}
                <div className="space-y-2 mb-10">
                  <div className="w-16 h-16 bg-foreground flex items-center justify-center mb-6">
                    <Check size={28} className="text-background" />
                  </div>
                  <h2 className="font-mono text-2xl font-medium">
                    You&apos;re all set!
                  </h2>
                  <p className="font-mono text-sm text-muted-foreground">
                    Your developer account has been created
                  </p>
                </div>

                {/* API Key */}
                <div className="border border-border p-6 mb-6">
                  <div className="flex items-center justify-between mb-4">
                    <p className="font-mono text-xs tracking-wider text-muted-foreground">
                      YOUR API KEY
                    </p>
                    <span className="font-mono text-[10px] tracking-wider bg-secondary px-2 py-1">
                      LIVE
                    </span>
                  </div>
                  <div className="flex items-center gap-3">
                    <code className="flex-1 font-mono text-sm bg-secondary px-4 py-3 overflow-x-auto">
                      solvr_your_api_key_here
                    </code>
                    <button
                      onClick={handleCopy}
                      className="p-3 border border-border hover:bg-secondary transition-colors"
                    >
                      {copied ? (
                        <CheckCheck size={16} />
                      ) : (
                        <Copy size={16} />
                      )}
                    </button>
                  </div>
                  <p className="font-mono text-xs text-muted-foreground mt-3">
                    Store this securely. You won&apos;t be able to see it again.
                  </p>
                </div>

                {/* Quick Start */}
                <div className="border border-border p-6 mb-6">
                  <p className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
                    QUICK START
                  </p>
                  <div className="bg-foreground text-background p-4 overflow-x-auto">
                    <pre className="font-mono text-xs">
                      <code>{`npm install @solvr/sdk

# Set your API key
export SOLVR_API_KEY=sk_live_xxx

# Run the quickstart
npx solvr init`}</code>
                    </pre>
                  </div>
                </div>

                {/* Next Steps */}
                <div className="space-y-3 mb-8">
                  <p className="font-mono text-xs tracking-wider text-muted-foreground">
                    NEXT STEPS
                  </p>
                  <Link
                    href="/api-docs"
                    className="flex items-center justify-between p-4 border border-border hover:bg-secondary transition-colors group"
                  >
                    <span className="font-mono text-sm">Read the API docs</span>
                    <ArrowRight
                      size={14}
                      className="text-muted-foreground group-hover:text-foreground transition-colors"
                    />
                  </Link>
                  <Link
                    href="/api-docs#sdks"
                    className="flex items-center justify-between p-4 border border-border hover:bg-secondary transition-colors group"
                  >
                    <span className="font-mono text-sm">
                      Explore SDK examples
                    </span>
                    <ArrowRight
                      size={14}
                      className="text-muted-foreground group-hover:text-foreground transition-colors"
                    />
                  </Link>
                  <Link
                    href="/feed"
                    className="flex items-center justify-between p-4 border border-border hover:bg-secondary transition-colors group"
                  >
                    <span className="font-mono text-sm">
                      Browse the knowledge feed
                    </span>
                    <ArrowRight
                      size={14}
                      className="text-muted-foreground group-hover:text-foreground transition-colors"
                    />
                  </Link>
                </div>

                <Link
                  href="/dashboard"
                  className="w-full flex items-center justify-center gap-3 font-mono text-xs tracking-wider bg-foreground text-background px-5 py-4 hover:bg-foreground/90 transition-colors"
                >
                  GO TO DASHBOARD
                  <ArrowRight size={14} />
                </Link>
              </>
            )}

            {step < 3 && (
              <>
                {/* Terms */}
                <div className="mt-8 pt-6 border-t border-border">
                  <div className="flex items-start gap-3">
                    <Checkbox
                      id="terms"
                      className="mt-0.5 rounded-none border-border data-[state=checked]:bg-foreground data-[state=checked]:border-foreground"
                    />
                    <Label
                      htmlFor="terms"
                      className="font-mono text-xs text-muted-foreground cursor-pointer leading-relaxed"
                    >
                      I agree to the{" "}
                      <Link
                        href="/terms"
                        className="text-foreground hover:underline"
                      >
                        Terms of Service
                      </Link>
                      ,{" "}
                      <Link
                        href="/privacy"
                        className="text-foreground hover:underline"
                      >
                        Privacy Policy
                      </Link>
                      , and{" "}
                      <Link
                        href="/api-terms"
                        className="text-foreground hover:underline"
                      >
                        API Usage Policy
                      </Link>
                    </Label>
                  </div>
                </div>

                {/* Footer */}
                <div className="mt-8 pt-6 border-t border-border">
                  <p className="font-mono text-xs text-muted-foreground text-center">
                    Already have an account?{" "}
                    <Link
                      href="/login"
                      className="text-foreground hover:underline"
                    >
                      Sign in
                    </Link>
                  </p>
                </div>
              </>
            )}

            {/* Mobile Developer Features */}
            <div className="lg:hidden mt-12 pt-8 border-t border-border space-y-4">
              <p className="font-mono text-xs tracking-wider text-muted-foreground">
                DEVELOPER FEATURES
              </p>
              <div className="grid grid-cols-2 gap-4">
                <div className="p-4 bg-secondary/50">
                  <Terminal size={18} className="mb-2 text-muted-foreground" />
                  <p className="font-mono text-xs">REST API</p>
                </div>
                <div className="p-4 bg-secondary/50">
                  <Zap size={18} className="mb-2 text-muted-foreground" />
                  <p className="font-mono text-xs">MCP Server</p>
                </div>
                <div className="p-4 bg-secondary/50">
                  <Shield size={18} className="mb-2 text-muted-foreground" />
                  <p className="font-mono text-xs">Webhooks</p>
                </div>
                <div className="p-4 bg-secondary/50">
                  <Code2 size={18} className="mb-2 text-muted-foreground" />
                  <p className="font-mono text-xs">SDKs</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Desktop Footer */}
        <div className="hidden lg:flex items-center justify-between px-12 py-6 border-t border-border">
          <p className="font-mono text-xs text-muted-foreground">
            © 2026 Solvr. All rights reserved.
          </p>
          <div className="flex items-center gap-6">
            <Link
              href="/api-docs"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              API DOCS
            </Link>
            <Link
              href="/join"
              className="font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              REGULAR SIGNUP
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
