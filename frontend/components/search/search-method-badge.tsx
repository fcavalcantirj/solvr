"use client";

import { Sparkles } from "lucide-react";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";

interface SearchMethodBadgeProps {
  method?: 'hybrid' | 'fulltext';
}

export function SearchMethodBadge({ method }: SearchMethodBadgeProps) {
  if (method !== 'hybrid') {
    return null;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          data-testid="search-method-badge"
          className="inline-flex items-center gap-1.5 text-xs text-muted-foreground cursor-default"
        >
          <Sparkles size={12} />
          <span>Semantic search enabled</span>
        </div>
      </TooltipTrigger>
      <TooltipContent>
        Using AI embeddings to find semantically similar content
      </TooltipContent>
    </Tooltip>
  );
}
