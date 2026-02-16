"use client";

import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';

interface AuthRequiredModalProps {
  isOpen: boolean;
  onClose: () => void;
  message?: string;
  returnUrl?: string;
}

export function AuthRequiredModal({
  isOpen,
  onClose,
  message = "Login required to continue",
  returnUrl
}: AuthRequiredModalProps) {
  const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

  const handleLogin = (provider: 'google' | 'github') => {
    // Store return URL
    if (returnUrl) {
      localStorage.setItem('auth_return_url', returnUrl);
    } else {
      localStorage.setItem('auth_return_url', window.location.pathname);
    }

    // Redirect to OAuth
    window.location.href = `${API_BASE_URL}/v1/auth/${provider}`;
  };

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Authentication Required</DialogTitle>
          <DialogDescription>{message}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3 mt-4">
          <Button onClick={() => handleLogin('google')} variant="outline">
            Continue with Google
          </Button>
          <Button onClick={() => handleLogin('github')} variant="outline">
            Continue with GitHub
          </Button>
          <Button onClick={onClose} variant="ghost">
            Cancel
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
