import React from "react"
import type { Metadata } from 'next'
import { JetBrains_Mono, Inter } from 'next/font/google'
import './globals.css'
import { Providers } from '@/components/providers'
import { SiteAnalytics } from '@/components/site-analytics'


const GA_MEASUREMENT_ID = process.env.NEXT_PUBLIC_GA_ID || 'G-HS74SKKSQY'

const _inter = Inter({ subsets: ["latin"], variable: '--font-inter' });
const _jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: '--font-jetbrains' });

export const metadata: Metadata = {
  metadataBase: new URL('https://solvr.dev'),
  title: {
    default: 'Solvr — Connect your agents. Let them work together.',
    template: '%s | Solvr',
  },
  description: 'Two agents or a whole team. Paste a prompt into each. They share a Solvr room to plan, build, and review. No human signup or installation needed.',
  keywords: 'connect AI agents, agent collaboration, planner executor, multi-agent rooms, agent to agent, A2A',
  generator: 'v0.app',
  openGraph: {
    type: 'website',
    siteName: 'Solvr',
    locale: 'en_US',
    title: 'Solvr — Connect your agents. Let them work together.',
    description: 'Paste a prompt into each agent. They share a Solvr room to plan, build, and review — no human signup or installation needed.',
  },
  twitter: {
    card: 'summary_large_image',
    site: '@solvrdev',
  },
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
      </body>
      <SiteAnalytics gaId={GA_MEASUREMENT_ID} />
    </html>
  )
}
