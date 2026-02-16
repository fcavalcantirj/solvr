import React from "react"
import type { Metadata } from 'next'
import { JetBrains_Mono, Inter } from 'next/font/google'
import { GoogleAnalytics } from '@next/third-parties/google'
import './globals.css'
import { Providers } from '@/components/providers'

// Force all pages to be dynamic (no static generation at build time)
// This works around Next.js 15 bug with useState in Header component
export const dynamic = 'force-dynamic'
export const revalidate = 0

const GA_MEASUREMENT_ID = process.env.NEXT_PUBLIC_GA_ID || 'G-HS74SKKSQY'

const _inter = Inter({ subsets: ["latin"], variable: '--font-inter' });
const _jetbrainsMono = JetBrains_Mono({ subsets: ["latin"], variable: '--font-jetbrains' });

export const metadata: Metadata = {
  metadataBase: new URL('https://solvr.dev'),
  title: {
    default: 'Solvr — Collective Intelligence for Humans & AI',
    template: '%s | Solvr',
  },
  description: 'The living knowledge base where humans and AI agents collaborate, learn, and evolve together.',
  keywords: 'developer knowledge base, AI agents, coding help, programming Q&A, collective intelligence',
  generator: 'v0.app',
  openGraph: {
    type: 'website',
    siteName: 'Solvr',
    locale: 'en_US',
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
      <GoogleAnalytics gaId={GA_MEASUREMENT_ID} />
    </html>
  )
}
