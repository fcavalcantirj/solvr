"use client";

import { User, Award, FileText, MessageSquare } from "lucide-react";
import type { UserData } from "@/hooks/use-user";

export interface ProfileHeaderProps {
  user: UserData;
}

export function ProfileHeader({ user }: ProfileHeaderProps) {
  return (
    <div className="border border-border p-8 mb-8">
      <div className="flex items-start gap-6">
        {/* Avatar */}
        <div className="w-20 h-20 bg-foreground text-background flex items-center justify-center overflow-hidden">
          {user.avatarUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={user.avatarUrl}
              alt={user.displayName}
              className="w-full h-full object-cover"
            />
          ) : (
            <User size={32} />
          )}
        </div>

        {/* User info */}
        <div className="flex-1">
          <h1 className="font-mono text-2xl tracking-tight mb-1">{user.displayName}</h1>
          <p className="font-mono text-sm text-muted-foreground mb-3">@{user.username}</p>
          {user.bio && (
            <p className="font-mono text-sm text-muted-foreground">{user.bio}</p>
          )}
        </div>
      </div>

      {/* Stats row */}
      <div className="mt-6 pt-6 border-t border-border grid grid-cols-3 gap-4">
        <div className="text-center">
          <div className="flex items-center justify-center gap-2 mb-1">
            <FileText size={14} className="text-muted-foreground" />
            <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">Posts</span>
          </div>
          <p className="font-mono text-2xl">{user.stats.postsCreated}</p>
        </div>
        <div className="text-center">
          <div className="flex items-center justify-center gap-2 mb-1">
            <MessageSquare size={14} className="text-muted-foreground" />
            <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">Contributions</span>
          </div>
          <p className="font-mono text-2xl">{user.stats.contributions}</p>
        </div>
        <div className="text-center">
          <div className="flex items-center justify-center gap-2 mb-1">
            <Award size={14} className="text-muted-foreground" />
            <span className="font-mono text-xs tracking-wider text-muted-foreground uppercase">Rep</span>
          </div>
          <p className="font-mono text-2xl">{user.stats.reputation}</p>
        </div>
      </div>
    </div>
  );
}
