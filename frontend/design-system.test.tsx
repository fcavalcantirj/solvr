/**
 * The visual system contract.
 *
 * Solvr's look is a short list of decisions: the SOLVR_ wordmark set in
 * JetBrains Mono, Inter for everything else, a warm near-white / near-black
 * monochrome palette, thin neutral borders, zero-radius geometry, a generous
 * vertical rhythm, and green reserved for confirmed presence or connection
 * health — always next to words that say the same thing.
 *
 * Those decisions live in app/globals.css and in the class names of the
 * components. Until now nothing enforced them, so a single `rounded-xl` or a
 * `text-green-500` that nobody can read in light mode could drift in unnoticed.
 * Every rule below fails loudly when that happens.
 *
 * Colour maths is done in OKLCH — the space the palette is authored in — and
 * converted to linear sRGB for the WCAG contrast ratio, so the thresholds here
 * are the real ones a browser produces, not eyeballed.
 */
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { SseStatusBadge } from '@/components/rooms/sse-status-badge'

const ROOT = path.dirname(fileURLToPath(import.meta.url))
const GLOBALS_CSS = fs.readFileSync(path.join(ROOT, 'app/globals.css'), 'utf8')
const TAILWIND_THEME = fs.readFileSync(
  path.join(ROOT, 'node_modules/tailwindcss/theme.css'),
  'utf8',
)

// ---------------------------------------------------------------------------
// OKLCH → sRGB → WCAG contrast
// ---------------------------------------------------------------------------

type Oklch = { l: number; c: number; h: number }

function parseOklch(value: string): Oklch | null {
  const match = value.match(
    /oklch\(\s*([\d.]+%?)\s+([\d.]+%?)\s+([\d.]+)(?:deg)?/,
  )
  if (!match) return null
  const scalar = (raw: string) =>
    raw.endsWith('%') ? parseFloat(raw) / 100 : parseFloat(raw)
  return { l: scalar(match[1]), c: scalar(match[2]), h: parseFloat(match[3]) }
}

/** OKLCH → linear-light sRGB, clamped to gamut. */
function toLinearSrgb({ l, c, h }: Oklch): [number, number, number] {
  const rad = (h * Math.PI) / 180
  const a = c * Math.cos(rad)
  const b = c * Math.sin(rad)
  const lp = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const mp = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const sp = (l - 0.0894841775 * a - 1.291485548 * b) ** 3
  return [
    4.0767416621 * lp - 3.3077115913 * mp + 0.2309699292 * sp,
    -1.2684380046 * lp + 2.6097574011 * mp - 0.3413193965 * sp,
    -0.0041960863 * lp - 0.7034186147 * mp + 1.707614701 * sp,
  ].map((v) => Math.min(1, Math.max(0, v))) as [number, number, number]
}

function luminance(color: Oklch): number {
  const [r, g, b] = toLinearSrgb(color)
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: Oklch, b: Oklch): number {
  const la = luminance(a)
  const lb = luminance(b)
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05)
}

// ---------------------------------------------------------------------------
// Reading the tokens
// ---------------------------------------------------------------------------

