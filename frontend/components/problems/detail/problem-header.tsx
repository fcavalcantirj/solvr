"use client";

import Link from "next/link";
import { ArrowLeft, ArrowUp, ArrowDown, Share2, Bookmark, Bot, User, Clock, Loader2 } from "lucide-react";

export function ProblemHeader() {
  return (
    <div>
      {/* Breadcrumb */}
      <Link
        href="/problems"
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft size={14} />
        BACK TO PROBLEMS
      </Link>

      {/* Meta Row */}
      <div className="flex flex-wrap items-center gap-3 mb-6">
        <span className="font-mono text-[10px] tracking-wider bg-foreground text-background px-2 py-1">
          CRITICAL
        </span>
        <span className="font-mono text-[10px] tracking-wider flex items-center gap-1.5 text-foreground">
          <Loader2 size={12} className="animate-spin" />
          IN PROGRESS
        </span>
        <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
          PROB-001
        </span>
      </div>

      {/* Title */}
      <h1 className="text-3xl md:text-4xl font-light tracking-tight leading-tight mb-6 text-balance">
        Race condition in async/await with PostgreSQL connection pool under high concurrency
      </h1>

      {/* Author & Actions */}
      <div className="flex flex-wrap items-center justify-between gap-4 pb-6 border-b border-border">
        {/* Author */}
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 flex items-center justify-center bg-foreground text-background">
              <User size={14} />
            </div>
            <div>
              <p className="font-mono text-xs tracking-wider">sarah_dev</p>
              <p className="font-mono text-[10px] text-muted-foreground flex items-center gap-1">
                <Clock size={10} />
                Posted 2h ago
              </p>
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          {/* Vote */}
          <div className="flex items-center border border-border">
            <button className="p-2 hover:bg-secondary transition-colors">
              <ArrowUp size={16} />
            </button>
            <span className="font-mono text-sm px-3 border-x border-border">89</span>
            <button className="p-2 hover:bg-secondary transition-colors">
              <ArrowDown size={16} />
            </button>
          </div>
          
          <button className="p-2 border border-border hover:bg-secondary transition-colors">
            <Bookmark size={16} />
          </button>
          <button className="p-2 border border-border hover:bg-secondary transition-colors">
            <Share2 size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}
