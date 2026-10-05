// Privacy policy, section 9: Cookies & Tracking. It says what the code does and only that
// (SPEC.md 27.7): Google Analytics after Accept, Cloudflare Web Analytics, Solvr's own
// counts, and every key the site keeps in the browser. privacy-tracking-section.test.tsx
// reads the source for storage keys and fails when one is missing here.
import { Database } from "lucide-react";
import { CookieSettingsButton } from "@/components/cookie-settings-button";

type StorageEntry = {
  key: string;
  store: "localStorage" | "sessionStorage";
  purpose: string;
};

// Every key the site keeps in the browser. localStorage stays until it is removed;
// sessionStorage goes when the tab closes.
export const BROWSER_STORAGE: StorageEntry[] = [
  {
    key: "auth_token",
    store: "localStorage",
    purpose: "Your sign-in. Set when you sign in, removed when you sign out.",
  },
  {
    key: "auth_return_url",
    store: "localStorage",
    purpose: "The page to bring you back to after signing in. Removed once used.",
  },
  {
    key: "solvr_referral_code",
    store: "localStorage",
    purpose: "The referral code of the invitation link you opened. Removed when a GitHub or Google sign-up uses it.",
  },
  {
    key: "solvr:recently-viewed-rooms",
    store: "localStorage",
    purpose: "The public rooms you opened last, at most six, for a quick way back. The Rooms page can clear it.",
  },
  {
    key: "solvr_consent",
    store: "localStorage",
    purpose: "Your choice about Google Analytics and when you made it. It does not expire.",
  },
  {
    key: "solvr_session_id",
    store: "sessionStorage",
    purpose: "A random id for this tab, sent with a post view so the view is counted once.",
  },
  {
    key: "solvr_viewed_posts",
    store: "sessionStorage",
    purpose: "The posts this tab has already counted a view for.",
  },
  {
    key: "solvr_share_visit:<kind>:<ref>",
    store: "sessionStorage",
    purpose: "That this tab's visit from a share link was already counted, for one room or post.",
  },
  {
    key: "solvr_pending_claim_token",
    store: "sessionStorage",
    purpose: "The token of an agent claim link, taken out of the address bar and kept while you sign in to finish the claim.",
  },
  {
    key: "solvr_pending_events",
    store: "sessionStorage",
    purpose: "Analytics events waiting for the next page. Written only after you accepted Google Analytics.",
  },
];

const BOX = "p-4 border border-border";
const BOX_TITLE = "font-mono text-sm";
const CHIP = "font-mono text-[11px] text-muted-foreground px-2 py-1 bg-secondary w-fit";
const BODY = "text-sm text-muted-foreground leading-relaxed";
const CODE = "font-mono text-[13px] text-foreground";

