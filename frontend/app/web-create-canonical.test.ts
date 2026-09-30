import { describe, it, expect } from 'vitest';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, resolve, sep } from 'node:path';
import { canonicalPath, config } from '@/middleware';

// idx 52 step 2: the web client creates canonical posts, with no type to choose.
// The canonical composer is /posts/new (components/posts/post-composer.tsx, whose
// own test pins that it sends no type). The pre-redesign typed form
// (components/new-post/new-post-form.tsx) is still in the tree until the legacy
// code goes all at once, so every page that renders it must be unreachable: the
// middleware has to redirect its route before it renders.

const FRONTEND = resolve(__dirname, '..');
const TYPED_FORM = '@/components/new-post/new-post-form';

function sourceFiles(dir: string): string[] {
  const out: string[] = [];
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name.startsWith('.')) continue;
    const full = join(dir, name);
    if (statSync(full).isDirectory()) {
      out.push(...sourceFiles(full));
    } else if (/\.(ts|tsx)$/.test(name) && !/\.test\.(ts|tsx)$/.test(name)) {
      out.push(full);
    }
  }
  return out;
}

// app/problems/new/page.tsx -> /problems/new ; route groups "(x)" add no segment.
function routeOf(pageFile: string): string {
  const segments = relative(join(FRONTEND, 'app'), pageFile)
    .split(sep)
    .slice(0, -1)
    .filter((s) => !(s.startsWith('(') && s.endsWith(')')));
  return '/' + segments.join('/');
}

function matcherCovers(route: string): boolean {
  return config.matcher.some((m) =>
    m.endsWith('/:path*') ? route.startsWith(m.slice(0, -'/:path*'.length) + '/') : m === route,
  );
}

describe('web post creation is canonical (idx 52)', () => {
  const pages = sourceFiles(join(FRONTEND, 'app')).filter((f) => /page\.tsx$/.test(f));
  const typedPages = pages.filter((f) => readFileSync(f, 'utf8').includes(TYPED_FORM));

  it('finds the pages that still hold the typed form (the check below is not vacuous)', () => {
    expect(typedPages.map(routeOf)).toContain('/new');
  });

  it('redirects every page that renders the typed form to the canonical composer before it renders', () => {
    const reachable = typedPages
      .map(routeOf)
      .filter((route) => canonicalPath(route) !== '/posts/new' || !matcherCovers(route));
    expect(reachable).toEqual([]);
  });

  it('links nothing in the app to the old /new composer', () => {
    const link = /(href\s*[=:]\s*\{?\s*|push\(\s*|replace\(\s*)["'`]\/new(\?[^"'`]*)?["'`]/;
    const offenders = ['app', 'components', 'hooks', 'lib']
      .flatMap((d) => sourceFiles(join(FRONTEND, d)))
      .filter((f) => link.test(readFileSync(f, 'utf8')))
      .map((f) => relative(FRONTEND, f));
    expect(offenders).toEqual([]);
  });
});
