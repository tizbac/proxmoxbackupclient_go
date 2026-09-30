import { useEffect, useState } from 'react'

const { GetBMRGuideContent, DownloadBMRGuide } = window.go?.main?.App || {}

// Tiny hand-rolled Markdown -> JSX renderer, just enough for this one guide
// (headings, hr, blockquotes, checkboxes, bullet/numbered lists, bold, code
// spans, links) — not a general-purpose parser, and deliberately not a new
// npm dependency for a single static document.
function renderInline(text, keyPrefix) {
  const nodes = []
  const pattern = /\*\*(.+?)\*\*|`(.+?)`|\[(.+?)\]\((.+?)\)/g
  let last = 0
  let m
  let i = 0
  while ((m = pattern.exec(text)) !== null) {
    if (m.index > last) nodes.push(text.slice(last, m.index))
    if (m[1] !== undefined) {
      nodes.push(<strong key={`${keyPrefix}-${i++}`}>{m[1]}</strong>)
    } else if (m[2] !== undefined) {
      nodes.push(<code key={`${keyPrefix}-${i++}`} style={{ background: '#f1f3f5', padding: '1px 5px', borderRadius: '3px' }}>{m[2]}</code>)
    } else {
      nodes.push(<a key={`${keyPrefix}-${i++}`} href={m[4]} target="_blank" rel="noreferrer">{m[3]}</a>)
    }
    last = pattern.lastIndex
  }
  if (last < text.length) nodes.push(text.slice(last))
  return nodes
}

function renderMarkdown(md) {
  const lines = md.split('\n')
  const blocks = []
  let list = null // { type: 'ul'|'ol', items: [] }

  const flushList = () => {
    if (!list) return
    const Tag = list.type
    blocks.push(
      <Tag key={`list-${blocks.length}`} style={{ paddingLeft: '22px', display: 'flex', flexDirection: 'column', gap: '6px', margin: '8px 0' }}>
        {list.items.map((item, idx) => <li key={idx}>{item}</li>)}
      </Tag>
    )
    list = null
  }

  lines.forEach((raw, idx) => {
    const line = raw.trimEnd()

    if (line.trim() === '') { flushList(); return }
    if (line.trim() === '---') { flushList(); blocks.push(<hr key={`hr-${idx}`} style={{ margin: '18px 0', border: 'none', borderTop: '1px solid #e2e8f0' }} />); return }

    const heading = line.match(/^(#{1,3})\s+(.*)/)
    if (heading) {
      flushList()
      const level = heading[1].length
      const size = level === 1 ? '20px' : level === 2 ? '17px' : '15px'
      const Tag = `h${Math.min(level + 2, 6)}`
      blocks.push(<Tag key={`h-${idx}`} style={{ fontSize: size, margin: '16px 0 8px' }}>{renderInline(heading[2], `h-${idx}`)}</Tag>)
      return
    }

    const quote = line.match(/^>\s?(.*)/)
    if (quote) {
      flushList()
      blocks.push(
        <div key={`q-${idx}`} style={{ borderLeft: '3px solid #cbd5e0', paddingLeft: '12px', color: '#4a5568', margin: '8px 0', fontStyle: 'italic' }}>
          {renderInline(quote[1], `q-${idx}`)}
        </div>
      )
      return
    }

    const checkbox = line.match(/^-\s+\[([ xX])\]\s+(.*)/)
    if (checkbox) {
      if (!list || list.type !== 'ul') { flushList(); list = { type: 'ul', items: [] } }
      list.items.push(
        <span style={{ listStyle: 'none' }}>
          <input type="checkbox" checked={checkbox[1].toLowerCase() === 'x'} readOnly style={{ marginRight: '6px' }} />
          {renderInline(checkbox[2], `cb-${idx}`)}
        </span>
      )
      return
    }

    const bullet = line.match(/^[-*]\s+(.*)/)
    if (bullet) {
      if (!list || list.type !== 'ul') { flushList(); list = { type: 'ul', items: [] } }
      list.items.push(renderInline(bullet[1], `b-${idx}`))
      return
    }

    const numbered = line.match(/^\d+\.\s+(.*)/)
    if (numbered) {
      if (!list || list.type !== 'ol') { flushList(); list = { type: 'ol', items: [] } }
      list.items.push(renderInline(numbered[1], `n-${idx}`))
      return
    }

    flushList()
    blocks.push(<p key={`p-${idx}`} style={{ margin: '8px 0', lineHeight: 1.5 }}>{renderInline(line, `p-${idx}`)}</p>)
  })
  flushList()
  return blocks
}

export default function BMRGuideModal({ onClose }) {
  const [content, setContent] = useState(null)
  const [saveMsg, setSaveMsg] = useState('')

  useEffect(() => {
    GetBMRGuideContent?.().then(setContent).catch(() => setContent('Failed to load guide.'))
  }, [])

  const handleDownload = async () => {
    setSaveMsg('')
    try {
      const path = await DownloadBMRGuide?.()
      if (path) setSaveMsg(`Saved to ${path}`)
    } catch (e) {
      setSaveMsg(`Failed to save: ${e}`)
    }
  }

  return (
    <div
      style={{
        position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.45)',
        display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
      }}
      onClick={onClose}
    >
      <div
        className="card"
        style={{ maxWidth: '760px', width: '92%', maxHeight: '86vh', margin: 0, display: 'flex', flexDirection: 'column' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px', flexShrink: 0 }}>
          <h3 style={{ margin: 0 }}>🛠️ Bare Metal Restore Guide</h3>
          <div style={{ display: 'flex', gap: '8px' }}>
            <button className="btn btn-secondary" onClick={handleDownload}>Download…</button>
            <button className="btn btn-secondary" onClick={onClose} style={{ padding: '4px 10px' }}>✕</button>
          </div>
        </div>
        {saveMsg && <div style={{ fontSize: '13px', color: '#2f855a', marginBottom: '8px' }}>{saveMsg}</div>}
        <div style={{ overflowY: 'auto', paddingRight: '8px' }}>
          {content === null ? 'Loading…' : renderMarkdown(content)}
        </div>
      </div>
    </div>
  )
}
