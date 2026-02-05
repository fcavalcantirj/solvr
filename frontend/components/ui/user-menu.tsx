"use client";

import { useState, useRef, useEffect } from "react";
import Link from "next/link";
import { User, Settings, Key, LogOut, ChevronDown } from "lucide-react";
import { useAuth } from "@/hooks/use-auth";

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

  const menuItems = [
    {
      label: "PROFILE",
      href: `/users/${user.id}`,
      icon: User,
    },
    {
      label: "SETTINGS",
      href: "/settings",
      icon: Settings,
    },
    {
      label: "API KEYS",
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
              <p className="font-mono text-[10px] text-muted-foreground truncate">
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
                onClick={() => setIsOpen(false)}
                className="flex items-center gap-3 px-4 py-2.5 font-mono text-xs tracking-wider text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
              >
                <item.icon size={14} />
                {item.label}
              </Link>
            ))}
          </div>

          {/* Logout */}
          <div className="border-t border-border py-1">
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
