"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { User, Settings, Key, LogOut, ChevronDown, Bot, HardDrive, PenLine, LayoutDashboard, Bell, Cookie } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";
import { CookieSettingsButton } from "@/components/cookie-settings-button";
import { trackNav } from "@/lib/track-attrs";

interface UserMenuProps {
  className?: string;
}

export function UserMenu({ className = "" }: UserMenuProps) {
  const { user, logout } = useAuth();
  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    }

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  // Close menu on escape key
  useEffect(() => {
    function handleEscape(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    }

    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, []);

  if (!user) return null;

  // `item` is the stable id the site's click listener reports (SPEC.md 27.7), never the label.
  const menuItems = [
    {
      label: "PROFILE",
      item: "profile",
      href: `/users/${user.id}`,
      icon: User,
    },
    {
      label: "DASHBOARD",
      item: "dashboard",
      href: "/dashboard",
      icon: LayoutDashboard,
    },
    {
      label: "MY AGENTS",
      item: "my_agents",
      href: "/settings/agents",
      icon: Bot,
    },
    {
      label: "MY PINS",
      item: "my_pins",
      href: "/pins",
      icon: HardDrive,
    },
    {
      label: "WRITE BLOG",
      item: "write_blog",
      href: "/blog/create",
      icon: PenLine,
    },
    {
      label: "NOTIFICATIONS",
      item: "notifications",
      href: "/notifications",
      icon: Bell,
    },
    {
      label: "SETTINGS",
      item: "settings",
      href: "/settings",
      icon: Settings,
    },
    {
      label: "API KEYS",
      item: "api_keys",
      href: "/settings/api-keys",
      icon: Key,
    },
  ];

  return (
    <div ref={menuRef} className={`relative ${className}`}>
      {/* Trigger button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 hover:opacity-80 transition-opacity"
        aria-expanded={isOpen}
        aria-haspopup="true"
      >
        <div className="w-8 h-8 bg-foreground text-background flex items-center justify-center">
          <User size={14} />
        </div>
        <span className="font-mono text-xs tracking-wider hidden sm:inline">
          {user.displayName}
        </span>
        <ChevronDown
          size={14}
          className={`transition-transform ${isOpen ? "rotate-180" : ""}`}
        />
      </button>

      {/* Dropdown menu */}
      {isOpen && (
        <div className="absolute right-0 mt-2 w-48 bg-background border border-border shadow-lg z-50">
          {/* User info header */}
          <div className="px-4 py-3 border-b border-border">
            <p className="font-mono text-xs tracking-wider truncate">
              {user.displayName}
            </p>
            {user.email && (
              <p className="font-mono text-[11px] text-muted-foreground truncate">
                {user.email}
              </p>
            )}
          </div>

          {/* Menu items */}
          <div className="py-1">
            {menuItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                {...trackNav(item.item, "account_menu")}
                onClick={() => setIsOpen(false)}
                className="flex items-center gap-3 px-4 py-2.5 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              >
                <item.icon size={14} />
                {item.label}
              </Link>
            ))}
          </div>

          {/* Cookie settings (many pages have no footer to carry it) and logout */}
          <div className="border-t border-border py-1">
            <CookieSettingsButton
              onOpen={() => setIsOpen(false)}
              className="flex items-center gap-3 w-full px-4 py-2.5 font-mono text-xs uppercase tracking-wider text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
            >
              <Cookie size={14} />
              Cookie settings
            </CookieSettingsButton>
            <button
              onClick={() => {
                logout();
                setIsOpen(false);
              }}
              className="flex items-center gap-3 w-full px-4 py-2.5 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
            >
              <LogOut size={14} />
              LOG OUT
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
