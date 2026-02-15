"use client";

import { useState } from "react";
import { Flag } from "lucide-react";
import { Button } from "@/components/ui/button";
import { QuestionData } from "@/hooks/use-question";
import { ReportModal } from "@/components/ui/report-modal";

interface QuestionContentProps {
  question: QuestionData;
}

export function QuestionContent({ question }: QuestionContentProps) {
  const [showReport, setShowReport] = useState(false);

  return (
    <div className="bg-card border border-border p-8">
      <div className="space-y-6">
        <div className="prose prose-sm max-w-none">
          <div className="text-foreground leading-relaxed whitespace-pre-wrap">
            {question.description}
          </div>
        </div>

        {question.tags.length > 0 && (
          <div className="flex flex-wrap gap-2 pt-4 border-t border-border">
            {question.tags.map((tag) => (
              <span
                key={tag}
                className="px-2 py-1 bg-secondary text-foreground font-mono text-[10px] tracking-wider border border-border hover:border-foreground/30 cursor-pointer transition-colors"
              >
                {tag}
              </span>
            ))}
          </div>
        )}

        <div className="flex items-center justify-between pt-4">
          <div className="flex items-center gap-4">
            <Button
              variant="ghost"
              size="sm"
              className="font-mono text-xs text-muted-foreground hover:text-foreground"
              onClick={() => setShowReport(true)}
            >
              <Flag className="w-3 h-3 mr-2" />
              FLAG
            </Button>
          </div>

          {question.updatedAt !== question.createdAt && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
              <span>edited {question.time}</span>
            </div>
          )}
        </div>
      </div>
      <ReportModal
        isOpen={showReport}
        onClose={() => setShowReport(false)}
        targetType="post"
        targetId={question.id}
        targetLabel="Question"
      />
    </div>
  );
}
