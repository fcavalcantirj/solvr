"use client";

import { useParams } from "next/navigation";
import { User, Calendar, Award, MessageSquare, Lightbulb, HelpCircle } from "lucide-react";
import Link from "next/link";

export default function UserProfilePage() {
  const params = useParams();
  const userId = params.id as string;

  return (
    <main className="min-h-screen bg-background pt-24 pb-16">
      <div className="max-w-4xl mx-auto px-6 lg:px-12">
        {/* Profile Header */}
        <div className="border border-border p-8 mb-8">
          <div className="flex items-start gap-6">
            <div className="w-20 h-20 bg-foreground text-background flex items-center justify-center">
              <User size={32} />
            </div>
            <div className="flex-1">
              <h1 className="font-mono text-2xl tracking-tight mb-2">User Profile</h1>
              <p className="font-mono text-xs text-muted-foreground mb-4">
                ID: {userId}
              </p>
              <div className="flex items-center gap-6 text-muted-foreground">
                <span className="flex items-center gap-2 font-mono text-xs">
                  <Calendar size={14} />
                  Member since --
                </span>
                <span className="flex items-center gap-2 font-mono text-xs">
                  <Award size={14} />
                  -- reputation
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4 mb-8">
          <div className="border border-border p-6">
            <div className="flex items-center gap-2 mb-2">
              <HelpCircle size={16} className="text-muted-foreground" />
              <span className="font-mono text-xs tracking-wider text-muted-foreground">QUESTIONS</span>
            </div>
            <p className="font-mono text-2xl">--</p>
          </div>
          <div className="border border-border p-6">
            <div className="flex items-center gap-2 mb-2">
              <MessageSquare size={16} className="text-muted-foreground" />
              <span className="font-mono text-xs tracking-wider text-muted-foreground">ANSWERS</span>
            </div>
            <p className="font-mono text-2xl">--</p>
          </div>
          <div className="border border-border p-6">
            <div className="flex items-center gap-2 mb-2">
              <Lightbulb size={16} className="text-muted-foreground" />
              <span className="font-mono text-xs tracking-wider text-muted-foreground">IDEAS</span>
            </div>
            <p className="font-mono text-2xl">--</p>
          </div>
        </div>

        {/* Content Placeholder */}
        <div className="border border-dashed border-border p-12 text-center">
          <User size={32} className="mx-auto mb-4 text-muted-foreground" />
          <p className="font-mono text-sm text-muted-foreground mb-2">
            User profiles coming soon
          </p>
          <p className="font-mono text-xs text-muted-foreground mb-6">
            Full profile pages with activity history, reputation, and contributions are in development
          </p>
          <Link
            href="/feed"
            className="inline-block font-mono text-xs tracking-wider bg-foreground text-background px-6 py-2.5 hover:bg-foreground/90 transition-colors"
          >
            BACK TO FEED
          </Link>
        </div>
      </div>
    </main>
  );
}
