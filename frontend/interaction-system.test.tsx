/**
 * The interaction contract.
 *
 * design-system.test.tsx holds what the pages look like at rest. This file
 * holds how they answer a visitor: a control that is pointed at must visibly
 * change, and a pointer over it must say it can be clicked.
 *
 * v1.3.7 measured the gap on the built site: 43 controls changed by less than
 * 1.35:1 on hover (an ink button going to 90% ink is 1.00:1), and Tailwind v4
 * no longer gives a <button> the hand cursor. The rules below keep it closed.
 */
import { describe, it, expect } from 'vitest'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = path.dirname(fileURLToPath(import.meta.url))

function sourceFiles(dirs: string[]): string[] {
  const found: string[] = []
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(path.join(ROOT, dir), { withFileTypes: true })) {
      const rel = `${dir}/${entry.name}`
      if (entry.isDirectory()) {
        if (entry.name === 'node_modules' || entry.name.startsWith('.')) continue
        walk(rel)
      } else if (/\.tsx?$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
        found.push(rel)
      }
    }
  }
  dirs.forEach(walk)
  return found
}

const read = (file: string) => fs.readFileSync(path.join(ROOT, file), 'utf8')

/** Every quoted string in the source, with where it sits. Class lists are quoted strings. */
function quotedStrings(file: string): { where: string; text: string }[] {
  const out: { where: string; text: string }[] = []
  read(file)
    .split('\n')
    .forEach((line, index) => {
      for (const match of line.matchAll(/(["'`])((?:(?!\1).)*)\1/g)) {
        out.push({ where: `${file}:${index + 1}`, text: match[2] })
      }
    })
  return out
}

const SOURCES = sourceFiles(['app', 'components'])

describe('every control answers the pointer', () => {
  // A hover that moves a fill by a few percent is invisible: 90% ink on ink,
  // 90% paper on paper, a 5% wash, an opacity of 0.9.
  const FAINT = /hover:bg-(?:foreground|primary|background)\/\d+|hover:opacity-9\d\b/

  it('uses no hover so faint it cannot be seen', () => {
    const faint = SOURCES.flatMap((file) =>
      read(file)
        .split('\n')
        .map((line, index) => (FAINT.test(line) ? `${file}:${index + 1}` : null))
        .filter((hit): hit is string => hit !== null),
    )
    expect(faint).toEqual([])
  })

  it('turns an ink control over to paper on hover', () => {
    // One rule for the site: ink inverts to paper, keeping its ink edge. A
    // selected option (ink with no hover of its own) is a state, not a
    // control waiting to be pressed, and the follow button turns red to say
    // "unfollow".
    const offenders: string[] = []
    for (const file of SOURCES) {
      for (const { where, text } of quotedStrings(file)) {
        const tokens = text.split(/\s+/)
        const ink = tokens.includes('bg-foreground') && tokens.includes('text-background')
        if (!ink || !tokens.some((t) => t.startsWith('hover:'))) continue
        if (tokens.includes('hover:bg-background') || tokens.includes('hover:bg-destructive')) continue
        offenders.push(where)
      }
    }
    expect(offenders).toEqual([])
  })

  it('gives every ink button in the header a hover, desktop and mobile', () => {
    // The header's CONNECT AGENTS sits on every page; its mobile twin and the
    // menu's full-width button had no hover at all.
    const buttons = quotedStrings('components/header.tsx').filter(({ text }) => {
      const tokens = text.split(/\s+/)
      return tokens.includes('bg-foreground') && tokens.includes('text-background') && tokens.some((t) => t.startsWith('px-'))
    })
    expect(buttons.length).toBeGreaterThanOrEqual(3)
    for (const { where, text } of buttons) {
      expect(text.split(/\s+/), where).toContain('hover:bg-background')
    }
  })

  it('shows the hand over anything that can be pressed', () => {
    // Tailwind v4 dropped cursor:pointer from buttons; the base layer puts it back.
    const css = read('app/globals.css')
    expect(css).toMatch(/button:not\(:disabled\)[^{]*\{\s*cursor:\s*pointer;/)
  })
})

describe('no link points at a page the app does not have', () => {
  // v1.3.7 crawled the built site: /api, /dashboard/settings and
  // /forgot-password were linked and 404 on production; /blog/tags returned 500.
  const ROUTES: string[][] = []
  const walk = (dir: string) => {
    for (const entry of fs.readdirSync(path.join(ROOT, dir), { withFileTypes: true })) {
      const rel = `${dir}/${entry.name}`
      if (entry.isDirectory()) {
        walk(rel)
        continue
      }
      const segments = dir.split('/').slice(1).filter((s) => !/^\(.*\)$/.test(s))
      if (/^(page|route)\.(tsx?|jsx?)$/.test(entry.name)) ROUTES.push(segments)
      const metadata: Record<string, string> = { 'sitemap.ts': 'sitemap.xml', 'robots.ts': 'robots.txt' }
      if (metadata[entry.name]) ROUTES.push([...segments, metadata[entry.name]])
    }
  }
  walk('app')

  const routeExists = (href: string) => {
    const parts = href.split('/').filter(Boolean)
    return ROUTES.some((route) => {
      for (let i = 0; i < route.length; i++) {
        if (/^\[\[?\.\.\./.test(route[i])) return true
        if (i >= parts.length) return false
        if (/^\[.*\]$/.test(route[i])) continue
        if (route[i] !== parts[i]) return false
      }
      return route.length === parts.length
    })
  }

  // Old paths the middleware redirects (/feed, /new, /problems, …) are real destinations too.
  const middleware = read('middleware.ts')
  const REDIRECTED = [...middleware.slice(middleware.indexOf('matcher')).matchAll(/'(\/[^']*)'/g)].map((m) =>
    m[1].replace(/\/:path\*$/, ''),
  )
  const isPublicFile = (href: string) => fs.existsSync(path.join(ROOT, 'public', href))

  function literalHrefs(): { where: string; href: string }[] {
    const found: { where: string; href: string }[] = []
    for (const file of SOURCES) {
      read(file)
        .split('\n')
        .forEach((line, index) => {
          for (const match of line.matchAll(/\bhref\s*[:=]\s*\{?\s*["'`](\/[^"'`\s]*)["'`]/g)) {
            if (match[1].startsWith('//') || match[1].includes('${')) continue
            let href = match[1].split('#')[0].split('?')[0] || '/'
            if (href.length > 1 && href.endsWith('/')) href = href.slice(0, -1)
            found.push({ where: `${file}:${index + 1}`, href })
          }
        })
    }
    return found
  }

  it('finds the routes and links it is meant to be checking', () => {
    expect(ROUTES.length).toBeGreaterThan(30)
    expect(literalHrefs().length).toBeGreaterThan(50)
  })

  it('resolves every literal link to a page, a route handler, a public file or a redirect', () => {
    const dead = literalHrefs()
      .filter(({ href }) => !routeExists(href) && !isPublicFile(href) && !REDIRECTED.some((r) => href === r || href.startsWith(`${r}/`)))
      .map(({ where, href }) => `${where} ${href}`)
    expect(dead).toEqual([])
  })

  it('does not link to /blog/tags, which blog/[slug] would otherwise swallow', () => {
    expect(literalHrefs().filter(({ href }) => href === '/blog/tags')).toEqual([])
  })
})

describe('no button that does nothing', () => {
  // v1.3.7: /privacy carried a DOWNLOAD PDF button with no handler and no PDF.
  // A <button> must act: a click handler, a submit, spread props, or a Radix
  // trigger (asChild) that wires the click in.
  it('gives every <button> an action', () => {
    const idle: string[] = []
    for (const file of SOURCES.filter((f) => f.endsWith('.tsx'))) {
      const src = read(file)
      for (const match of src.matchAll(/<button\b/g)) {
        let depth = 0
        let end = match.index + 7
        for (; end < src.length; end++) {
          const c = src[end]
          if (c === '{') depth++
          else if (c === '}') depth--
          else if (c === '>' && depth === 0) break
        }
        const tag = src.slice(match.index, end + 1)
        if (/onClick|onMouseDown|onPointerDown|type=["']submit["']|\{\.\.\.|form=/.test(tag)) continue
        if (/Trigger asChild>\s*$/.test(src.slice(Math.max(0, match.index - 120), match.index))) continue
        idle.push(`${file}:${src.slice(0, match.index).split('\n').length}`)
      }
    }
    expect(idle).toEqual([])
  })
})

describe('the consent bar asks without steering', () => {
  // SPEC.md 27.7: one bar asks every visitor whether Google Analytics may load. Decline and
  // Accept are ONE control used twice, same size and same style, neither filled with ink: a
  // bar that made Accept the prominent button would be asking for a yes. It is a region at
  // the bottom of the window, never a dialog over the page.
  const BAR = 'components/consent-bar.tsx'

  /** Every <button ...> opening tag in a source, read to its own closing bracket. */
  function buttonTags(src: string): string[] {
    const tags: string[] = []
    for (const match of src.matchAll(/<button\b/g)) {
      let depth = 0
      let end = match.index + 7
      for (; end < src.length; end++) {
        const c = src[end]
        if (c === '{') depth++
        else if (c === '}') depth--
        else if (c === '>' && depth === 0) break
      }
      tags.push(src.slice(match.index, end + 1))
    }
    return tags
  }

  it('gives Decline and Accept one shared class list', () => {
    const tags = buttonTags(read(BAR))
    expect(tags).toHaveLength(2)
    const classes = tags.map((tag) => /className=\{(\w+)\}/.exec(tag)?.[1])
    expect(classes[0]).toBeDefined()
    expect(classes[0]).toBe(classes[1])
  })

  it('fills neither choice with ink at rest, and answers the pointer on both', () => {
    const shared = quotedStrings(BAR).filter(({ text }) => text.split(/\s+/).includes('uppercase') && text.includes('hover:'))
    expect(shared.length).toBeGreaterThan(0)
    for (const { where, text } of shared) {
      const tokens = text.split(/\s+/)
      expect(tokens, where).not.toContain('bg-foreground')
      expect(tokens, where).toContain('hover:bg-foreground')
      expect(tokens.some((t) => t.startsWith('focus-visible:outline')), where).toBe(true)
    }
  })

  it('is a region at the bottom of the window, never a dialog', () => {
    const src = read(BAR)
    expect(src).toMatch(/<section\b/)
    expect(src).not.toMatch(/role=["']dialog["']|aria-modal|@\/components\/ui\/(dialog|alert-dialog|sheet|drawer)/)
    expect(src).not.toMatch(/\binset-0\b/)
  })

  it('offers Cookie settings in the footer, the account menu and the mobile menu', () => {
    // Many pages have no footer, so the footer alone would strand a visitor who changed their mind.
    for (const file of ['components/footer.tsx', 'components/ui/user-menu.tsx', 'components/header.tsx']) {
      expect(read(file), file).toMatch(/<CookieSettingsButton\b/)
    }
  })
})