function cssBlock(selector: string): Record<string, string> {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`).exec(GLOBALS_CSS)
  if (!match) throw new Error(`globals.css has no "${selector}" block`)
  const vars: Record<string, string> = {}
  for (const line of match[1].split('\n')) {
    const decl = /^\s*(--[\w-]+):\s*([^;]+);/.exec(line)
    if (decl) vars[decl[1]] = decl[2].trim()
  }
  return vars
}

const LIGHT = cssBlock(':root')
const DARK = cssBlock('.dark')
const THEME = cssBlock('@theme inline')

function token(theme: Record<string, string>, name: string): Oklch {
  const raw = theme[`--${name}`]
  if (!raw) throw new Error(`no --${name} token`)
  const parsed = parseOklch(raw)
  if (!parsed) throw new Error(`--${name} is not an oklch() colour: ${raw}`)
  return parsed
}

/** Resolve a Tailwind palette utility value (e.g. "green-700") to its OKLCH. */
function palette(shade: string): Oklch {
  const match = new RegExp(`--color-${shade}:\\s*([^;]+);`).exec(TAILWIND_THEME)
  if (!match) throw new Error(`tailwindcss/theme.css has no --color-${shade}`)
  const parsed = parseOklch(match[1])
  if (!parsed) throw new Error(`--color-${shade} is not oklch(): ${match[1]}`)
  return parsed
}

// Tokens that are allowed to carry colour: the error state, the categorical
// chart ramp, and the prompt's one highlighter accent (a fill behind ink).
// Everything else in the palette is monochrome by design.
const CHROMATIC_TOKENS = /^--(destructive|destructive-foreground|chart-\d|prompt-accent)$/

// ---------------------------------------------------------------------------
// Reading the source
// ---------------------------------------------------------------------------

const SOURCE_DIRS = ['app', 'components', 'lib', 'hooks']

function sourceFiles(dirs: string[] = SOURCE_DIRS): string[] {
  const found: string[] = []
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(path.join(ROOT, dir), {
      withFileTypes: true,
    })) {
      const rel = `${dir}/${entry.name}`
      if (entry.isDirectory()) {
        if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
        walk(rel)
      } else if (
        /\.tsx?$/.test(entry.name) &&
        !/\.test\.tsx?$/.test(entry.name)
      ) {
        found.push(rel)
      }
    }
  }
  dirs.forEach(walk)
  return found
}

function read(rel: string): string {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8')
}

describe('design tokens — typography', () => {
  it('sets Inter as the body and display family', () => {
    expect(THEME['--font-sans']).toMatch(/^'Inter'/)
  })

  it('sets JetBrains Mono as the label family', () => {
    expect(THEME['--font-mono']).toMatch(/^'JetBrains Mono'/)
  })

  it('loads both families in the root layout and defaults the body to Inter', () => {
    const layout = read('app/layout.tsx')
    expect(layout).toMatch(/JetBrains_Mono.*from 'next\/font\/google'|from 'next\/font\/google'/)
    expect(layout).toContain('Inter(')
    expect(layout).toContain('JetBrains_Mono(')
    expect(layout).toMatch(/<body className={`font-sans/)
  })

  it('keeps the SOLVR_ wordmark in the mono family in both header and footer', () => {
    for (const file of ['components/header.tsx', 'components/footer.tsx']) {
      const source = read(file)
      expect(source, file).toContain('SOLVR_')
      expect(source, file).toContain(
        'font-mono text-lg tracking-tight font-medium',
      )
    }
  })
})

describe('design tokens — monochrome palette', () => {
  it('keeps the light background a warm near-white', () => {
    const bg = token(LIGHT, 'background')
    expect(bg.l).toBeGreaterThanOrEqual(0.95)
    expect(bg.c).toBeLessThanOrEqual(0.01)
    // A warm hue (yellow-ish) rather than a cold blue-grey.
    expect(bg.h).toBeGreaterThan(60)
    expect(bg.h).toBeLessThan(120)
  })

  it('keeps the light foreground a near-black', () => {
    expect(token(LIGHT, 'foreground').l).toBeLessThanOrEqual(0.15)
  })

  it.each([
    ['light', LIGHT],
    ['dark', DARK],
  ])('keeps every %s surface and text token neutral', (_name, theme) => {
    const coloured = Object.entries(theme)
      .filter(([key]) => !CHROMATIC_TOKENS.test(key) && key !== '--radius')
      .map(([key, value]) => [key, parseOklch(value)] as const)
      .filter(([, oklch]) => oklch && oklch.c > 0.01)
      .map(([key, oklch]) => `${key} (chroma ${oklch!.c})`)

    expect(coloured).toEqual([])
  })

  it('defines a dark counterpart for every light token except the shared radius', () => {
    const missing = Object.keys(LIGHT).filter(
      (key) => key !== '--radius' && !(key in DARK),
    )
    expect(missing).toEqual([])
  })
})

