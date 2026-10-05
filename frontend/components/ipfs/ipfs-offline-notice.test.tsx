import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import { IpfsOfflineNotice } from './ipfs-offline-notice';

describe('IpfsOfflineNotice', () => {
  it('says pinning is offline and what happens to a pin', () => {
    render(<IpfsOfflineNotice />);
    const notice = screen.getByRole('status');
    expect(notice).toHaveTextContent('IPFS pinning is offline');
    expect(notice).toHaveTextContent('accepted and then fails');
  });

  it('opens the /ipfs page, before its hero', () => {
    const page = readFileSync(resolve(__dirname, '../../app/ipfs/page.tsx'), 'utf8');
    expect(page.indexOf('<IpfsOfflineNotice />')).toBeGreaterThan(-1);
    expect(page.indexOf('<IpfsOfflineNotice />')).toBeLessThan(page.indexOf('<IpfsHero />'));
  });

  // AMCP checkpoints are pins on Solvr's pinning service: /amcp opens with the same notice.
  it('opens the /amcp page, before its hero', () => {
    const page = readFileSync(resolve(__dirname, '../../app/amcp/page.tsx'), 'utf8');
    expect(page.indexOf('<IpfsOfflineNotice />')).toBeGreaterThan(-1);
    expect(page.indexOf('<IpfsOfflineNotice />')).toBeLessThan(page.indexOf('<AmcpHero />'));
  });

  it('keeps /ipfs out of the sitemap while it is offline', () => {
    const policy = readFileSync(resolve(__dirname, '../../lib/seo/route-policy.ts'), 'utf8');
    expect(policy).toMatch(/\{ path: '\/ipfs', sitemap: false \}/);
  });
});
