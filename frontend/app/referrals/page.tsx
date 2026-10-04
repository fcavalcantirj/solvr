'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { Header } from '@/components/header';
import { useAuth } from '@/hooks/use-auth';
import { api } from '@/lib/api';
import type { APIReferralResponse } from '@/lib/api-types';
import { Copy, Check, ExternalLink } from 'lucide-react';
import { CAPTION } from '@/components/page/caption';
import { PageHeading } from '@/components/page/page-header';
import { PageSection, SECTION } from '@/components/page/page-section';
import { INK_BUTTON, LINE_BUTTON } from '@/components/page/controls';
import { cn } from '@/lib/utils';

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
      <main className="pt-16 pb-16">
        <PageHeading
          title="REFERRALS"
          lede="Share Solvr with your network and track your referrals."
        />

        {/* Skeleton Loading */}
        {showSkeleton && (
          <div aria-busy="true">
            <div className={SECTION}>
              <div className="h-8 w-48 bg-muted animate-pulse" />
              <div className="h-28 w-4/5 bg-muted animate-pulse" />
            </div>
            <div className={SECTION}>
              <div className="h-8 w-24 bg-muted animate-pulse" />
              <div className="h-20 w-24 bg-muted animate-pulse" />
            </div>
            <div className={SECTION}>
              <div className="h-8 w-32 bg-muted animate-pulse" />
              <div className="flex gap-3">
                <div className="h-11 w-36 bg-muted animate-pulse" />
                <div className="h-11 w-48 bg-muted animate-pulse" />
              </div>
            </div>
          </div>
        )}

        {/* Error State */}
        {!showSkeleton && fetchError && (
          <div className="border-t border-border px-4 py-16 sm:px-6 lg:px-12">
            <p className="mb-8 text-3xl font-light tracking-[-0.025em] text-red-700 dark:text-red-400">{fetchError}</p>
            <button
              onClick={handleRetry}
              className={INK_BUTTON}
            >
              RETRY
            </button>
          </div>
        )}

        {/* Success State */}
        {!showSkeleton && !fetchError && referral && (
          <div>
            {/* Referral Code: the page's thing, set big */}
            <PageSection heading="YOUR REFERRAL CODE">
              <span
                className="block text-[clamp(3.25rem,11vw,10rem)] font-light leading-none tracking-[-0.02em] text-foreground tabular-nums [overflow-wrap:anywhere]"
                data-testid="referral-code"
              >
                {referral.referral_code}
              </span>
              <button
                onClick={handleCopyCode}
                aria-label="Copy referral code"
                className={cn(LINE_BUTTON, "mt-8")}
              >
                {copiedCode ? (
                  <>
                    <Check />
                    Copied!
                  </>
                ) : (
                  <>
                    <Copy />
                    COPY CODE
                  </>
                )}
              </button>
            </PageSection>

            {/* Stats */}
            <PageSection heading="STATS">
              <div className="flex flex-col">
                <span
                  className="text-[clamp(5rem,12vw,11rem)] font-light leading-none tracking-[-0.06em] tabular-nums"
                  data-testid="referral-count"
                >
                  {referral.referral_count}
                </span>
                <span className={cn(CAPTION, "mt-5")}>
                  successful referral{referral.referral_count !== 1 ? 's' : ''}
                </span>
              </div>
            </PageSection>

            {/* Share Section */}
            <PageSection heading="SHARE">
              <div className="space-y-8">
                {/* Tweet link */}
                <a
                  href={tweetLink}
                  target="_blank"
                  rel="noopener noreferrer"
                  data-testid="tweet-link"
                  className={INK_BUTTON}
                >
                  <ExternalLink />
                  SHARE ON X
                </a>

                {/* Copy referral link */}
                <div className="border-t border-border pt-8">
                  <p className="mb-5 text-sm text-muted-foreground [overflow-wrap:anywhere]">
                    Your referral link:{' '}
                    <span className="font-mono text-xs text-foreground">{referralUrl}</span>
                  </p>
                  <button
                    onClick={handleCopyLink}
                    aria-label="Copy referral link"
                    className={LINE_BUTTON}
                  >
                    {copiedLink ? (
                      <>
                        <Check />
                        Copied!
                      </>
                    ) : (
                      <>
                        <Copy />
                        COPY REFERRAL LINK
                      </>
                    )}
                  </button>
                </div>
              </div>
            </PageSection>
          </div>
        )}
      </main>
    </div>
  );
}
