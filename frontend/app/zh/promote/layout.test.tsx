import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';

import ZhPromoteLayout, { metadata } from './layout';
import { GET as sitemapCore } from '../../sitemap-core.xml/route';

// idx 92 step 5: a partial translation is never indexed. /zh/promote is a partly-Chinese
// page with no complete localized counterpart, so it is noindex, says which language it is
// in, and stays out of the sitemap until a complete localized site exists (then with
// reciprocal hreflang).
describe('/zh/promote is a partial translation', () => {
  it('asks crawlers not to index or follow it', () => {
    expect(metadata.robots).toEqual({ index: false, follow: false });
  });

  it('marks its content as Chinese', () => {
    render(
      <ZhPromoteLayout>
        <p>分享 Solvr</p>
      </ZhPromoteLayout>,
    );
    expect(screen.getByText('分享 Solvr').closest('[lang]')).toHaveAttribute('lang', 'zh-CN');
  });

  it('is not listed in the sitemap', async () => {
    const xml = await (await sitemapCore()).text();
    expect(xml).not.toContain('/zh');
  });
});