describe('design tokens — thin neutral borders', () => {
  it.each([
    ['light', LIGHT],
    ['dark', DARK],
  ])('%s borders are neutral hairlines, visible but never a hard rule', (
    _name,
    theme,
  ) => {
    const border = token(theme, 'border')
    expect(border.c).toBeLessThanOrEqual(0.01)

    const ratio = contrast(border, token(theme, 'background'))
    expect(ratio).toBeGreaterThan(1.2)
    expect(ratio).toBeLessThan(2)
  })

  it('uses the same neutral for inputs as for borders', () => {
    expect(LIGHT['--input']).toBe(LIGHT['--border'])
    expect(DARK['--input']).toBe(DARK['--border'])
  })
})

describe('design tokens — accessible contrast in both themes', () => {
  const PAIRS: [string, string, number][] = [
    ['foreground', 'background', 7],
    ['card-foreground', 'card', 7],
    ['popover-foreground', 'popover', 7],
    ['primary-foreground', 'primary', 7],
    ['secondary-foreground', 'secondary', 4.5],
    ['accent-foreground', 'accent', 4.5],
    ['muted-foreground', 'background', 4.5],
    ['muted-foreground', 'card', 4.5],
    ['muted-foreground', 'muted', 4.5],
    ['prompt-accent-foreground', 'prompt-accent', 7],
  ]

  for (const [themeName, theme] of [
    ['light', LIGHT],
    ['dark', DARK],
  ] as const) {
    it.each(PAIRS)(
      `${themeName}: %s on %s clears %d:1`,
      (fg, bg, minimum) => {
        expect(contrast(token(theme, fg), token(theme, bg))).toBeGreaterThanOrEqual(
          minimum,
        )
      },
    )
  }
})

describe('sharp geometry', () => {
  it('pins the radius token to zero', () => {
    expect(LIGHT['--radius']).toBe('0rem')
  })

  it('keeps test files out of the utility scan', () => {
    // Tailwind treats every source file as markup, so a class name merely
    // *mentioned* below — in a comment, a regex, an assertion — would ship a
    // real rule. Measured: without these two lines this very file puts
    // border-radius 0.25rem, 4px and calc(var(--radius) + 4px) back into the
    // stylesheet it exists to keep square.
    expect(GLOBALS_CSS).toContain('@source not "../**/*.test.ts";')
    expect(GLOBALS_CSS).toContain('@source not "../**/*.test.tsx";')
  })

  /**
   * With --radius at 0rem the theme scale resolves to: sm = -4px, md = -2px
   * (both invalid/clamped to 0), lg = 0. `full` is a circle — dots, avatars and
   * spinners, not a corner. Everything else Tailwind still supplies from its own
   * defaults: bare `rounded` is 0.25rem, `xs` is 0.125rem, `xl` is radius+4px,
   * `2xl`/`3xl` larger still. Those are the ones that round a corner.
   */
  const SHARP_SIZES = new Set(['none', 'sm', 'md', 'lg', 'full'])
  // The trailing lookahead (rather than \b) is what makes an arbitrary-value
  // utility parse: \b never matches after the closing bracket of
  // `rounded-[4px]`, so the engine would happily backtrack to a bare `rounded`
  // and mis-report the size.
  const RADIUS_UTILITY =
    /\brounded(?:-(?:t|r|b|l|s|e|tl|tr|br|bl|ss|se|es|ee))?(?:-(\[[^\]]+\]|[a-z0-9]+))?(?![\w-])/g

  function isSharp(size: string | undefined): boolean {
    if (size === undefined) return false
    if (SHARP_SIZES.has(size)) return true
    if (size.startsWith('[')) {
      const value = size.slice(1, -1)
      if (value === 'inherit') return true
      if (value.includes('var(--radius)')) return true
      const length = /^(-?[\d.]+)(?:px|rem)?$/.exec(value)
      return length ? parseFloat(length[1]) <= 0 : false
    }
    return false
  }

  it('never rounds a corner anywhere in app, components, lib or hooks', () => {
    const offenders: string[] = []

    for (const file of sourceFiles()) {
      read(file)
        .split('\n')
        .forEach((line, index) => {
          for (const match of line.matchAll(RADIUS_UTILITY)) {
            if (!isSharp(match[1])) {
              offenders.push(`${file}:${index + 1} ${match[0]}`)
            }
          }
        })
    }

    expect(offenders).toEqual([])
  })

  it('keeps the primary control square at every size and variant', () => {
    const button = read('components/ui/button.tsx')
    for (const match of button.matchAll(RADIUS_UTILITY)) {
      expect(isSharp(match[1]), `button.tsx: ${match[0]}`).toBe(true)
    }
  })
})

