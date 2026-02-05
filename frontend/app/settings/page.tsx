"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";
import { Settings, Key, Bell, AlertTriangle } from "lucide-react";
import Link from "next/link";

export default function SettingsPage() {
  const router = useRouter();
  const { isAuthenticated, isLoading, user } = useAuth();

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
    { label: "PROFILE", href: "/settings", icon: Settings, active: true },
    { label: "API KEYS", href: "/settings/api-keys", icon: Key },
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
              <h1 className="font-mono text-xl tracking-tight mb-2">Profile Settings</h1>
              <p className="font-mono text-sm text-muted-foreground mb-8">
                Manage your account settings and preferences
              </p>

              <div className="space-y-6">
                <div>
                  <label className="font-mono text-xs tracking-wider text-muted-foreground">
                    DISPLAY NAME
                  </label>
                  <p className="font-mono text-sm mt-1">{user?.displayName}</p>
                </div>
                <div>
                  <label className="font-mono text-xs tracking-wider text-muted-foreground">
                    EMAIL
                  </label>
                  <p className="font-mono text-sm mt-1">{user?.email || "Not set"}</p>
                </div>
                <div>
                  <label className="font-mono text-xs tracking-wider text-muted-foreground">
                    ACCOUNT TYPE
                  </label>
                  <p className="font-mono text-sm mt-1 capitalize">{user?.type}</p>
                </div>
              </div>

              <div className="mt-8 pt-8 border-t border-border">
                <p className="font-mono text-xs text-muted-foreground">
                  Profile editing coming soon...
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
