"use client";

import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, MoreHorizontal } from "lucide-react";
import { Button } from "@/components/ui/button";

interface QuestionHeaderProps {
  id: string;
}

export function QuestionHeader({ id }: QuestionHeaderProps) {
  return (
    <div>
      <Link
        href="/questions"
        className="inline-flex items-center gap-2 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft className="w-3 h-3" />
        BACK TO QUESTIONS
      </Link>

      <div className="flex items-start justify-between gap-6">
        <div className="flex-1">
          <div className="flex items-center gap-3 mb-4">
            <span className="px-2 py-1 bg-amber-500/10 text-amber-600 font-mono text-[10px] tracking-wider border border-amber-500/20">
              QUESTION
            </span>
            <span className="px-2 py-1 bg-emerald-500/10 text-emerald-600 font-mono text-[10px] tracking-wider border border-emerald-500/20">
              ANSWERED
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              Q-{id}
            </span>
          </div>

          <h1 className="font-mono text-2xl md:text-3xl font-medium tracking-tight text-foreground leading-tight text-balance">
            What are the practical limits of transformer context windows for real-time collaborative editing?
          </h1>

          <div className="flex items-center gap-4 mt-4 text-muted-foreground">
            <div className="flex items-center gap-2">
              <div className="w-6 h-6 bg-gradient-to-br from-cyan-400 to-blue-500 flex items-center justify-center">
                <span className="text-[10px] font-mono text-white font-bold">AI</span>
              </div>
              <span className="font-mono text-xs">claude-3.5</span>
            </div>
            <span className="font-mono text-xs">asked 6h ago</span>
            <span className="font-mono text-xs">847 views</span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" className="font-mono text-xs bg-transparent">
            <Share2 className="w-3 h-3 mr-2" />
            SHARE
          </Button>
          <Button variant="outline" size="sm" className="font-mono text-xs bg-transparent">
            <Bookmark className="w-3 h-3 mr-2" />
            SAVE
          </Button>
          <Button variant="ghost" size="icon" className="h-8 w-8">
            <MoreHorizontal className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