describe('generous spacing', () => {
  // The homepage sections share one rhythm. Keeping it identical is what makes
  // the page read as a single system rather than a stack of separate widgets.
  // Tightened from py-24 lg:py-32 in v1.3.4: the index had become one very long page.
  const RHYTHM = 'px-4 sm:px-6 lg:px-12 py-12 lg:py-16'
  const SECTIONS = [
    'components/collaboration-example.tsx',
    'components/homepage/use-cases-section.tsx',
    'components/homepage/room-stats-section.tsx',
    'components/homepage/room-activity-section.tsx',
    'components/homepage/room-previews-section.tsx',
    'components/homepage/api-usage-section.tsx',
    'components/homepage/search-stats-section.tsx',
    'components/homepage/community-totals-section.tsx',
    'components/homepage/reusable-posts-section.tsx',
    'components/homepage/closing-section.tsx',
  ]

  it.each(SECTIONS)('%s keeps the shared section rhythm', (file) => {
    expect(read(file)).toContain(RHYTHM)
  })

  it('gives the hero the same horizontal gutters and clears the fixed header', () => {
    const hero = read('components/hero-section.tsx')
    expect(hero).toContain('px-4 sm:px-6 lg:px-12')
    expect(hero).toContain('pt-24')
  })
})

describe('restrained status colour on the room surfaces', () => {
  // Rooms and room detail are where Solvr uses colour at all: green for
  // confirmed presence, amber while reconnecting, red at the hard limit.
  // v1.3.7: /data carries the same live dots and the amber stale notice, so it
  // is held to the same contrast.
  const ROOM_FILES = [
    ...sourceFiles(['components/rooms']),
    'app/data/page.tsx',
    'components/homepage/room-stats-section.tsx',
    'components/homepage/overview-meta-banner.tsx',
    'app/status/page.tsx',
    'app/claim/page.tsx',
    'app/email/unsubscribe/page.tsx',
    'components/posts/post-detail.tsx',
    'app/pins/page.tsx',
    'app/referrals/page.tsx',
    'app/settings/page.tsx',
    'app/settings/api-keys/page.tsx',
    'components/admin/ipfs-status.tsx',
    'components/agents/agent-briefing.tsx',
    'components/agents/agent-briefing-platform.tsx',
    'components/agents/agent-profile-client.tsx',
    'components/agents/agents-list.tsx',
    'components/footer.tsx',
    'components/badges-display.tsx',
    'components/claim-agent-form.tsx',
  ]
  const FAMILY = '(?:green|emerald|amber|red|yellow)'
  const TEXT = new RegExp(`(dark:)?text-(${FAMILY}-\\d{2,3})\\b`, 'g')
  const DOT = new RegExp(`(dark:)?bg-(${FAMILY}-\\d{2,3})\\b`, 'g')

  // WCAG 1.4.3 for text, 1.4.11 for the indicator dots beside it.
  const TEXT_MINIMUM = 4.5
  const INDICATOR_MINIMUM = 3

  type Usage = { where: string; shade: string; light: boolean; dark: boolean }

  function usages(pattern: RegExp, onlyDots: boolean): Usage[] {
    const collected: Usage[] = []

    for (const file of ROOM_FILES) {
      read(file)
        .split('\n')
        .forEach((line, index) => {
          if (onlyDots && !line.includes('rounded-full')) return
          const matches = [...line.matchAll(pattern)]
          if (matches.length === 0) return
          const hasDarkVariant = matches.some((m) => m[1] === 'dark:')

          for (const match of matches) {
            const isDarkVariant = match[1] === 'dark:'
            collected.push({
              where: `${file}:${index + 1} ${match[0]}`,
              shade: match[2],
              // A base utility with no dark: counterpart on the same element
              // paints in BOTH themes and has to survive both backgrounds.
              light: !isDarkVariant,
              dark: isDarkVariant || !hasDarkVariant,
            })
          }
        })
    }

    return collected
  }

  function failures(list: Usage[], minimum: number): string[] {
    const bad: string[] = []
    for (const use of list) {
      const colour = palette(use.shade)
      if (use.light) {
        const ratio = contrast(colour, token(LIGHT, 'background'))
        if (ratio < minimum) {
          bad.push(`${use.where} — light ${ratio.toFixed(2)}:1`)
        }
      }
      if (use.dark) {
        const ratio = contrast(colour, token(DARK, 'background'))
        if (ratio < minimum) {
          bad.push(`${use.where} — dark ${ratio.toFixed(2)}:1`)
        }
      }
    }
    return bad
  }

  it('finds the status colour it is meant to be checking', () => {
    // Guards the scan itself: a refactor that renames the room components must
    // not turn these rules into a silent no-op.
    expect(ROOM_FILES.length).toBeGreaterThan(5)
    expect(usages(TEXT, false).length).toBeGreaterThan(0)
    expect(usages(DOT, true).length).toBeGreaterThan(0)
  })

  it('keeps status text readable in whichever theme it paints in', () => {
    expect(failures(usages(TEXT, false), TEXT_MINIMUM)).toEqual([])
  })

  it('keeps presence dots distinguishable in whichever theme they paint in', () => {
    expect(failures(usages(DOT, true), INDICATOR_MINIMUM)).toEqual([])
  })

  it('says "live" in words, not only in green', () => {
    render(<SseStatusBadge status="connected" />)
    expect(screen.getByText('LIVE')).toBeInTheDocument()
  })

  it('says "reconnecting" in words, not only in amber', () => {
    render(<SseStatusBadge status="reconnecting" />)
    expect(screen.getByText('RECONNECTING...')).toBeInTheDocument()
  })
})

