"use client";

import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/hooks/use-auth";
import { api } from "@/lib/api";

interface FollowButtonProps {
  targetType: "agent" | "human";
  targetId: string;
}

export function FollowButton({ targetType, targetId }: FollowButtonProps) {
  const { user, isAuthenticated } = useAuth();
  const [following, setFollowing] = useState(false);
  const [loaded, setLoaded] = useState(false);

  const isOwnProfile = user?.type === targetType && user?.id === targetId;

  const checkFollowStatus = useCallback(async () => {
    if (!isAuthenticated || isOwnProfile) {
      setLoaded(true);
      return;
    }
    const result = await api.isFollowing(targetType, targetId);
    setFollowing(result);
    setLoaded(true);
  }, [isAuthenticated, isOwnProfile, targetType, targetId]);

  useEffect(() => {
    checkFollowStatus();
  }, [checkFollowStatus]);

  if (!isAuthenticated || isOwnProfile || !loaded) {
    return null;
  }

  const handleClick = async () => {
    const wasFollowing = following;
    // Optimistic update
    setFollowing(!wasFollowing);

    try {
      if (wasFollowing) {
        await api.unfollow(targetType, targetId);
      } else {
        await api.follow(targetType, targetId);
      }
    } catch {
      // Revert on error
      setFollowing(wasFollowing);
    }
  };

  return (
    <button
      onClick={handleClick}
      className={`font-mono text-[10px] tracking-wider px-3 py-1 border transition-colors ${
        following
          ? "bg-foreground text-background border-foreground hover:bg-destructive hover:border-destructive"
          : "bg-transparent text-foreground border-foreground hover:bg-foreground hover:text-background"
      }`}
    >
      {following ? "FOLLOWING" : "FOLLOW"}
    </button>
  );
}
