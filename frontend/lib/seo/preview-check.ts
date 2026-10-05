import type { Metadata } from 'next';
import { SITE_ORIGIN } from './site';

// Test support: what a link-preview bot will read from a page's metadata, and whether it
// is right. No page imports this file. It works out what it expects on its own (the
// title template is spelled out here on purpose) so a mistake in lib/seo/link-preview.ts
// cannot agree with itself.

const TEMPLATE_SUFFIX = ' | Solvr';

type Loose = Record<string, unknown>;

const list = (value: unknown): unknown[] => (value == null ? [] : Array.isArray(value) ? value : [value]);

const imageUrl = (image: unknown): string =>
  typeof image === 'string' || image instanceof URL ? String(image) : String((image as Loose | undefined)?.url ?? '');

// tagTitle is the text of the page's <title> tag: Next puts a plain title, or a layout's
// default title, into the root template, and shows an absolute title as written.
export function tagTitle(title: Metadata['title']): string {
  if (!title) return '';
  if (typeof title === 'string') return `${title}${TEMPLATE_SUFFIX}`;
  if ('absolute' in title && title.absolute) return title.absolute;
  return 'default' in title && title.default ? `${title.default}${TEMPLATE_SUFFIX}` : '';
}

export interface Preview {
  title: string;
  description: string | undefined;
  url: string | undefined;
  type: string | undefined;
  image: string;
  images: Loose[];
  twitterCard: string | undefined;
  twitterTitle: string;
  twitterImage: string;
}

// readPreview flattens the Open Graph and Twitter objects of a page's metadata.
export function readPreview(metadata: Metadata): Preview {
  const og = (metadata.openGraph ?? {}) as Loose;
  const tw = (metadata.twitter ?? {}) as Loose;
  const images = list(og.images) as Loose[];
  return {
    title: String(og.title ?? ''),
    description: og.description == null ? undefined : String(og.description),
    url: og.url == null ? undefined : String(og.url),
    type: og.type == null ? undefined : String(og.type),
    image: imageUrl(images[0]),
    images,
    twitterCard: tw.card == null ? undefined : String(tw.card),
    twitterTitle: String(tw.title ?? ''),
    twitterImage: imageUrl(list(tw.images)[0]),
  };
}

export interface PreviewExpectation {
  // The home page's title (the site default). Only the home page may preview under it.
  homeTitle: string;
  // True for the home page itself.
  home?: boolean;
}

// previewProblems lists what is wrong with a page's link preview; an empty list means a
// bot gets an absolute picture, the page's canonical as its address (none when the page
// has no canonical), the page's own title as its title tag shows it, and a large-image
// Twitter card with the same picture.
export function previewProblems(metadata: Metadata, { homeTitle, home = false }: PreviewExpectation): string[] {
  if (!metadata.openGraph) return ['the page states no Open Graph preview'];
  if (!metadata.twitter) return ['the page states no Twitter preview'];
  const problems: string[] = [];
  const preview = readPreview(metadata);

  if (preview.images.length !== 1) problems.push(`og:image is stated ${preview.images.length} times`);
  if (!/^https?:\/\/[^/]+\/\S+$/.test(preview.image)) problems.push(`og:image "${preview.image}" is not an absolute address`);

  const canonical = metadata.alternates?.canonical;
  if (canonical == null) {
    if (preview.url !== undefined) problems.push(`og:url "${preview.url}" on a page with no canonical`);
  } else {
    const want = new URL(String(canonical), SITE_ORIGIN).href;
    if (preview.url !== want) problems.push(`og:url "${preview.url}" is not the canonical "${want}"`);
  }

  const want = tagTitle(metadata.title);
  if (!want) problems.push('the page states no title');
  if (preview.title !== want) problems.push(`og:title "${preview.title}" is not the title tag "${want}"`);
  if (home && preview.title !== homeTitle) problems.push(`the home page previews as "${preview.title}"`);
  if (!home && preview.title === homeTitle) problems.push('og:title is the home page title');

  if (preview.twitterCard !== 'summary_large_image') problems.push(`twitter:card is "${preview.twitterCard}"`);
  if (preview.twitterImage !== preview.image) problems.push(`twitter:image "${preview.twitterImage}" is not og:image`);
  if (preview.twitterTitle !== preview.title) problems.push(`twitter:title "${preview.twitterTitle}" is not og:title`);
  return problems;
}
