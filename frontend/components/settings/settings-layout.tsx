"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/hooks/use-auth";
import { Header } from "@/components/header";
import { PageHeading } from "@/components/page/page-header";
import { useEffect } from "react";

const navItems = [
  { href: "/settings", label: "PROFILE" },
  { href: "/settings/agents", label: "MY AGENTS" },
  { href: "/settings/api-keys", label: "API KEYS" },
];

interface SettingsLayoutProps {
  children: React.ReactNode;
}

// Settings opens like /status: a plain heading and its purpose, then one square row
// of its three pages, then the page's own sections in the left-heading grammar.
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
        <main className="pt-16">
          <div className="flex items-center px-4 py-24 sm:px-6 lg:px-12">
            <Loader2 className="w-5 h-5 animate-spin text-muted-foreground" />
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
      <main className="pt-16 pb-16">
        <PageHeading
          title="SETTINGS"
          lede="Manage your profile and account preferences."
        />

        {/* The three settings pages */}
        <nav className="mx-4 border-t border-border py-4 sm:mx-6 lg:mx-12">
          <div className="grid grid-cols-3 border border-border divide-x divide-border sm:inline-grid">
            {navItems.map((item) => {
              const isActive = pathname === item.href;
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  aria-current={isActive ? "page" : undefined}
                  className={cn(
                    "px-3 py-3 text-center font-mono text-[11px] uppercase tracking-[0.18em] whitespace-nowrap transition-colors focus-visible:relative focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground sm:px-6",
                    isActive
                      ? "bg-foreground text-background"
                      : "text-muted-foreground hover:text-foreground hover:bg-secondary"
                  )}
                >
                  {item.label}
                </Link>
              );
            })}
          </div>
        </nav>

        {/* Main Content */}
        <div className="min-w-0">
          {children}
        </div>
      </main>
    </div>
  );
}
