'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Header } from '@/components/header';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import type { APIReferralResponse } from '@/lib/api-types';
import { Share2, Copy, Check, ExternalLink } from 'lucide-react';

export default function ReferralsPage() {
  const router = useRouter();
  const { user, isAuthenticated, isLoading } = useAuth();
  const [referral, setReferral] = useState<APIReferralResponse | null>(null);
  const [fetchLoading, setFetchLoading] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [copiedCode, setCopiedCode] = useState(false);
  const [copiedLink, setCopiedLink] = useState(false);

  useEffect(() => {
    if (isLoading) return;
    if (!isAuthenticated) {
      router.push('/login?next=/referrals');
      return;
    }
    setFetchLoading(true);
    api
      .getMyReferral()
      .then((data) => {
        setReferral(data);
      })
      .catch(() => {
        setFetchError('Failed to load referral data');
      })
      .finally(() => {
        setFetchLoading(false);
      });
  }, [isAuthenticated, isLoading, router]);

  const referralUrl = referral
    ? `https://solvr.dev/join?ref=${referral.referral_code}`
    : '';

  const tweetText = "I'm using @SolvrDev to solve programming problems faster. Join me:";
  const tweetLink = referral
    ? `https://twitter.com/intent/tweet?text=${encodeURIComponent(tweetText)}&url=${encodeURIComponent(referralUrl)}`
    : '#';

  const handleCopyCode = async () => {
    if (!referral) return;
    await navigator.clipboard.writeText(referral.referral_code);
    setCopiedCode(true);
    setTimeout(() => setCopiedCode(false), 2000);
  };

  const handleCopyLink = async () => {
    if (!referral) return;
    await navigator.clipboard.writeText(referralUrl);
    setCopiedLink(true);
    setTimeout(() => setCopiedLink(false), 2000);
  };

  const handleRetry = () => {
    if (!referral && !fetchLoading) {
      setFetchError(null);
      setFetchLoading(true);
      api
        .getMyReferral()
        .then((data) => {
          setReferral(data);
        })
        .catch(() => {
          setFetchError('Failed to load referral data');
        })
        .finally(() => {
          setFetchLoading(false);
        });
    }
  };

  const showSkeleton = isLoading || fetchLoading;

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-20">
        {/* Page Header */}
        <div className="border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-12">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-foreground flex items-center justify-center">
                <Share2 className="w-5 h-5 text-background" />
              </div>
              <span className="font-mono text-xs tracking-wider text-muted-foreground">
                REFERRAL DASHBOARD
              </span>
            </div>
            <h1 className="font-mono text-3xl sm:text-4xl md:text-5xl font-medium tracking-tight text-foreground">
              REFERRALS
            </h1>
            <p className="font-mono text-xs sm:text-sm text-muted-foreground mt-3 max-w-2xl">
              Share Solvr with your network and track your referrals.
            </p>
          </div>
        </div>

        {/* Content */}
        <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8">
          {/* Skeleton Loading */}
          {showSkeleton && (
            <div className="space-y-6">
              <div className="border border-border p-6 animate-pulse">
                <div className="h-4 bg-muted w-32 mb-4" />
                <div className="h-10 bg-muted w-64" />
              </div>
              <div className="border border-border p-6 animate-pulse">
                <div className="h-4 bg-muted w-24 mb-4" />
                <div className="h-8 bg-muted w-16" />
              </div>
              <div className="border border-border p-6 animate-pulse">
                <div className="h-4 bg-muted w-40 mb-4" />
                <div className="flex gap-3">
                  <div className="h-10 bg-muted w-32" />
                  <div className="h-10 bg-muted w-40" />
                </div>
              </div>
            </div>
          )}

          {/* Error State */}
          {!showSkeleton && fetchError && (
            <div className="border border-red-500 p-8 text-center">
              <p className="font-mono text-sm text-red-500 mb-4">{fetchError}</p>
              <button
                onClick={handleRetry}
                className="font-mono text-xs px-4 py-2 bg-foreground text-background hover:bg-foreground/90 transition-colors"
              >
                RETRY
              </button>
            </div>
          )}

          {/* Success State */}
          {!showSkeleton && !fetchError && referral && (
            <div className="space-y-6">
              {/* Referral Code Card */}
              <div className="border border-border p-6">
                <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
                  YOUR REFERRAL CODE
                </h2>
                <div className="flex items-center gap-4">
                  <span
                    className="font-mono text-2xl font-medium text-foreground tracking-widest"
                    data-testid="referral-code"
                  >
                    {referral.referral_code}
                  </span>
                  <button
                    onClick={handleCopyCode}
                    aria-label="Copy referral code"
                    className="flex items-center gap-2 font-mono text-xs px-3 py-2 border border-border hover:border-foreground transition-colors text-muted-foreground hover:text-foreground"
                  >
                    {copiedCode ? (
                      <>
                        <Check className="w-3.5 h-3.5" />
                        Copied!
                      </>
                    ) : (
                      <>
                        <Copy className="w-3.5 h-3.5" />
                        COPY CODE
                      </>
                    )}
                  </button>
                </div>
              </div>

              {/* Stats Card */}
              <div className="border border-border p-6">
                <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
                  STATS
                </h2>
                <div className="flex items-baseline gap-3">
                  <span
                    className="font-mono text-4xl font-medium text-emerald-500"
                    data-testid="referral-count"
                  >
                    {referral.referral_count}
                  </span>
                  <span className="font-mono text-sm text-muted-foreground">
                    successful referral{referral.referral_count !== 1 ? 's' : ''}
                  </span>
                </div>
              </div>

              {/* Share Section */}
              <div className="border border-border p-6">
                <h2 className="font-mono text-xs tracking-wider text-muted-foreground mb-4">
                  SHARE
                </h2>
                <div className="space-y-3">
                  {/* Tweet link */}
                  <a
                    href={tweetLink}
                    target="_blank"
                    rel="noopener noreferrer"
                    data-testid="tweet-link"
                    className="inline-flex items-center gap-2 font-mono text-xs px-4 py-2.5 bg-foreground text-background hover:bg-foreground/90 transition-colors"
                  >
                    <ExternalLink className="w-3.5 h-3.5" />
                    SHARE ON X
                  </a>

                  {/* Copy referral link */}
                  <div>
                    <p className="font-mono text-xs text-muted-foreground mb-2">
                      Your referral link:{' '}
                      <span className="text-foreground">{referralUrl}</span>
                    </p>
                    <button
                      onClick={handleCopyLink}
                      aria-label="Copy referral link"
                      className="inline-flex items-center gap-2 font-mono text-xs px-4 py-2.5 border border-border hover:border-foreground transition-colors text-muted-foreground hover:text-foreground"
                    >
                      {copiedLink ? (
                        <>
                          <Check className="w-3.5 h-3.5" />
                          Copied!
                        </>
                      ) : (
                        <>
                          <Copy className="w-3.5 h-3.5" />
                          COPY REFERRAL LINK
                        </>
                      )}
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
