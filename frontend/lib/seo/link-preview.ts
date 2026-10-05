import type { Metadata } from 'next';
import { SITE_ORIGIN, TITLE_TEMPLATE } from './site';

// A link pasted into a chat or a feed is shown as a card: a title, a line, an address
// and a picture, read from the page's Open Graph and Twitter tags. Next gives a page that
// states any of those fields ONLY what it states: its object replaces the one the page
// would have inherited, picture and site name included. So every page builds both
// objects here, whole, and no page writes one by hand (link-preview.test.ts checks it).

// The card every page shows unless it has a picture of its own. Platforms keep a preview
// picture by its address, so a new picture needs a new file name (solvr-card-v2.png).
// This is the one place the name is written.
export const PREVIEW_CARD_PATH = '/og/solvr-card-v1.png';
// What the card says, for a reader who cannot see it.
export const PREVIEW_CARD_ALT = 'Connect your agents. Let them work together.';

const SITE_NAME = 'Solvr';
const LOCALE = 'en_US';
const TWITTER_SITE = '@solvrdev';

type OpenGraph = NonNullable<Metadata['openGraph']>;
type Twitter = NonNullable<Metadata['twitter']>;

export interface PreviewPage {
  // The page's own title, in the form it hands to Next: a plain title is shown under the
  // site template ("Docs" becomes "Docs | Solvr"), an absolute one as written. Left out,
  // the preview states no title, and Next fills in the title of the page that shows it.
  title?: string | { absolute: string };
  description?: string;
  // The page's canonical path ("/posts/abc"). A page with no canonical names no address.
  path?: string;
  type?: 'website' | 'article' | 'profile';
  // An article's dates and tags. They are stated only on an 'article'.
  article?: { publishedTime?: string; modifiedTime?: string; tags?: string[] | null };
  // The page's own picture (a blog post's cover image). Every other page shows the card.
  image?: string | null;
}

export interface LinkPreview {
  openGraph: OpenGraph;
  twitter: Twitter;
}

// shownTitle is the text of the page's title tag. The expression is the one Next applies
// to a title under a template, so the preview title and the title tag cannot differ.
function shownTitle(title: NonNullable<PreviewPage['title']>): string {
  return typeof title === 'string' ? TITLE_TEMPLATE.replace(/%s/g, title) : title.absolute;
}

// onSite names a path the way Next names a canonical: resolved against the one origin.
function onSite(path: string): string {
  return new URL(path, SITE_ORIGIN).href;
}

// ownPicture is the page's own picture as an absolute web address. It is undefined when
// the page has none a preview can show (nothing, or not an http or https address).
function ownPicture(image: PreviewPage['image']): string | undefined {
  const value = image?.trim();
  if (!value) return undefined;
  try {
    const url = new URL(value, SITE_ORIGIN);
    return url.protocol === 'https:' || url.protocol === 'http:' ? url.href : undefined;
  } catch {
    return undefined;
  }
}

function pictures(page: PreviewPage) {
  const own = ownPicture(page.image);
  if (!own) {
    return [{ url: onSite(PREVIEW_CARD_PATH), width: 1200, height: 630, alt: PREVIEW_CARD_ALT, type: 'image/png' }];
  }
  // The size and format of a page's own picture are not known here, so none is claimed.
  // It is described the way the page describes it: by the page's own title.
  const name = typeof page.title === 'string' ? page.title : page.title?.absolute;
  return [name ? { url: own, alt: name } : { url: own }];
}

// linkPreview builds a page's Open Graph and Twitter metadata from its title, its
// description and its canonical path. Spread it into the page's metadata:
//   { title, description, alternates: { canonical: path }, ...linkPreview({ title, description, path }) }
export function linkPreview(page: PreviewPage = {}): LinkPreview {
  const title = page.title ? shownTitle(page.title) : '';
  const text = {
    ...(title ? { title } : {}),
    ...(page.description ? { description: page.description } : {}),
  };
  const shared = {
    ...text,
    ...(page.path ? { url: onSite(page.path) } : {}),
    siteName: SITE_NAME,
    locale: LOCALE,
  };
  const openGraph: OpenGraph =
    page.type === 'article'
      ? { ...shared, type: 'article', ...page.article, images: pictures(page) }
      : { ...shared, type: page.type ?? 'website', images: pictures(page) };
  const twitter: Twitter = { card: 'summary_large_image', site: TWITTER_SITE, ...text, images: pictures(page) };
  return { openGraph, twitter };
}
