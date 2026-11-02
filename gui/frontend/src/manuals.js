// The user manuals on the Help page: Markdown built into the app (gui/manuals.go)
// rendered with marked. Links between the manuals stay in the app; links to
// other files in the repository open on GitHub at this version.
import { Marked } from 'marked'

export const REPO = 'https://github.com/mjb-is/proxmoxbackupclient_go'

// Same ids as gui/manuals.go; path is the file's place in the repository.
export const MANUALS = [
  { id: 'gui', path: 'docs/manual/GUI.md', label: 'manualGui' },
  { id: 'cli', path: 'docs/manual/CLI.md', label: 'manualCli' },
  { id: 'bmr', path: 'docs/manual/BMR.md', label: 'manualBmr' },
  { id: 'restore', path: 'docs/RESTORE_GUIDE.md', label: 'manualRestore' },
]

// GitHub's heading ids, so the manuals' own "#9-restore" links work:
// lower case, punctuation dropped, spaces to hyphens, repeats get -1, -2.
export function makeSlugger() {
  const seen = new Map()
  return (text) => {
    const base = text.toLowerCase().trim()
      .replace(/[^\p{L}\p{M}\p{N}\p{Pc}\- ]/gu, '')
      .replace(/ /g, '-')
    const n = seen.get(base)
    seen.set(base, n === undefined ? 0 : n + 1)
    return n === undefined ? base : `${base}-${n + 1}`
  }
}

const unescape = (s) => s.replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>')
  .replace(/&quot;/g, '"').replace(/&#39;/g, "'")
const attr = (s) => String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')

// "v0.9.1" for a release, master for a development build.
export function repoRef(appVersion) {
  return /^\d+\.\d+\.\d+$/.test(appVersion || '') ? `v${appVersion}` : 'master'
}

// Where a link in the manual at docPath goes: another place in the manuals
// ({ doc, anchor }), or a web address ({ url }).
export function resolveLink(href, docPath, ref) {
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return { url: href }
  const self = MANUALS.find((m) => m.path === docPath)
  if (href.startsWith('#')) return { doc: self.id, anchor: decodeURIComponent(href.slice(1)) }
  const u = new URL(href, `https://repo/${docPath}`)
  const path = u.pathname.replace(/^\//, '')
  const target = MANUALS.find((m) => m.path === path)
  const anchor = u.hash ? decodeURIComponent(u.hash.slice(1)) : ''
  if (target) return { doc: target.id, anchor }
  return { url: `${REPO}/blob/${ref}/${path}${u.hash}` }
}

// Markdown to HTML, plus the h2 headings for the contents list.
export function renderManual(md, docId, ref) {
  const doc = MANUALS.find((m) => m.id === docId)
  const slug = makeSlugger()
  const headings = []
  const marked = new Marked({
    gfm: true,
    renderer: {
      heading({ tokens, depth }) {
        const html = this.parser.parseInline(tokens)
        const id = slug(unescape(this.parser.parseInline(tokens, this.parser.textRenderer)))
        if (depth === 2) headings.push({ id, text: unescape(this.parser.parseInline(tokens, this.parser.textRenderer)) })
        return `<h${depth} id="${attr(id)}">${html}</h${depth}>\n`
      },
      link({ href, title, tokens }) {
        const text = this.parser.parseInline(tokens)
        const t = title ? ` title="${attr(title)}"` : ''
        const to = resolveLink(href, doc.path, ref)
        if (to.url) return `<a href="${attr(to.url)}" target="_blank" rel="noopener noreferrer" data-external="true"${t}>${text}</a>`
        return `<a href="#" data-doc="${attr(to.doc)}" data-anchor="${attr(to.anchor)}"${t}>${text}</a>`
      },
    },
  })
  // The manuals open with a hand-written contents list; the page has its own.
  return { html: marked.parse(md.replace(/\r\n/g, '\n')), headings: headings.filter((h) => h.id !== 'contents') }
}