describe('the one-sentence page language', () => {
  // Pages moved to the v1.3.7 look, one family at a time. A migrated file sets no
  // kicker above a heading (the 0.3em-tracked label), no old 7xl column and no
  // 10px labels; captions are the 11px mono of components/page/caption.tsx.
  const MIGRATED = [
    'app/data/page.tsx',
    'components/data/platform-statistics.tsx',
    'components/data/statistics-primitives.tsx',
    'components/homepage/room-stats-section.tsx',
    'components/homepage/api-usage-section.tsx',
    'components/homepage/search-stats-section.tsx',
    'components/homepage/community-totals-section.tsx',
    'components/page/caption.tsx',
    'components/page/segmented-control.tsx',
    'components/posts/posts-page-client.tsx',
    'components/posts/posts-list.tsx',
    'components/posts/post-card.tsx',
    'app/status/page.tsx',
    'components/leaderboard/leaderboard-page-client.tsx',
    'app/login/page.tsx',
    'app/join/page.tsx',
    'app/claim/page.tsx',
    'app/auth/callback/page.tsx',
    'app/email/unsubscribe/page.tsx',
    'app/not-found.tsx',
    'app/notifications/page.tsx',
    'app/zh/promote/page.tsx',
    'components/blog/blog-page-client.tsx',
    'app/blog/[slug]/blog-post-content.tsx',
    'app/blog/create/page.tsx',
    'components/hero-section.tsx',
    'components/homepage/room-activity-section.tsx',
    'components/homepage/room-previews-section.tsx',
    'components/homepage/reusable-posts-section.tsx',
    'components/homepage/closing-section.tsx',
    'components/collaboration-example.tsx',
    'app/about/page.tsx',
    'app/docs/page.tsx',
    'app/docs/protocol/page.tsx',
    'app/docs/guides/page.tsx',
    'app/terms/page.tsx',
    'app/privacy/page.tsx',
    'components/legal/privacy-later-sections.tsx',
    'components/posts/post-detail.tsx',
    'components/posts/post-composer.tsx',
    'components/posts/post-editor.tsx',
    'app/posts/[id]/page.tsx',
    'app/posts/[id]/replies/[page]/page.tsx',
    'app/posts/new/page.tsx',
    'app/posts/[id]/edit/page.tsx',
    'app/rooms/page.tsx',
    'app/rooms/[slug]/page.tsx',
    'app/rooms/[slug]/history/[page]/page.tsx',
    'components/rooms/room-header.tsx',
    'components/rooms/room-card.tsx',
    'components/rooms/room-list.tsx',
    'components/rooms/room-detail-client.tsx',
    'components/rooms/message-bubble.tsx',
    'components/rooms/presence-sidebar.tsx',
    'components/rooms/rooms-browser.tsx',
    'app/how-it-works/page.tsx',
    'app/mcp/page.tsx',
    'components/amcp/amcp-features.tsx',
    'components/amcp/amcp-hero.tsx',
    'components/amcp/amcp-recovery.tsx',
    'components/api/api-cta.tsx',
    'components/api/api-endpoints.tsx',
    'components/api/api-hero.tsx',
    'components/api/api-mcp.tsx',
    'components/api/api-playground.tsx',
    'components/api/api-quickstart.tsx',
    'components/api/api-rate-limits.tsx',
    'components/api/api-sdks.tsx',
    'components/how/how-cta.tsx',
    'components/how/how-hero.tsx',
    'components/how/how-honesty.tsx',
    'components/how/how-problem.tsx',
    'components/how/how-research.tsx',
    'components/how/how-solvr.tsx',
    'components/how/how-stack.tsx',
    'components/how/how-vision.tsx',
    'components/ipfs/ipfs-api.tsx',
    'components/ipfs/ipfs-features.tsx',
    'components/ipfs/ipfs-hero.tsx',
    'components/mcp/mcp-hero.tsx',
    'components/mcp/mcp-setup.tsx',
    'components/mcp/mcp-tools.tsx',
    'components/page/copy-button.tsx',
    'components/page/marketing.tsx',
    'components/skill/skill-hero.tsx',
    'components/skill/skill-install.tsx',
    'components/skill/skill-preview.tsx',
    'app/admin/system/page.tsx',
    'app/agents/[id]/page.tsx',
    'app/agents/page.tsx',
    'app/dashboard/page.tsx',
    'app/pins/page.tsx',
    'app/referrals/page.tsx',
    'app/settings/agents/page.tsx',
    'app/settings/api-keys/page.tsx',
    'app/settings/page.tsx',
    'app/users/[id]/page.tsx',
    'app/users/page.tsx',
    'components/admin/ipfs-status.tsx',
    'components/agents/agent-activity-feed.tsx',
    'components/agents/agent-briefing-platform.tsx',
    'components/agents/agent-briefing.tsx',
    'components/agents/agent-profile-client.tsx',
    'components/agents/agents-list.tsx',
    'components/agents/agents-page-client.tsx',
    'components/agents/agents-sidebar.tsx',
    'components/agents/briefing-block.tsx',
    'components/page/controls.ts',
    'components/page/figure-ledger.tsx',
    'components/page/page-header.tsx',
    'components/page/page-section.tsx',
    'components/page/profile-hero.tsx',
    'components/settings/edit-agent-modal.tsx',
    'components/settings/settings-layout.tsx',
    'components/users/contributions-list.tsx',
    'components/users/user-posts-list.tsx',
    'components/users/user-profile-client.tsx',
    'components/users/users-page-client.tsx',
    'components/footer.tsx',
    'components/homepage/overview-meta-banner.tsx',
    'components/homepage/live-overview.tsx',
    'components/follow-button.tsx',
    'components/badges-display.tsx',
    'components/ui/user-menu.tsx',
    'components/claim-agent-form.tsx',
  ]

  it.each(MIGRATED)('%s speaks the new language', (file) => {
    const source = read(file)
    expect(source, file).not.toContain('tracking-[0.3em]')
    expect(source, file).not.toContain('max-w-7xl')
    expect(source, file).not.toContain('text-[10px]')
    // The widest letter-spacing is the old kicker's; captions use tracking-[0.18em].
    expect(source, file).not.toContain('tracking-widest')
    // Display headings are Inter light; monospace stays at caption, code and wordmark
    // sizes (code itself, an install line or a config, may be set big).
    expect(source, file).not.toMatch(/<h[1-3][^>]*className="[^"]*(?:font-mono[^"]*\btext-[2-6]xl\b|\btext-[2-6]xl\b[^"]*font-mono)/)
    // Monochrome: no gradients and no off-palette hues (status shades are checked separately).
    expect(source, file).not.toMatch(/bg-gradient-|\b(?:from|to|via)-(?:cyan|blue|indigo|purple|violet|pink|emerald|teal|sky)-/)
    // The CI limit (scripts/check-file-size.sh): a migrated file stays under 800 lines.
    expect(source.split('\n').length, file).toBeLessThanOrEqual(800)
  })

  // The owner's rule for every index page: the page scrolls vertically only.
  it.each([
    'components/posts/posts-list.tsx',
    'components/posts/posts-page-client.tsx',
    'components/blog/blog-page-client.tsx',
    'components/leaderboard/leaderboard-page-client.tsx',
    'components/agents/agents-page-client.tsx',
    'components/agents/agents-list.tsx',
    'components/users/users-page-client.tsx',
    'app/pins/page.tsx',
  ])('%s never scrolls a strip sideways', (file) => {
    expect(read(file), file).not.toMatch(/overflow-x-(?:auto|scroll)/)
  })
})

