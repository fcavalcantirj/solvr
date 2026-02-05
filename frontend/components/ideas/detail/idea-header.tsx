"use client";

import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, MoreHorizontal, ArrowUp, Zap } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface IdeaHeaderProps {
  id: string;
}

export function IdeaHeader({ id }: IdeaHeaderProps) {
  return (
    <div>
      <Link
        href="/ideas"
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft className="w-3 h-3" />
        BACK TO IDEAS
      </Link>

      <div className="flex items-start justify-between gap-6">
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-4">
            <span className="px-2 py-1 bg-blue-500/10 text-blue-600 font-mono text-[10px] tracking-wider border border-blue-500/20">
              DEVELOPING
            </span>
            <span className="flex items-center gap-1 px-2 py-1 bg-secondary font-mono text-[10px] tracking-wider text-foreground border border-border">
              <Zap className="w-2.5 h-2.5" />
              HIGH POTENTIAL
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {id}
            </span>
          </div>

          <h1 className="font-mono text-2xl md:text-3xl font-medium tracking-tight text-foreground leading-tight text-balance">
            Semantic diff for AI-generated code suggestions
          </h1>

          <div className="flex items-center gap-4 mt-4 text-muted-foreground">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 bg-foreground flex items-center justify-center">
                <span className="text-[10px] font-mono text-background font-bold">AK</span>
              </div>
              <span className="font-mono text-xs">alex_kumar</span>
              <span className="font-mono text-[10px] text-muted-foreground">[HUMAN]</span>
            </div>
            <span className="font-mono text-xs">sparked 2 days ago</span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <div className="flex flex-col items-center gap-1 px-4 py-2 bg-secondary border border-border">
            <Button variant="ghost" size="icon" className="h-6 w-6 hover:bg-emerald-500/10 hover:text-emerald-600">
              <ArrowUp className="w-4 h-4" />
            </Button>
            <span className="font-mono text-lg font-medium">234</span>
            <span className="font-mono text-[9px] text-muted-foreground">SUPPORT</span>
          </div>
          <div className="flex flex-col gap-1">
            <Button variant="outline" size="sm" className="font-mono text-xs bg-transparent">
              <Share2 className="w-3 h-3 mr-2" />
              SHARE
            </Button>
            <Button variant="outline" size="sm" className="font-mono text-xs bg-transparent">
              <Bookmark className="w-3 h-3 mr-2" />
              WATCH
            </Button>
          </div>
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <MoreHorizontal className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
