"use client";

import Link from "next/link";
import { AlertTriangle, Clock } from "lucide-react";

interface ModerationBannerProps {
  status: string;
  postId: string;
  postType: "problems" | "questions" | "ideas";
}

export function ModerationBanner({ status, postId, postType }: ModerationBannerProps) {
  const normalizedStatus = status.toUpperCase();

  if (normalizedStatus === "UNDER REVIEW") {
    return (
      <div className="mb-6 p-4 border border-yellow-500/30 bg-yellow-500/5">
        <div className="flex items-start gap-3">
          <Clock size={16} className="text-yellow-600 mt-0.5 flex-shrink-0" />
          <p className="font-mono text-xs text-yellow-700">
            This post is being reviewed by our moderation system. It will appear in the feed once approved.
          </p>
        </div>
      </div>
    );
  }

  if (normalizedStatus === "REJECTED") {
    return (
      <div className="mb-6 p-4 border border-red-500/30 bg-red-500/5">
        <div className="flex items-start gap-3">
          <AlertTriangle size={16} className="text-red-600 mt-0.5 flex-shrink-0" />
          <div>
            <p className="font-mono text-xs text-red-700">
              This post was rejected by moderation. You can edit and resubmit.
            </p>
            <Link
              href={`/${postType}/${postId}/edit`}
              className="inline-flex items-center mt-2 font-mono text-xs tracking-wider px-3 py-1.5 bg-red-600 text-white hover:bg-red-700 transition-colors"
            >
              Edit Post
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return null;
}
