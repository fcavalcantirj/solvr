"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";
import { Settings, Key, Bell, AlertTriangle, Plus } from "lucide-react";
import Link from "next/link";

export default function APIKeysPage() {
  const router = useRouter();
  const { isAuthenticated, isLoading } = useAuth();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading) {
    return (
      <main className="min-h-screen bg-background flex items-center justify-center">
        <div className="w-8 h-8 border-2 border-muted-foreground/30 border-t-foreground rounded-full animate-spin" />
      </main>
    );
  }

  if (!isAuthenticated) {
    return null;
  }

  const menuItems = [
    { label: "PROFILE", href: "/settings", icon: Settings },
    { label: "API KEYS", href: "/settings/api-keys", icon: Key, active: true },
    { label: "NOTIFICATIONS", href: "/settings/notifications", icon: Bell },
    { label: "DANGER ZONE", href: "/settings/danger", icon: AlertTriangle },
  ];

  return (
    <main className="min-h-screen bg-background pt-24 pb-16">
      <div className="max-w-5xl mx-auto px-6 lg:px-12">
        <div className="flex gap-12">
          {/* Sidebar */}
          <nav className="w-48 space-y-1">
            {menuItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 px-4 py-2.5 font-mono text-xs tracking-wider transition-colors ${
                  item.active
                    ? "bg-secondary text-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
                }`}
              >
                <item.icon size={14} />
                {item.label}
              </Link>
            ))}
          </nav>

          {/* Content */}
          <div className="flex-1">
            <div className="border border-border p-8">
              <div className="flex items-center justify-between mb-8">
                <div>
                  <h1 className="font-mono text-xl tracking-tight mb-2">API Keys</h1>
                  <p className="font-mono text-sm text-muted-foreground">
                    Manage your API keys for accessing the Solvr API
                  </p>
                </div>
                <button className="flex items-center gap-2 font-mono text-xs tracking-wider bg-foreground text-background px-4 py-2.5 hover:bg-foreground/90 transition-colors cursor-pointer">
                  <Plus size={14} />
                  CREATE KEY
                </button>
              </div>

              <div className="border border-dashed border-border p-12 text-center">
                <Key size={32} className="mx-auto mb-4 text-muted-foreground" />
                <p className="font-mono text-sm text-muted-foreground mb-2">
                  No API keys yet
                </p>
                <p className="font-mono text-xs text-muted-foreground">
                  Create your first API key to start using the Solvr API
                </p>
              </div>

              <div className="mt-8 pt-8 border-t border-border">
                <p className="font-mono text-xs text-muted-foreground">
                  API key management coming soon. For now, use the API docs to learn about authentication.
                </p>
                <Link
                  href="/api-docs"
                  className="inline-block mt-4 font-mono text-xs tracking-wider text-foreground hover:underline"
                >
                  View API Documentation →
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
