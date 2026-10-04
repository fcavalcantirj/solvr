"use client";

// Force dynamic rendering - this page imports Header which uses client-side state
export const dynamic = 'force-dynamic';


import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/hooks/use-auth";
import { useProfileEdit } from "@/hooks/use-profile-edit";
import { useAuthMethods } from "@/hooks/use-auth-methods";
import { SettingsLayout } from "@/components/settings/settings-layout";
import { Loader2, Check, AlertCircle, Trash2, AlertTriangle } from "lucide-react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { CAPTION } from "@/components/page/caption";
import { PageSection } from "@/components/page/page-section";
import { DANGER_BUTTON, FIELD, INK_BUTTON, LINE_BUTTON } from "@/components/page/controls";

// A read-only ledger row: its caption on the left, its value on the right.
const ROW = "grid gap-2 border-b border-border py-5 sm:grid-cols-[12rem_minmax(0,1fr)] sm:items-baseline sm:gap-6";

export default function SettingsPage() {
  const router = useRouter();
  const { user } = useAuth();
  const { saving, error, success, updateProfile, clearStatus } = useProfileEdit();
  const { authMethods, loading: authMethodsLoading } = useAuthMethods();

  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [hasChanges, setHasChanges] = useState(false);

  // Delete account state
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

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

  const handleDeleteAccount = async () => {
    setIsDeleting(true);
    setDeleteError(null);

    try {
      await api.deleteMe();
      // Redirect to landing page after successful deletion
      router.push("/");
    } catch (err) {
      setDeleteError("Failed to delete account. Please try again.");
      setIsDeleting(false);
    }
  };

  return (
    <SettingsLayout>
      {/* Profile Information: the person, set big */}
      <PageSection heading="PROFILE INFORMATION">
        <div className="flex min-w-0 flex-col gap-6 sm:flex-row sm:items-end sm:gap-8">
          <div className="flex size-16 shrink-0 items-center justify-center bg-foreground text-background sm:size-20">
            <span className="font-mono text-lg">
              {user?.displayName?.slice(0, 2).toUpperCase() || "??"}
            </span>
          </div>
          <div className="min-w-0">
            <h3 className="text-[clamp(2.75rem,6vw,6rem)] font-light leading-none tracking-[-0.055em] [overflow-wrap:anywhere]">
              {user?.displayName || "Unknown User"}
            </h3>
            <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2">
              <p className="font-mono text-[11px] text-muted-foreground">
                @{user?.id?.slice(0, 8) || "unknown"}
              </p>
              <p className={CAPTION}>
                Member since {new Date().toLocaleDateString("en-US", { month: "short", year: "numeric" })}
              </p>
            </div>
          </div>
        </div>
      </PageSection>

      {/* Edit Profile Form */}
      <PageSection heading="EDIT PROFILE">
        {error && (
          <div className="mb-6 flex items-center gap-2 border-y border-destructive py-3 text-destructive">
            <AlertCircle size={16} />
            <span className="text-sm">{error}</span>
          </div>
        )}

        {success && (
          <div className="mb-6 flex items-center gap-2 border-y border-border py-3 text-green-700 dark:text-green-400">
            <Check size={16} />
            <span className="text-sm">Profile updated successfully</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="max-w-2xl space-y-8">
          <div>
            <label htmlFor="settings-display-name" className={cn(CAPTION, "mb-3 block")}>
              DISPLAY NAME
            </label>
            <input
              id="settings-display-name"
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              maxLength={50}
              className={FIELD}
              placeholder="Your display name"
            />
            <p className={cn(CAPTION, "mt-2 normal-case tracking-normal")}>
              {displayName.length}/50 characters
            </p>
          </div>

          <div>
            <label htmlFor="settings-bio" className={cn(CAPTION, "mb-3 block")}>
              BIO
            </label>
            <textarea
              id="settings-bio"
              value={bio}
              onChange={(e) => setBio(e.target.value)}
              maxLength={500}
              rows={4}
              className={cn(FIELD, "resize-none")}
              placeholder="Tell us about yourself..."
            />
            <p className={cn(CAPTION, "mt-2 normal-case tracking-normal")}>
              {bio.length}/500 characters
            </p>
          </div>

          <div>
            <button
              type="submit"
              disabled={!hasChanges || saving}
              className={INK_BUTTON}
            >
              {saving && <Loader2 className="animate-spin" />}
              {saving ? "SAVING..." : "SAVE CHANGES"}
            </button>
          </div>
        </form>
      </PageSection>

      {/* Account Details (Read-only) */}
      <PageSection heading="ACCOUNT DETAILS">
        <dl className="border-t border-border">
          <div className={ROW}>
            <dt className={CAPTION}>
              EMAIL
            </dt>
            <dd className="text-base [overflow-wrap:anywhere] sm:text-right">
              {user?.email || "Not set"}
            </dd>
          </div>
          <div className={ROW}>
            <dt className={CAPTION}>
              ACCOUNT TYPE
            </dt>
            <dd className="text-base uppercase sm:text-right">
              {user?.type || "Unknown"}
            </dd>
          </div>
          <div className={ROW}>
            <dt className={CAPTION}>
              LINKED ACCOUNTS
            </dt>
            <dd className="sm:text-right">
              {authMethodsLoading ? (
                <span className="text-sm text-muted-foreground">Loading...</span>
              ) : authMethods.length === 0 ? (
                <span className="text-sm text-muted-foreground">No authentication methods found</span>
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
                      <div key={index} className="text-base text-foreground">
                        • {providerName} - Linked {linkedDate}
                      </div>
                    );
                  })}
                </div>
              )}
            </dd>
          </div>
          <div className={ROW}>
            <dt className={CAPTION}>
              USER ID
            </dt>
            <dd className="font-mono text-xs text-muted-foreground [overflow-wrap:anywhere] sm:text-right">
              {user?.id || "Unknown"}
            </dd>
          </div>
        </dl>
      </PageSection>

      {/* Danger Zone */}
      <PageSection
        heading="DANGER ZONE"
        headingClassName="text-destructive"
        intro="Deleting your account is permanent and cannot be undone."
      >
        <div className="border-t border-destructive pt-8">
          {deleteError && (
            <div className="mb-6 flex items-center gap-2 text-destructive">
              <AlertCircle size={16} />
              <span className="text-sm">{deleteError}</span>
            </div>
          )}

          <AlertDialog>
            <AlertDialogTrigger asChild>
              <button
                type="button"
                className={DANGER_BUTTON}
                disabled={isDeleting}
              >
                {isDeleting ? (
                  <>
                    <Loader2 className="animate-spin" />
                    DELETING...
                  </>
                ) : (
                  <>
                    <Trash2 />
                    DELETE MY ACCOUNT
                  </>
                )}
              </button>
            </AlertDialogTrigger>
            <AlertDialogContent className="border-foreground p-8 shadow-none">
              <AlertDialogHeader>
                <AlertDialogTitle className="flex items-center gap-3 text-2xl font-light tracking-[-0.025em]">
                  <AlertTriangle className="text-destructive" size={20} />
                  Are you sure?
                </AlertDialogTitle>
                <AlertDialogDescription className="text-sm leading-relaxed">
                  This will permanently delete your account. Your posts and contributions
                  will remain visible but anonymized. This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel className={cn(LINE_BUTTON, "h-auto")}>
                  Cancel
                </AlertDialogCancel>
                <AlertDialogAction
                  onClick={handleDeleteAccount}
                  className={cn(DANGER_BUTTON, "h-auto bg-destructive text-destructive-foreground hover:bg-destructive/90")}
                >
                  Yes, delete my account
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </PageSection>
    </SettingsLayout>
  );
}
