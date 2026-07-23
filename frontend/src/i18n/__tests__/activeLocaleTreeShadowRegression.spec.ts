import { describe, expect, it } from 'vitest'
import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import en from '../locales/en'
import zh from '../locales/zh'

// --- Regression guard for the shadow bug -----------------------------------
//
// frontend/src/i18n/index.ts loads locales via `import('./locales/en')` /
// `import('./locales/zh')`. Bundler/Node module resolution picks a sibling
// FILE (`en.ts`) over a same-named DIRECTORY (`en/`) when both exist, so a
// monolithic `en.ts`/`zh.ts` silently shadows the actively-maintained modular
// `en/`/`zh/` tree -- every key added only to the modular tree renders as a
// raw i18n key in the UI, with zero build-time signal.
//
// The fix was to delete the monolithic files and port every key forward into
// the modular tree. This test file exists so that bug can never quietly
// recur: if `en.ts` or `zh.ts` reappear next to the directories, or if any
// key actually referenced by a component stops resolving, this test fails.

const LOCALES_DIR = join(__dirname, '..', 'locales')
const SRC_DIR = join(__dirname, '..', '..')

describe('monolithic locale files must not reappear', () => {
  it.each(['en', 'zh'] as const)('locales/%s.ts does not exist next to locales/%s/', (locale) => {
    const monolithicPath = join(LOCALES_DIR, `${locale}.ts`)
    expect(existsSync(monolithicPath)).toBe(false)
  })
})

// --- Flatten helper ----------------------------------------------------------

function flattenKeys(obj: Record<string, unknown>, prefix = ''): Set<string> {
  const keys = new Set<string>()
  for (const [k, v] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${k}` : k
    if (v !== null && typeof v === 'object' && !Array.isArray(v)) {
      for (const nested of flattenKeys(v as Record<string, unknown>, fullKey)) {
        keys.add(nested)
      }
    } else {
      keys.add(fullKey)
    }
  }
  return keys
}

// --- Static extraction of every t('...') / $t('...') literal key in src/ ----

const SCAN_EXTENSIONS = ['.vue', '.ts', '.tsx']
const SKIP_DIRS = new Set(['__tests__', 'node_modules'])

function collectSourceFiles(dir: string, out: string[]): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (SKIP_DIRS.has(entry.name)) continue
      collectSourceFiles(join(dir, entry.name), out)
      continue
    }
    if (entry.name.endsWith('.spec.ts') || entry.name.endsWith('.test.ts')) continue
    if (SCAN_EXTENSIONS.some((ext) => entry.name.endsWith(ext))) {
      out.push(join(dir, entry.name))
    }
  }
  return out
}

// Matches t('key') / t("key") / $t('key') / $t("key"). Deliberately only
// matches single/double-quoted string literals -- template literals
// (`t(\`x.${y}\`)`) are dynamic and can't be statically resolved, so they're
// naturally excluded (the regex never matches a backtick).
const CALL_RE = /(?<![a-zA-Z0-9_.$])\$?t\(\s*(['"])((?:\\.|(?!\1).)*)\1/g

// Trailing-dot dynamic namespaces built via `t('x.' + val)` (e.g.
// `payment.methods.`, `admin.accounts.status.`, `admin.redeem.types.`,
// `admin.users.roles.`, `keys.status.`, `usage.errors.categories.`, and a
// handful of siblings). These base namespaces already have concrete sibling
// keys; the trailing-dot form itself is never a resolvable leaf key.
const isDynamicTrailingDot = (key: string): boolean => key.endsWith('.')

// home.resources.* in HomeView.vue is locale-conditional by design: the en
// and zh branches of the component reference different key names on purpose
// (e.g. `trustItems.productHub` only for en, `trustItems.xinyong` only for
// zh). So each such key only needs to resolve in ITS OWN locale, not both.
const isHomeResources = (key: string): boolean => key.startsWith('home.resources.')

function extractReferencedKeys(): Map<string, string[]> {
  const files = collectSourceFiles(SRC_DIR, [])
  const refs = new Map<string, string[]>()
  for (const file of files) {
    const content = readFileSync(file, 'utf8')
    let match: RegExpExecArray | null
    CALL_RE.lastIndex = 0
    while ((match = CALL_RE.exec(content))) {
      const key = match[2]
      if (isDynamicTrailingDot(key)) continue
      const rel = file.replace(SRC_DIR + '/', '')
      const existing = refs.get(key)
      if (existing) {
        if (!existing.includes(rel)) existing.push(rel)
      } else {
        refs.set(key, [rel])
      }
    }
  }
  return refs
}

describe('every referenced i18n key resolves in the active modular locale tree', () => {
  const referenced = extractReferencedKeys()
  const enKeys = flattenKeys(en as Record<string, unknown>)
  const zhKeys = flattenKeys(zh as Record<string, unknown>)

  it('found a substantial number of referenced keys (extraction sanity check)', () => {
    // Guards against the extraction regex silently breaking (e.g. after a
    // refactor) and this whole test suite going vacuously green.
    expect(referenced.size).toBeGreaterThan(1000)
  })

  it('zh: no referenced key is missing (excluding locale-conditional home.resources.*)', () => {
    const missing: string[] = []
    for (const [key, files] of referenced) {
      if (isHomeResources(key)) continue
      if (!zhKeys.has(key)) missing.push(`${key} (${files.join(', ')})`)
    }
    expect(missing).toEqual([])
  })

  it('en: no referenced key is missing (excluding locale-conditional home.resources.*)', () => {
    const missing: string[] = []
    for (const [key, files] of referenced) {
      if (isHomeResources(key)) continue
      if (!enKeys.has(key)) missing.push(`${key} (${files.join(', ')})`)
    }
    expect(missing).toEqual([])
  })

  it('home.resources.*: every referenced key resolves in at least its own locale', () => {
    const missing: string[] = []
    for (const [key, files] of referenced) {
      if (!isHomeResources(key)) continue
      if (!enKeys.has(key) && !zhKeys.has(key)) missing.push(`${key} (${files.join(', ')})`)
    }
    expect(missing).toEqual([])
  })
})
