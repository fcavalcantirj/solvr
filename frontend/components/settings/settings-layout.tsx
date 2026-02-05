"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Settings, Key, User, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/hooks/use-auth";
import { Header } from "@/components/header";
import { useEffect } from "react";

const navItems = [
  { href: "/settings", label: "PROFILE", icon: User },
  { href: "/settings/api-keys", label: "API KEYS", icon: Key },
];

interface SettingsLayoutProps {
  children: React.ReactNode;
}

export function SettingsLayout({ children }: SettingsLayoutProps) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, isLoading, isAuthenticated } = useAuth();

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push("/login");
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading) {
    return (
      <div className="min-h-screen bg-background">
        <Header />
        <main className="pt-20">
          <div className="flex items-center justify-center py-24">
            <Loader2 className="w-6 h-6 animate-spin text-muted-foreground" />
          </div>
        </main>
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return null; // Will redirect
  }

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8 sm:py-12">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-foreground flex items-center justify-center shrink-0">
                <Settings className="w-5 h-5 text-background" />
              </div>
              <span className="font-mono text-xs tracking-wider text-muted-foreground">
                ACCOUNT
              </span>
            </div>
            <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
              SETTINGS
            </h1>
            <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-xl">
              Manage your profile and account preferences.
            </p>
          </div>
        </div>

        {/* Content with Sidebar */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-12 py-8">
          <div className="flex flex-col lg:flex-row gap-8 lg:gap-12">
            {/* Sidebar Navigation */}
            <nav className="lg:w-48 flex-shrink-0">
              <div className="flex lg:flex-col gap-2 overflow-x-auto lg:overflow-visible pb-4 lg:pb-0 -mx-4 px-4 lg:mx-0 lg:px-0">
                {navItems.map((item) => {
                  const isActive = pathname === item.href;
                  const Icon = item.icon;
                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={cn(
                        "flex items-center gap-3 px-4 py-2.5 font-mono text-xs tracking-wider transition-colors whitespace-nowrap",
                        isActive
                          ? "bg-foreground text-background"
                          : "text-muted-foreground hover:text-foreground hover:bg-secondary"
                      )}
                    >
                      <Icon size={14} />
                      {item.label}
                    </Link>
                  );
                })}
              </div>
            </nav>

            {/* Main Content */}
            <div className="flex-1 min-w-0">
              {children}
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
