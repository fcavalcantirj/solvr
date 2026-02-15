"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { Bot, User, Trophy, HelpCircle, MessageSquare, TrendingUp, Zap } from "lucide-react";
import { useQuestionsStats } from "@/hooks/use-questions-stats";
import { useTrending } from "@/hooks/use-stats";
import { api, APIPost, formatRelativeTime } from "@/lib/api";

function formatNumber(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k';
  return n.toLocaleString();
}

function formatResponseTime(hours: number): string {
  if (hours < 1) {
    const mins = Math.round(hours * 60);
    return mins <= 0 ? '<1min' : `${mins}min`;
  }
  return `${Math.round(hours)}h`;
}

interface QuestionsSidebarProps {
  onTagClick?: (tag: string) => void;
}

export function QuestionsSidebar({ onTagClick }: QuestionsSidebarProps) {
  const { stats: questionsStats, loading: statsLoading } = useQuestionsStats();
  const { trending, loading: trendingLoading } = useTrending();
  const [unansweredQuestions, setUnansweredQuestions] = useState<APIPost[]>([]);
  const [unansweredLoading, setUnansweredLoading] = useState(true);
  const [hotQuestions, setHotQuestions] = useState<APIPost[]>([]);
  const [hotLoading, setHotLoading] = useState(true);

  useEffect(() => {
    api.getQuestions({ status: 'open', sort: 'votes', per_page: 3 })
      .then(res => setUnansweredQuestions(res.data))
      .catch(() => setUnansweredQuestions([]))
      .finally(() => setUnansweredLoading(false));
  }, []);

  useEffect(() => {
    api.getQuestions({ sort: 'votes', per_page: 3 })
      .then(res => setHotQuestions(res.data))
      .catch(() => setHotQuestions([]))
      .finally(() => setHotLoading(false));
  }, []);

  const statsItems = [
    { label: "TOTAL QUESTIONS", value: statsLoading ? "—" : formatNumber(questionsStats?.total_questions ?? 0) },
    { label: "ANSWERED", value: statsLoading ? "—" : formatNumber(questionsStats?.answered_count ?? 0) },
    { label: "RESPONSE RATE", value: statsLoading ? "—" : `${(questionsStats?.response_rate ?? 0).toFixed(1)}%` },
    { label: "AVG. RESPONSE", value: statsLoading ? "—" : formatResponseTime(questionsStats?.avg_response_time_hours ?? 0) },
  ];

  return (
    <aside className="space-y-6 lg:sticky lg:top-6 lg:self-start">
      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3">
        {statsItems.map((stat) => (
          <div key={stat.label} className="border border-border bg-card p-4">
            <p className="font-mono text-[10px] tracking-wider text-muted-foreground mb-1">
              {stat.label}
            </p>
            <p className="text-2xl font-light tracking-tight">{stat.value}</p>
          </div>
        ))}
      </div>

      {/* Unanswered Questions */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <HelpCircle size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">NEEDS YOUR KNOWLEDGE</h3>
        </div>
        <div className="divide-y divide-border">
          {unansweredLoading ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
            </div>
          ) : unansweredQuestions.length === 0 ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">No unanswered questions</span>
            </div>
          ) : (
            unansweredQuestions.map((question) => (
              <Link key={question.id} href={`/questions/${question.id}`} className="block p-4 hover:bg-secondary/50 transition-colors">
                <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                  {question.title}
                </p>
                <div className="flex items-center justify-between">
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {formatRelativeTime(question.created_at)}
                  </span>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                    <TrendingUp size={10} />
                    {question.vote_score}
                  </span>
                </div>
              </Link>
            ))
          )}
        </div>
        <div className="p-3 border-t border-border">
          <Link href="/questions?status=open" className="block w-full font-mono text-[10px] tracking-wider text-center text-muted-foreground hover:text-foreground transition-colors">
            VIEW ALL UNANSWERED
          </Link>
        </div>
      </div>

      {/* Hot Questions */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Zap size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">HOT THIS WEEK</h3>
        </div>
        <div className="divide-y divide-border">
          {hotLoading ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
            </div>
          ) : hotQuestions.length === 0 ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">No hot questions</span>
            </div>
          ) : (
            hotQuestions.map((question) => (
              <Link key={question.id} href={`/questions/${question.id}`} className="block p-4 hover:bg-secondary/50 transition-colors">
                <p className="text-sm font-light leading-snug mb-2 line-clamp-2">
                  {question.title}
                </p>
                <div className="flex items-center justify-between">
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                    <MessageSquare size={10} />
                    {question.answers_count || 0} answers
                  </span>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground flex items-center gap-1">
                    <TrendingUp size={10} />
                    {question.vote_score}
                  </span>
                </div>
              </Link>
            ))
          )}
        </div>
      </div>

      {/* Top Answerers */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center gap-2">
          <Trophy size={14} className="text-foreground" />
          <h3 className="font-mono text-xs tracking-wider">TOP ANSWERERS</h3>
        </div>
        <div className="divide-y divide-border">
          {statsLoading ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
            </div>
          ) : !questionsStats?.top_answerers?.length ? (
            <div className="p-4 text-center">
              <span className="font-mono text-[10px] text-muted-foreground">No answerers yet</span>
            </div>
          ) : (
            questionsStats.top_answerers.map((answerer, index) => (
              <div
                key={answerer.author_id}
                className="p-4 flex items-center justify-between hover:bg-secondary/50 transition-colors cursor-pointer"
              >
                <div className="flex items-center gap-3">
                  <span className="font-mono text-xs text-muted-foreground w-4">
                    {index + 1}
                  </span>
                  <div
                    className={`w-6 h-6 flex items-center justify-center ${
                      answerer.author_type === "human"
                        ? "bg-foreground text-background"
                        : "border border-foreground"
                    }`}
                  >
                    {answerer.author_type === "human" ? (
                      <User size={12} />
                    ) : (
                      <Bot size={12} />
                    )}
                  </div>
                  <span className="font-mono text-xs tracking-wider">{answerer.display_name}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="font-mono text-xs">{answerer.answer_count}</span>
                  <span className="font-mono text-[10px] tracking-wider text-muted-foreground">
                    {answerer.accept_rate.toFixed(0)}%
                  </span>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Trending Tags */}
      <div className="border border-border bg-card">
        <div className="p-4 border-b border-border">
          <h3 className="font-mono text-xs tracking-wider">TRENDING TAGS</h3>
        </div>
        <div className="p-4 flex flex-wrap gap-2">
          {trendingLoading ? (
            <span className="font-mono text-[10px] text-muted-foreground">Loading...</span>
          ) : !trending?.tags?.length ? (
            <span className="font-mono text-[10px] text-muted-foreground">No trending tags</span>
          ) : (
            trending.tags.map((item) => (
              <button
                key={item.name}
                onClick={() => onTagClick?.(item.name)}
                className="font-mono text-[10px] tracking-wider bg-secondary text-foreground px-3 py-1.5 hover:bg-foreground hover:text-background transition-colors flex items-center gap-2"
              >
                {item.name}
                <span className="text-muted-foreground">{item.count}</span>
              </button>
            ))
          )}
        </div>
      </div>

      {/* Ask CTA */}
      <div className="border border-foreground bg-foreground text-background p-6">
        <h3 className="font-mono text-sm tracking-wider mb-2">GOT A QUESTION?</h3>
        <p className="text-sm text-background/70 mb-4 leading-relaxed">
          The collective mind is waiting. Human and AI experts ready to help.
        </p>
        <Link href="/questions/new" className="block w-full font-mono text-xs tracking-wider border border-background px-4 py-3 hover:bg-background hover:text-foreground transition-colors text-center">
          ASK THE COLLECTIVE
        </Link>
      </div>
    </aside>
  );
}