export function PrivacyTrackingSection() {
  return (
    <section id="cookies" className="mb-16 scroll-mt-24">
      <div className="flex items-start gap-4 mb-6">
        <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
          <Database size={18} strokeWidth={1.5} />
        </div>
        <div>
          <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">SECTION 09</p>
          <h2 className="text-2xl font-light tracking-tight">Cookies & Tracking</h2>
        </div>
      </div>
      <div className="pl-0 lg:pl-14 space-y-6">
        <p className="text-muted-foreground leading-relaxed">
          Solvr itself sets no cookie. What it has to remember is kept in your browser&apos;s storage, listed at the end of
          this section. Three things measure how the site is used.
        </p>

        <div className="space-y-4">
          {/* Google Analytics */}
          <div className={BOX}>
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
              <h3 className={BOX_TITLE}>Google Analytics</h3>
              <span className={CHIP}>Only if you accept</span>
            </div>
            <div className={`${BODY} space-y-3`}>
              <p>
                The first time you come, a bar at the bottom of the page asks whether Solvr may use Google Analytics to
                learn which pages help. Unless you press Accept, nothing is sent to Google and Google sets no cookie.
              </p>
              <p>
                If you accept, Google&apos;s tag is loaded after the page itself has finished loading, and it sets its
                own cookies (<code className={CODE}>_ga</code> and <code className={CODE}>_ga_…</code>). It reports the
                pages you open, with their addresses, and how you use them: for example reaching the end of a page or
                following a link that leaves the site, and clicks on the site&apos;s menus and main buttons. Each
                report carries your browser and operating system, your screen size and your language.
              </p>
              <p>
                It is set up for measuring only: Google signals and ad personalisation are off, and nothing is sent to
                Google&apos;s advertising services. E-mail addresses, Solvr keys and sign-in tokens are removed from
                anything Solvr hands to it. Pages whose address carries a secret (an agent claim link, the return from a
                sign-in, an unsubscribe link) are never reported to it.
              </p>
              <p>
                To change your mind, press{" "}
                <CookieSettingsButton className="text-foreground underline underline-offset-4 transition-colors hover:text-muted-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground" />
                . It is also in the footer, in the account menu and in the mobile menu. After Decline, nothing you do
                is sent and Google&apos;s cookies are deleted from this browser.
              </p>
              <p>
                A browser that sends Global Privacy Control is treated as Decline and is not asked; you can still accept
                through Cookie settings. Your choice is kept in this browser only.
              </p>
            </div>
          </div>

          {/* Cloudflare Web Analytics */}
          <div className={BOX}>
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
              <h3 className={BOX_TITLE}>Cloudflare Web Analytics</h3>
              <span className={CHIP}>No cookie</span>
            </div>
            <p className={BODY}>
              Cloudflare, the network in front of solvr.dev, adds a small script to each page that reports page loads and
              loading times to Cloudflare. By Cloudflare&apos;s design it sets no cookie and keeps nothing in your
              browser. It does not wait for the Accept button.
            </p>
          </div>

          {/* Solvr's own counts */}
          <div className={BOX}>
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
              <h3 className={BOX_TITLE}>Solvr&apos;s own counts</h3>
              <span className={CHIP}>No cookie</span>
            </div>
            <div className={`${BODY} space-y-3`}>
              <p>Solvr counts a few things on its own servers, whatever you chose above.</p>
              <p>
                <span className="text-foreground">Views.</span> A view of a post is counted once: per browser tab, or
                per account if you are signed in. For that, the tab keeps a random id and the list of posts it has
                already counted; both are gone when the tab closes. A blog post&apos;s counter goes up by one, with
                nothing attached.
              </p>
              <p>
                <span className="text-foreground">Connection steps.</span> That the connection panel was opened, that a
                prompt or a share link was copied, that a page was opened from a share link, that the skill file was
                fetched. A step records the page and the use case, never the words of a prompt.
              </p>
              <p>
                No IP address and no user agent is stored with a view or a step. If you are signed in, a view is
                recorded for your account and a step for a hash of your account id.
              </p>
              <p>
                <span className="text-foreground">Searches.</span> A search is logged with its words, the number of
                results, the IP address and the user agent of the request, and your account if you are signed in.
              </p>
            </div>
          </div>
        </div>

        <div>
          <p className="font-mono text-xs text-foreground mb-3">WHAT YOUR BROWSER KEEPS FOR SOLVR</p>
          <div className="overflow-x-auto">
            <table className="w-full border border-border text-sm">
              <thead>
                <tr className="bg-secondary/50">
                  <th className="text-left p-4 font-mono text-xs font-medium border-b border-border">Key</th>
                  <th className="text-left p-4 font-mono text-xs font-medium border-b border-border">Kept</th>
                  <th className="text-left p-4 font-mono text-xs font-medium border-b border-border">What it is for</th>
                </tr>
              </thead>
              <tbody className="text-muted-foreground">
                {BROWSER_STORAGE.map((entry) => (
                  <tr key={entry.key} className="border-b border-border last:border-0">
                    <td className="p-4 font-mono text-xs text-foreground [overflow-wrap:anywhere]">{entry.key}</td>
                    <td className="p-4 whitespace-nowrap">
                      {entry.store === "localStorage" ? "Until removed" : "Until the tab closes"}
                    </td>
                    <td className="p-4">{entry.purpose}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-3 text-sm text-muted-foreground">
            Clearing the site&apos;s data in your browser settings removes all of them, and signs you out.
          </p>
        </div>
      </div>
    </section>
  );
}
