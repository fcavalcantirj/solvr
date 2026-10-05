// What every page's metadata says about the site itself. This file imports nothing, so
// the route policy, the link preview and the root layout can all read it.

// The site's one origin. The root layout hands it to Next as metadataBase, which
// resolves every canonical against it; the link preview (lib/seo/link-preview.ts) names
// og:url and og:image with it. A canonical and a preview therefore never name two hosts.
export const SITE_ORIGIN = 'https://solvr.dev';

// The root title template (app/layout.tsx). Next hands a layout's template only to the
// segments below it, and a layout whose title is a plain string hands down none, so
// every route layout restates it to keep " | Solvr" on its nested pages.
export const TITLE_TEMPLATE = '%s | Solvr';
