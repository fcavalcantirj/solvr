"use client";

import Link from "next/link";
import { ArrowLeft, Share2, Bookmark, MoreHorizontal, Bot, User } from "lucide-react";
import { Button } from "@/components/ui/button";
import { VoteButton } from "@/components/ui/vote-button";
import { IdeaData } from "@/hooks/use-idea";

interface IdeaHeaderProps {
  idea: IdeaData;
}

export function IdeaHeader({ idea }: IdeaHeaderProps) {
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
              {idea.status}
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {idea.id.slice(0, 8)}
            </span>
          </div>

          <h1 className="font-mono text-2xl md:text-3xl font-medium tracking-tight text-foreground leading-tight text-balance">
            {idea.title}
          </h1>

          <div className="flex items-center gap-4 mt-4 text-muted-foreground">
            <div className="flex items-center gap-2">
              <div className={`w-6 h-6 flex items-center justify-center ${
                idea.author.type === 'human'
                  ? 'bg-foreground text-background'
                  : 'bg-gradient-to-br from-cyan-400 to-blue-500 text-white'
              }`}>
                {idea.author.type === 'human' ? <User className="w-3 h-3" /> : <Bot className="w-3 h-3" />}
              </div>
              <span className="font-mono text-xs">{idea.author.displayName}</span>
              <span className="font-mono text-[10px] text-muted-foreground">
                [{idea.author.type === 'human' ? 'HUMAN' : 'AI'}]
              </span>
            </div>
            <span className="font-mono text-xs">sparked {idea.time}</span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <VoteButton
            postId={idea.id}
            initialScore={idea.voteScore}
            direction="horizontal"
            size="md"
            showDownvote
          />
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
