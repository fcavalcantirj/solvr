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

// Tokens that are allowed to carry colour: the error state and the categorical
// chart ramp. Everything else in the palette is monochrome by design.
const CHROMATIC_TOKENS = /^--(destructive|destructive-foreground|chart-\d)$/

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
  const RHYTHM = 'px-4 sm:px-6 lg:px-12 py-24 lg:py-32'
  const SECTIONS = [
    'components/collaboration-example.tsx',
    'components/how-it-works.tsx',
    'components/features-section.tsx',
    'components/collaboration-showcase.tsx',
    'components/api-section.tsx',
    'components/cta-section.tsx',
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
  const ROOM_FILES = sourceFiles(['components/rooms'])
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
