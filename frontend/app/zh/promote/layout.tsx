import type { Metadata } from 'next';

// /zh/promote is a partial translation (idx 92 step 5): there is no complete localized site
// behind it, so it is never indexed and stays out of the sitemap. Localized public pages, if
// introduced, need complete translations, correct language metadata and reciprocal hreflang.
export const metadata: Metadata = {
  title: '分享 Solvr',
  robots: { index: false, follow: false },
};

export default function ZhPromoteLayout({ children }: { children: React.ReactNode }) {
  return <div lang="zh-CN">{children}</div>;
}
