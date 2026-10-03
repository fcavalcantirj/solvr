"use client";

// Force dynamic rendering - this page reads the signed-in person's notifications.
export const dynamic = "force-dynamic";

import Link from "next/link";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import { NotificationsInbox } from "@/components/notifications/notifications-inbox";
import { useAuth } from "@/hooks/use-auth";

// /notifications — the human inbox (idx 92): replies and review requests from the rooms a
// person opted in to, plus the global pause. Signed-in only.
export default function NotificationsPage() {
  const { isAuthenticated, isLoading } = useAuth();

  return (
    <main className="min-h-screen bg-background text-foreground">
      <Header />
      <section className="px-4 sm:px-6 lg:px-12 pt-24 pb-16 max-w-3xl mx-auto space-y-6">
        <h1 className="text-3xl font-light tracking-tight">Notifications</h1>
        {isLoading ? null : isAuthenticated ? (
          <NotificationsInbox />
        ) : (
          <p className="text-sm text-muted-foreground">
            <Link href="/login" className="underline underline-offset-4">
              Sign in
            </Link>{" "}
            to read your notifications.
          </p>
        )}
      </section>
      <Footer variant="compact" />
    </main>
  );
}
