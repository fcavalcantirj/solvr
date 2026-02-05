"use client";

import { Clock, Bot, User } from "lucide-react";
import { IdeaData } from "@/hooks/use-idea";

interface IdeaSidePanelProps {
  idea: IdeaData;
}

export function IdeaSidePanel({ idea }: IdeaSidePanelProps) {
  return (
    <div className="space-y-6">
      {/* Idea Stats */}
      <div className="bg-card border border-border p-5">
        <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
          IDEA STATS
        </h3>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Status</span>
            <span className="px-2 py-0.5 bg-blue-500/10 text-blue-600 font-mono text-[10px] border border-blue-500/20">
              {idea.status}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Support</span>
            <span className="font-mono text-xs">{idea.voteScore} votes</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="font-mono text-xs text-muted-foreground">Sparked</span>
            <span className="font-mono text-xs">{idea.time}</span>
          </div>
        </div>
      </div>

      {/* Author */}
      <div className="bg-card border border-border p-5">
        <div className="flex items-center justify-between mb-4">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground">
            SPARKED BY
          </h3>
          <Clock className="w-3 h-3 text-muted-foreground" />
        </div>
        <div className="flex items-center gap-3">
          <div
            className={`w-8 h-8 flex items-center justify-center ${
              idea.author.type === "human"
                ? "bg-foreground text-background"
                : "bg-gradient-to-br from-cyan-400 to-blue-500 text-white"
            }`}
          >
            {idea.author.type === "human" ? (
              <User className="w-4 h-4" />
            ) : (
              <Bot className="w-4 h-4" />
            )}
          </div>
          <div>
            <p className="font-mono text-xs tracking-wider">{idea.author.displayName}</p>
            <p className="font-mono text-[10px] text-muted-foreground">
              {idea.author.type === "human" ? "Human" : "AI Agent"}
            </p>
          </div>
        </div>
      </div>

      {/* Tags */}
      {idea.tags.length > 0 && (
        <div className="bg-card border border-border p-5">
          <h3 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
            TAGS
          </h3>
          <div className="flex flex-wrap gap-2">
            {idea.tags.map((tag) => (
              <span
                key={tag}
                className="font-mono text-[10px] tracking-wider text-muted-foreground bg-secondary px-2 py-1"
              >
                {tag}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
