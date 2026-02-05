/**
 * Tests for RootLayout component and metadata
 * Tests metadata configuration per SPEC.md Part 19.2 SEO
 */

// Mock next/font/google since it requires Next.js context
jest.mock('next/font/google', () => ({
  Inter: () => ({
    variable: '--font-inter',
  }),
  JetBrains_Mono: () => ({
    variable: '--font-jetbrains-mono',
  }),
}));

import { metadata } from '../app/layout';

describe('Metadata', () => {
  it('has correct site title per SPEC.md Part 19.2', () => {
    expect(metadata.title).toBe('Solvr - Knowledge Base for Developers & AI Agents');
  });

  it('has correct description per SPEC.md Part 19.2', () => {
    expect(metadata.description).toBe(
      'Where humans and AI agents collaborate to solve problems, share knowledge, and build collective intelligence.'
    );
  });

  it('has keywords for SEO', () => {
    expect(metadata.keywords).toBeDefined();
    expect(Array.isArray(metadata.keywords)).toBe(true);
    const keywords = metadata.keywords as string[];
    expect(keywords).toContain('developer knowledge base');
    expect(keywords).toContain('AI agents');
    expect(keywords).toContain('coding help');
    expect(keywords).toContain('programming Q&A');
  });

  it('has openGraph configuration per SPEC.md Part 19.2', () => {
    expect(metadata.openGraph).toBeDefined();
    const og = metadata.openGraph as Record<string, unknown>;
    expect(og.type).toBe('website');
    expect(og.siteName).toBe('Solvr');
    expect(og.title).toBe('Solvr - Knowledge Base for Developers & AI Agents');
    expect(og.description).toBe(
      'Where humans and AI agents collaborate to solve problems, share knowledge, and build collective intelligence.'
    );
  });

  it('has twitter card configuration per SPEC.md Part 19.2', () => {
    expect(metadata.twitter).toBeDefined();
    const twitter = metadata.twitter as Record<string, unknown>;
    expect(twitter.card).toBe('summary_large_image');
    expect(twitter.title).toBe('Solvr - Knowledge Base for Developers & AI Agents');
    expect(twitter.description).toBe(
      'Where humans and AI agents collaborate to solve problems, share knowledge, and build collective intelligence.'
    );
  });

  it('has robots configuration for indexing', () => {
    expect(metadata.robots).toBeDefined();
    const robots = metadata.robots as { index: boolean; follow: boolean };
    expect(robots.index).toBe(true);
    expect(robots.follow).toBe(true);
  });

  it('has authors defined', () => {
    expect(metadata.authors).toBeDefined();
  });

  it('has metadataBase for absolute URL generation', () => {
    expect(metadata.metadataBase).toBeDefined();
  });
});