describe('the posts mosaic', () => {
  // v1.3.7: /posts uses the whole width as a mosaic. Its rhythm of tile sizes is
  // a fixed pattern by position, written in CSS, so no tile size is ever computed
  // from a score, a count or a text length.
  const MOSAIC_CSS = 'components/posts/posts-mosaic.module.css'

  it('lays the posts out over the full width, never in the old narrow strip', () => {
    for (const file of ['components/posts/posts-page-client.tsx', 'components/posts/posts-list.tsx']) {
      const source = read(file)
      expect(source, file).not.toMatch(/max-w-(?:2xl|3xl|4xl)/)
    }
  })

  it('sets the tile sizes by position in CSS', () => {
    const css = read(MOSAIC_CSS)
    expect(css).toMatch(/nth-child\(10n \+ 1\)/)
    expect(css).toContain('grid-template-columns: repeat(12')
    for (const file of ['components/posts/posts-list.tsx', 'components/posts/post-card.tsx']) {
      // Showing a count is fine; branching a tile's size on one is not.
      expect(read(file), file).not.toMatch(
        /(?:vote_score|reply_count|upvotes|view_count)\s*[<>]=?|(?:title|description)\.length/,
      )
    }
  })
})

describe('square corners and tokens in CSS modules', () => {
  // The radius scan above reads .ts and .tsx only. A CSS module is held to the
  // same rules: no rounded corners and no colour that is not a token.
  function cssModules(dir: string): string[] {
    const found: string[] = []
    const walk = (rel: string) => {
      for (const entry of fs.readdirSync(path.join(ROOT, rel), { withFileTypes: true })) {
        const child = `${rel}/${entry.name}`
        if (entry.isDirectory()) {
          if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
          walk(child)
        } else if (entry.name.endsWith('.module.css')) {
          found.push(child)
        }
      }
    }
    walk(dir)
    return found
  }
  const MODULES = [...cssModules('app'), ...cssModules('components')]

  it('finds the CSS modules it is meant to be checking', () => {
    expect(MODULES.length).toBeGreaterThan(0)
  })

  it.each(MODULES)('%s rounds no corner and paints only with tokens', (file) => {
    const css = read(file)
    expect(css, file).not.toMatch(/border-radius\s*:\s*(?!0[;\s]|0px)/)
    expect(css, file).not.toMatch(/#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(|\boklch\(/)
  })
})
