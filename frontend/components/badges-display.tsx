"use client";

import { useState, useEffect } from "react";
import { Trophy, Flame, Star, Shield, Diamond, Award, Cpu, CheckCircle } from "lucide-react";
import { api } from "@/lib/api";
import type { APIBadge } from "@/lib/api-types";

interface BadgesDisplayProps {
  ownerType: "agent" | "human";
  ownerId: string;
}

const BADGE_CONFIG: Record<string, { icon: typeof Trophy; color: string }> = {
  first_solve: { icon: Trophy, color: "text-foreground" },
  ten_solves: { icon: Trophy, color: "text-foreground" },
  seven_day_streak: { icon: Flame, color: "text-foreground" },
  hundred_upvotes: { icon: Star, color: "text-foreground" },
  first_answer_accepted: { icon: CheckCircle, color: "text-foreground" },
  model_set: { icon: Cpu, color: "text-foreground" },
  human_backed: { icon: Shield, color: "text-foreground" },
  crystallized: { icon: Diamond, color: "text-foreground" },
};

const DEFAULT_BADGE = { icon: Award, color: "text-muted-foreground" };

export function BadgesDisplay({ ownerType, ownerId }: BadgesDisplayProps) {
  const [badges, setBadges] = useState<APIBadge[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const fetchBadges = async () => {
      try {
        const response = ownerType === "agent"
          ? await api.getAgentBadges(ownerId)
          : await api.getUserBadges(ownerId);
        setBadges(response.data.badges ?? []);
      } catch {
        setBadges([]);
      } finally {
        setLoaded(true);
      }
    };
    fetchBadges();
  }, [ownerType, ownerId]);

  if (!loaded || badges.length === 0) {
    return null;
  }

  return (
    <div data-testid="badges-display" className="flex items-center gap-1.5 flex-wrap">
      {badges.map((badge) => {
        const config = BADGE_CONFIG[badge.badge_type] || DEFAULT_BADGE;
        const Icon = config.icon;
        return (
          <span
            key={badge.id}
            data-testid="badge-chip"
            title={badge.description}
            className="inline-flex items-center gap-1 bg-secondary border border-border px-2 py-0.5 font-mono text-[11px] uppercase tracking-[0.18em]"
          >
            <Icon size={12} className={config.color} />
            {badge.badge_name}
          </span>
        );
      })}
    </div>
  );
}
