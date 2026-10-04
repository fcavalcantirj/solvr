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
