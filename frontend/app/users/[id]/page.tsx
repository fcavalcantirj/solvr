"use client";

import { useParams } from "next/navigation";
import { User, AlertCircle, Loader2 } from "lucide-react";
import Link from "next/link";
import { useUser } from "@/hooks/use-user";
import { ProfileHeader } from "@/components/users/profile-header";
import { UserPostsList } from "@/components/users/user-posts-list";

export default function UserProfilePage() {
  const params = useParams();
  const userId = params.id as string;
  const { user, posts, loading, error } = useUser(userId);

  // Loading state
  if (loading) {
    return (
      <main className="min-h-screen bg-background pt-24 pb-16">
        <div className="max-w-4xl mx-auto px-6 lg:px-12">
          <div className="flex flex-col items-center justify-center py-24">
            <Loader2 size={32} className="animate-spin text-muted-foreground mb-4" />
            <p className="font-mono text-sm text-muted-foreground">Loading profile...</p>
          </div>
        </div>
      </main>
    );
  }

  // Error state
  if (error) {
    return (
      <main className="min-h-screen bg-background pt-24 pb-16">
        <div className="max-w-4xl mx-auto px-6 lg:px-12">
          <div className="border border-destructive/50 bg-destructive/5 p-8 text-center">
            <AlertCircle size={32} className="mx-auto mb-4 text-destructive" />
            <h2 className="font-mono text-lg mb-2">Failed to load profile</h2>
            <p className="font-mono text-sm text-muted-foreground mb-6">{error}</p>
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

  // Not found state
  if (!user) {
    return (
      <main className="min-h-screen bg-background pt-24 pb-16">
        <div className="max-w-4xl mx-auto px-6 lg:px-12">
          <div className="border border-border p-12 text-center">
            <User size={32} className="mx-auto mb-4 text-muted-foreground" />
            <h2 className="font-mono text-lg mb-2">User not found</h2>
            <p className="font-mono text-sm text-muted-foreground mb-6">
              The user you&apos;re looking for doesn&apos;t exist.
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

  return (
    <main className="min-h-screen bg-background pt-24 pb-16">
      <div className="max-w-4xl mx-auto px-6 lg:px-12">
        {/* Profile Header */}
        <ProfileHeader user={user} />

        {/* User's Posts */}
        <section>
          <h2 className="font-mono text-lg tracking-tight mb-4">Posts</h2>
          <UserPostsList posts={posts} />
        </section>
      </div>
    </main>
  );
}
