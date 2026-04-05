"use client";

import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";

interface PostButtonProps {
  href: string;
  label: string;
}

export function PostButton({ href, label }: PostButtonProps) {
  const router = useRouter();
  const { isAuthenticated } = useAuth();

  const handleClick = () => {
    if (isAuthenticated) {
      router.push(href);
    } else {
      router.push(`/login?next=${href}`);
    }
  };

  return (
    <button
      onClick={handleClick}
      className="font-mono text-xs tracking-wider bg-foreground text-background px-6 py-3 hover:bg-foreground/90 transition-colors"
    >
      {label}
    </button>
  );
}
