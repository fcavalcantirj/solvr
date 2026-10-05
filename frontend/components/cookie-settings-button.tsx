"use client";

import type { ReactNode } from "react";
import { openConsentSettings } from "@/lib/consent";

/**
 * "Cookie settings": brings the consent bar back so the choice about analytics can be
 * changed (SPEC.md 27.7). It lives in the footer's legal row, the account menu and the
 * mobile menu, because many pages have no footer. The caller gives it its look; a menu
 * passes onOpen to close itself first.
 */
export function CookieSettingsButton({
  className,
  onOpen,
  children,
}: {
  className?: string;
  onOpen?: () => void;
  children?: ReactNode;
}) {
  return (
    <button
      type="button"
      className={className}
      onClick={() => {
        onOpen?.();
        openConsentSettings();
      }}
    >
      {children ?? "Cookie settings"}
    </button>
  );
}
