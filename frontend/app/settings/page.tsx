"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';


import { useState, useEffect } from "react";
import { useAuth } from "@/hooks/use-auth";
import { useProfileEdit } from "@/hooks/use-profile-edit";
import { useAuthMethods } from "@/hooks/use-auth-methods";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { Button } from "@/components/ui/button";
import { Loader2, Check, AlertCircle, User } from "lucide-react";

export default function SettingsPage() {
  const { user } = useAuth();
  const { saving, error, success, updateProfile, clearStatus } = useProfileEdit();
  const { authMethods, loading: authMethodsLoading } = useAuthMethods();

  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [hasChanges, setHasChanges] = useState(false);

  // Initialize form with user data
  useEffect(() => {
    if (user) {
      setDisplayName(user.displayName || "");
      // Bio would come from user profile - for now we start empty
      setBio("");
    }
  }, [user]);

  // Track changes
  useEffect(() => {
    if (user) {
      const nameChanged = displayName !== (user.displayName || "");
      const bioChanged = bio !== "";
      setHasChanges(nameChanged || bioChanged);
    }
  }, [displayName, bio, user]);

  // Clear status after a delay
  useEffect(() => {
    if (success) {
      const timer = setTimeout(clearStatus, 3000);
      return () => clearTimeout(timer);
    }
  }, [success, clearStatus]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!hasChanges) return;

    const data: { display_name?: string; bio?: string } = {};
    if (displayName !== user?.displayName) {
      data.display_name = displayName;
    }
    if (bio) {
      data.bio = bio;
    }

    const success = await updateProfile(data);
    if (success) {
      // Optionally reload to refresh user context
      window.location.reload();
    }
  };

  return (
    <SettingsLayout>
      {/* Profile Information Card */}
      <div className="border border-border p-8 mb-6">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-6">
          PROFILE INFORMATION
        </h2>
        <div className="flex items-start gap-6">
          <div className="w-20 h-20 bg-foreground text-background flex items-center justify-center flex-shrink-0">
            <span className="font-mono text-2xl font-bold">
              {user?.displayName?.slice(0, 2).toUpperCase() || "??"}
            </span>
          </div>
          <div className="flex-1 min-w-0">
            <h3 className="font-mono text-2xl tracking-tight truncate">
              {user?.displayName || "Unknown User"}
            </h3>
            <p className="font-mono text-sm text-muted-foreground mt-1">
              @{user?.id?.slice(0, 8) || "unknown"}
            </p>
            <p className="font-mono text-xs text-muted-foreground mt-2">
              Member since {new Date().toLocaleDateString("en-US", { month: "short", year: "numeric" })}
            </p>
          </div>
        </div>
      </div>

      {/* Edit Profile Form */}
      <div className="border border-border p-8 mb-6">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-6">
          EDIT PROFILE
        </h2>

        {error && (
          <div className="flex items-center gap-2 bg-destructive/10 border border-destructive text-destructive px-4 py-3 mb-6">
            <AlertCircle size={16} />
            <span className="font-mono text-xs">{error}</span>
          </div>
        )}

        {success && (
          <div className="flex items-center gap-2 bg-emerald-500/10 border border-emerald-500 text-emerald-600 px-4 py-3 mb-6">
            <Check size={16} />
            <span className="font-mono text-xs">Profile updated successfully</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-6">
          <div>
            <label className="font-mono text-xs tracking-wider text-muted-foreground block mb-2">
              DISPLAY NAME
            </label>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              maxLength={50}
              className="w-full bg-secondary/50 border border-border px-4 py-3 font-mono text-sm focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
              placeholder="Your display name"
            />
            <p className="font-mono text-[10px] text-muted-foreground mt-1">
              {displayName.length}/50 characters
            </p>
          </div>

          <div>
            <label className="font-mono text-xs tracking-wider text-muted-foreground block mb-2">
              BIO
            </label>
            <textarea
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              maxLength={500}
              rows={4}
              className="w-full bg-secondary/50 border border-border px-4 py-3 font-mono text-sm resize-none focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
              placeholder="Tell us about yourself..."
            />
            <p className="font-mono text-[10px] text-muted-foreground mt-1">
              {bio.length}/500 characters
            </p>
          </div>

          <div className="flex justify-end">
            <Button
              type="submit"
              disabled={!hasChanges || saving}
              className="font-mono text-xs tracking-wider"
            >
              {saving && <Loader2 className="w-3 h-3 mr-2 animate-spin" />}
              {saving ? "SAVING..." : "SAVE CHANGES"}
            </Button>
          </div>
        </form>
      </div>

      {/* Account Details (Read-only) */}
      <div className="border border-border p-8">
        <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-6">
          ACCOUNT DETAILS
        </h2>
        <div className="space-y-4">
          <div className="flex items-center justify-between py-3 border-b border-border">
            <span className="font-mono text-xs tracking-wider text-muted-foreground">
              EMAIL
            </span>
            <span className="font-mono text-sm">
              {user?.email || "Not set"}
            </span>
          </div>
          <div className="flex items-center justify-between py-3 border-b border-border">
            <span className="font-mono text-xs tracking-wider text-muted-foreground">
              ACCOUNT TYPE
            </span>
            <span className="font-mono text-sm uppercase">
              {user?.type || "Unknown"}
            </span>
          </div>
          <div className="py-3 border-b border-border">
            <span className="font-mono text-xs tracking-wider text-muted-foreground block mb-3">
              LINKED ACCOUNTS
            </span>
            {authMethodsLoading ? (
              <span className="font-mono text-xs text-muted-foreground">Loading...</span>
            ) : authMethods.length === 0 ? (
              <span className="font-mono text-xs text-muted-foreground">No authentication methods found</span>
            ) : (
              <div className="space-y-2">
                {authMethods.map((method, index) => {
                  const providerName = method.provider === 'google' ? 'Google'
                    : method.provider === 'github' ? 'GitHub'
                    : method.provider === 'email' ? 'Email/Password'
                    : method.provider;

                  const linkedDate = new Date(method.linked_at).toLocaleDateString('en-US', {
                    year: 'numeric',
                    month: 'short',
                    day: 'numeric'
                  });

                  return (
                    <div key={index} className="font-mono text-sm text-foreground">
                      • {providerName} - Linked {linkedDate}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
          <div className="flex items-center justify-between py-3">
            <span className="font-mono text-xs tracking-wider text-muted-foreground">
              USER ID
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {user?.id || "Unknown"}
            </span>
          </div>
        </div>
      </div>
    </SettingsLayout>
  );
}
