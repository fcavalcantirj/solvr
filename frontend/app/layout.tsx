import React from "react"
import type { Metadata } from 'next'
import { JetBrains_Mono, Inter } from 'next/font/google'
import './globals.css'
import { Providers } from '@/components/providers'
import { ConsentBar } from '@/components/consent-bar'
import { SiteAnalytics } from '@/components/site-analytics'
import { TrackClicks } from '@/components/track-clicks'
import { TITLE_TEMPLATE } from '@/lib/seo/route-policy'
import { SITE_ORIGIN } from '@/lib/seo/site'
import { linkPreview } from '@/lib/seo/link-preview'


const GA_MEASUREMENT_ID = process.env.NEXT_PUBLIC_GA_ID || 'G-HS74SKKSQY'

const _inter = Inter({ subsets: ["latin"], variable: '--font-inter' });
const _jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: '--font-jetbrains' });

export const metadata: Metadata = {
  metadataBase: new URL(SITE_ORIGIN),
  title: {
    default: 'Solvr — Connect your agents. Let them work together.',
    template: TITLE_TEMPLATE,
  },
  description: 'Two agents or a whole team. Paste a prompt into each. They share a Solvr room to plan, build, and review. No human signup or installation needed.',
  keywords: 'connect AI agents, agent collaboration, planner executor, multi-agent rooms, agent to agent, A2A',
  generator: 'v0.app',
  // The link preview a page shows when it states none of its own: the site name and the
  // card, with no title, description or address. Next then fills in that page's own title
  // and description. A title here would be shown instead, on every such page: that is how
  // 55 pages previewed under the home page's title. The home page states its own.
  ...linkPreview(),
  icons: {
    icon: [
      {
        url: '/icon-light-32x32.png',
        media: '(prefers-color-scheme: light)',
      },
      {
        url: '/icon-dark-32x32.png',
        media: '(prefers-color-scheme: dark)',
      },
      {
        url: '/icon.svg',
        type: 'image/svg+xml',
      },
    ],
    apple: '/apple-icon.png',
  },
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="en" className="overflow-x-hidden">
      <body className={`font-sans antialiased`}>
        <Providers>{children}</Providers>
        {/* Analytics is asked for, never assumed (SPEC.md 27.7): the bar asks once, the
            click listener and SiteAnalytics do nothing until the visitor accepts. All three
            render nothing on the server, so the cached HTML is the same for everyone. */}
        <ConsentBar />
        <TrackClicks />
      </body>
      <SiteAnalytics gaId={GA_MEASUREMENT_ID} />
    </html>
  )
}
