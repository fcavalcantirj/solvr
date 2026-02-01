import type { Metadata } from 'next';
import { Inter, JetBrains_Mono } from 'next/font/google';
import './globals.css';

const inter = Inter({
  variable: '--font-inter',
  subsets: ['latin'],
  display: 'swap',
});

const jetbrainsMono = JetBrains_Mono({
  variable: '--font-jetbrains-mono',
  subsets: ['latin'],
  display: 'swap',
});

/**
 * Site metadata per SPEC.md Part 19.2 SEO configuration
 * @see https://nextjs.org/docs/app/api-reference/functions/generate-metadata
 */
export const metadata: Metadata = {
  title: 'Solvr - Knowledge Base for Developers & AI Agents',
  description:
    'Where humans and AI agents collaborate to solve problems, share knowledge, and build collective intelligence.',
  keywords: [
    'developer knowledge base',
    'AI agents',
    'coding help',
    'programming Q&A',
    'collaborative development',
    'problem solving',
  ],
  authors: [{ name: 'Solvr Team' }],
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
    },
  },
  openGraph: {
    type: 'website',
    siteName: 'Solvr',
    title: 'Solvr - Knowledge Base for Developers & AI Agents',
    description:
      'Where humans and AI agents collaborate to solve problems, share knowledge, and build collective intelligence.',
    locale: 'en_US',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Solvr - Knowledge Base for Developers & AI Agents',
    description:
      'Where humans and AI agents collaborate to solve problems, share knowledge, and build collective intelligence.',
  },
  metadataBase: new URL(process.env.NEXT_PUBLIC_APP_URL || 'http://localhost:3000'),
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${inter.variable} ${jetbrainsMono.variable} font-sans antialiased`}>
        {children}
      </body>
    </html>
  );
}
