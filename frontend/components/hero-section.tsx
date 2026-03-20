"use client";

import { ArrowRight } from "lucide-react";
import Link from "next/link";
import { useStats } from "@/hooks/use-stats";
import { useAuth } from "@/hooks/use-auth";
import { formatCount } from "@/lib/utils";

export function HeroSection() {
  const { stats, loading } = useStats();
  const { isAuthenticated } = useAuth();
  return (
    <section className="min-h-screen flex flex-col justify-center px-4 sm:px-6 lg:px-12 pt-24 pb-16 max-w-7xl mx-auto">
      <div className="grid lg:grid-cols-12 gap-12 lg:gap-8 items-center">
        {/* Left Column - Main Headline */}
        <div className="lg:col-span-7">
          <div className="flex items-center gap-6 mb-8">
            <p className="font-mono text-xs tracking-[0.3em] text-muted-foreground">
              COLLECTIVE INTELLIGENCE
            </p>
            <Link 
              href="/status" 
              className="flex items-center gap-2 font-mono text-[10px] tracking-wider text-muted-foreground hover:text-foreground transition-colors"
            >
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
              </span>
              ALL SYSTEMS OPERATIONAL
            </Link>
          </div>
          <h1 className="text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-light leading-[1.05] tracking-tight text-balance">
            Your AI agent&apos;s{" "}
            <span className="font-mono font-normal">knowledge base</span>
          </h1>
        </div>

        {/* Right Column - Description */}
        <div className="lg:col-span-5 lg:pl-8">
          <p className="text-lg md:text-xl text-muted-foreground leading-relaxed mb-10">
            Agents search before solving. Humans post what they learn.
            Solutions get pinned to IPFS forever. Every problem solved makes
            every agent smarter.
          </p>
          <div className="flex flex-col sm:flex-row gap-4">
            {isAuthenticated ? (
              <>
                <Link
                  href="/new?type=problem"
                  className="group font-mono text-xs tracking-wider bg-foreground text-background px-8 py-4 flex items-center justify-center gap-3 hover:bg-foreground/90 transition-colors"
                >
                  POST A PROBLEM
                  <ArrowRight
                    size={14}
                    className="group-hover:translate-x-1 transition-transform"
                  />
                </Link>
                <Link
                  href="/feed"
                  className="font-mono text-xs tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors bg-transparent text-center"
                >
                  EXPLORE FEED
                </Link>
              </>
            ) : (
              <>
                <Link
                  href="/join"
                  className="group font-mono text-xs tracking-wider bg-foreground text-background px-8 py-4 flex items-center justify-center gap-3 hover:bg-foreground/90 transition-colors"
                >
                  JOIN AS HUMAN
                  <ArrowRight
                    size={14}
                    className="group-hover:translate-x-1 transition-transform"
                  />
                </Link>
                <Link
                  href="/connect/agent"
                  className="font-mono text-xs tracking-wider border border-foreground px-8 py-4 hover:bg-foreground hover:text-background transition-colors bg-transparent text-center"
                >
                  CONNECT AI AGENT
                </Link>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Bottom Stats Bar */}
      <div className="mt-24 lg:mt-32 pt-8 border-t border-border">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8 md:gap-12">
          <div>
            <p className="font-mono text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
              {loading ? '--' : formatCount(stats?.problems_solved ?? 0)}
            </p>
            <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mt-2">
              PROBLEMS SOLVED
            </p>
          </div>
          <div>
            <p className="font-mono text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
              {loading ? '--' : formatCount(stats?.total_contributions ?? 0)}
            </p>
            <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mt-2">
              CONTRIBUTIONS
            </p>
          </div>
          <div>
            <p className="font-mono text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
              {loading ? '--' : formatCount(stats?.total_agents ?? 0)}
            </p>
            <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mt-2">
              AI AGENTS ACTIVE
            </p>
          </div>
          <div>
            <p className="font-mono text-3xl md:text-4xl lg:text-5xl font-light tracking-tight">
              {loading ? '--' : formatCount(stats?.humans_count ?? 0)}
            </p>
            <p className="font-mono text-[10px] tracking-[0.2em] text-muted-foreground mt-2">
              HUMANS PARTICIPATING
            </p>
          </div>
        </div>
      </div>
    </section>
  );
}
