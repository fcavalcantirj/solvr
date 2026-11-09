import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { ImageResponse } from 'next/og';

// ogCard draws a page's own link-preview picture (SEO audit 2026-11-08, item 7): before it,
// every one of the 500 post and blog pages previewed with the same site card. The card is
// deterministic, 1200×630, black on paper like the site: a kicker (what the page is), the
// title, up to four tags and the site name. It is drawn on the server by next/og with the
// bundled Inter face (assets/fonts/, traced into the standalone build by next.config.mjs).

export const OG_WIDTH = 1200;
export const OG_HEIGHT = 630;

export interface OgCardContent {
  // What the page is, set small above the title: "POST", "BLOG".
  kicker: string;
  title: string;
  tags?: string[] | null;
  // The line under the title: the author, a date, a reading time.
  byline?: string;
}

let fontData: Promise<ArrayBuffer> | undefined;
function inter(): Promise<ArrayBuffer> {
  fontData ??= readFile(path.join(process.cwd(), 'assets', 'fonts', 'Inter-SemiBold.ttf')).then(
    (b) => b.buffer.slice(b.byteOffset, b.byteOffset + b.byteLength) as ArrayBuffer
  );
  return fontData;
}

// A title that would not fit is cut at a word and finished with an ellipsis.
export function fitTitle(title: string, max = 110): string {
  const t = title.trim();
  if (t.length <= max) return t;
  const cut = t.slice(0, max).replace(/\s+\S*$/, '');
  return `${cut}…`;
}

export async function ogCard(content: OgCardContent): Promise<ImageResponse> {
  const title = fitTitle(content.title);
  const tags = (content.tags ?? []).filter(Boolean).slice(0, 4);
  const size = title.length > 70 ? 54 : title.length > 40 ? 64 : 76;
  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          padding: '64px 72px',
          background: '#f7f5f0',
          color: '#111111',
          fontFamily: 'Inter',
        }}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 26, letterSpacing: 4 }}>
          <span>{content.kicker.toUpperCase()}</span>
          <span>SOLVR_</span>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 28 }}>
          <div style={{ fontSize: size, lineHeight: 1.08, letterSpacing: -2, display: 'flex' }}>{title}</div>
          {content.byline ? <div style={{ fontSize: 28, color: '#555555', display: 'flex' }}>{content.byline}</div> : null}
        </div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-end', fontSize: 24 }}>
          <div style={{ display: 'flex', gap: 18 }}>
            {tags.map((tag) => (
              <span key={tag} style={{ border: '2px solid #111111', padding: '6px 14px', letterSpacing: 2 }}>
                {tag.toUpperCase()}
              </span>
            ))}
          </div>
          <span style={{ letterSpacing: 2 }}>solvr.dev</span>
        </div>
      </div>
    ),
    {
      width: OG_WIDTH,
      height: OG_HEIGHT,
      fonts: [{ name: 'Inter', data: await inter(), weight: 600, style: 'normal' }],
      headers: { 'Cache-Control': 'public, max-age=86400, s-maxage=86400, stale-while-revalidate=604800' },
    }
  );
}
