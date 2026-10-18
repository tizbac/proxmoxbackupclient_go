import { useEffect, useRef, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// PaperKeyDialog shows an encryption key laid out for printing (from the
// backend's EncryptionPaperKey): fingerprint, the key file's text and a QR
// code of it. Print sends only the page below to the printer, through a
// hidden frame, so the app around it is not printed. print() does not wait
// for the print dialog (WebView2 returns at once and prints when the user
// clicks Print in its preview), so the frame stays until the dialog closes:
// removed earlier, the job has nothing to print and fails.
//
// Where the app's window cannot print (WebView2 while the app runs as
// administrator, paperKey.print_in_browser), the same page is handed to the
// backend, which opens it once in the user's web browser; there it asks to
// print as soon as it has loaded, with a note on screen only.

const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))

function resolvePrintInBrowser() {
  if (window.go && window.go.main && window.go.main.App) return window.go.main.App.PrintPaperKeyInBrowser
  return undefined
}

function printablePage(pk, t, inBrowser = false) {
  const rows = [
    [t('paperKeyFingerprint'), pk.fingerprint],
    [t('paperKeyCreated'), pk.created],
    [t('paperKeyComputer'), pk.host],
    [t('paperKeyFile'), pk.path],
    [t('paperKeyPrinted'), pk.printed],
  ].filter(([, v]) => v)
  return `<!doctype html><html><head><meta charset="utf-8"><title>${esc(t('paperKeyTitle'))}</title>
<style>
  @page { margin: 0; } /* no browser header and footer (date, title, address) */
  body { font-family: Segoe UI, Arial, sans-serif; margin: 0; padding: 18mm; color: #000; }
  h1 { font-size: 20px; margin: 0 0 6px; }
  p { font-size: 12px; line-height: 1.45; margin: 4px 0 10px; }
  table { border-collapse: collapse; font-size: 12px; margin: 8px 0 12px; }
  td { padding: 2px 12px 2px 0; vertical-align: top; }
  td:first-child { color: #444; white-space: nowrap; }
  .fp { font-family: Consolas, monospace; font-weight: 600; }
  img { width: 70mm; height: 70mm; image-rendering: pixelated; display: block; margin: 6px 0 10px; }
  pre { font-family: Consolas, monospace; font-size: 9.5px; white-space: pre-wrap; word-break: break-all; border: 1px solid #999; padding: 8px; margin: 0; }
  .warn { font-weight: 600; }
  .note { font-size: 14px; background: #fff4d6; border: 1px solid #e0b84c; padding: 8px 12px; margin: 0 0 14px; }
  @media print { .note { display: none; } }
</style></head><body>
${inBrowser ? `<p class="note">${esc(t('paperKeyBrowserTabNote'))}</p>` : ''}
<h1>${esc(t('paperKeyTitle'))}</h1>
<p>${esc(t('paperKeyIntro'))}</p>
<p class="warn">${esc(pk.passphrase_protected ? t('paperKeyProtected') : t('paperKeyUnprotected'))}</p>
<table>${rows.map(([k, v]) => `<tr><td>${esc(k)}</td><td${k === t('paperKeyFingerprint') ? ' class="fp"' : ''}>${esc(v)}</td></tr>`).join('')}</table>
<img alt="QR" src="data:image/png;base64,${esc(pk.qr_png)}">
<pre>${esc(pk.key_text)}</pre>
${inBrowser ? '<script>addEventListener("load", function () { setTimeout(function () { window.print() }, 300) })</script>' : ''}
</body></html>`
}

export default function PaperKeyDialog({ paperKey, onClose }) {
  const { t } = useTranslation()
  const closeRef = useRef(null)
  const frameRef = useRef(null)
  const [browserMsg, setBrowserMsg] = useState(null)
  const [busy, setBusy] = useState(false)
  // busyRef guards at once: a second click can arrive before React has
  // re-rendered the button as disabled (tested: two quick clicks opened two
  // browser tabs with only the state).
  const busyRef = useRef(false)

  useEffect(() => {
    if (closeRef.current) closeRef.current.focus()
    return () => {
      if (frameRef.current) frameRef.current.remove()
    }
  }, [])

  const handlePrint = async () => {
    const inBrowser = paperKey.print_in_browser && resolvePrintInBrowser()
    if (inBrowser) {
      // One page per click: a second click while the browser opens would
      // serve the key a second time.
      if (busyRef.current) return
      busyRef.current = true
      setBusy(true)
      try {
        await inBrowser(printablePage(paperKey, t, true))
        setBrowserMsg({ ok: true, text: t('paperKeyBrowserOpened') })
        await new Promise((r) => setTimeout(r, 3000))
      } catch (err) {
        setBrowserMsg({ ok: false, text: t('paperKeyBrowserFailed').replace('{err}', () => String(err)) })
      } finally {
        busyRef.current = false
        setBusy(false)
      }
      return
    }
    if (frameRef.current) frameRef.current.remove()
    const frame = document.createElement('iframe')
    frame.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0'
    frame.setAttribute('aria-hidden', 'true')
    frame.srcdoc = printablePage(paperKey, t)
    frame.onload = () => {
      frame.contentWindow.focus()
      frame.contentWindow.print()
    }
    frameRef.current = frame
    document.body.appendChild(frame)
  }

  const pk = paperKey
  return (
    <div
      style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1250, padding: '16px' }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}
      onKeyDown={(e) => { if (e.key === 'Escape') { e.preventDefault(); onClose() } }}
    >
      <div role="dialog" aria-modal="true" aria-labelledby="paper-key-title"
        style={{ maxWidth: '640px', width: '100%', background: '#fff', borderRadius: '8px', overflow: 'hidden', boxShadow: '0 12px 44px rgba(0,0,0,.35)' }}>
        <div id="paper-key-title" style={{ background: 'var(--accent)', color: '#fff', padding: '12px 18px', fontSize: '16px', fontWeight: 600, display: 'flex', gap: '10px' }}>
          <span aria-hidden="true">🔑</span><span>{t('paperKeyTitle')}</span>
        </div>
        <div style={{ padding: '18px', color: '#2d3748', fontSize: '14px', lineHeight: 1.5, maxHeight: '65vh', overflowY: 'auto' }}>
          <p style={{ margin: '0 0 8px' }}>{t('paperKeyIntro')}</p>
          <p style={{ margin: '0 0 12px', fontWeight: 600, color: pk.passphrase_protected ? '#2d3748' : '#c62828' }}>
            {pk.passphrase_protected ? t('paperKeyProtected') : t('paperKeyUnprotected')}
          </p>
          <div style={{ display: 'flex', gap: '16px', alignItems: 'flex-start', flexWrap: 'wrap' }}>
            <img alt="QR" src={`data:image/png;base64,${pk.qr_png}`} style={{ width: '180px', height: '180px', imageRendering: 'pixelated', border: '1px solid #e2e8f0' }} />
            <dl style={{ display: 'grid', gridTemplateColumns: 'max-content 1fr', gap: '4px 12px', margin: 0, fontSize: '13px', flex: '1 1 240px', minWidth: 0 }}>
              <dt style={{ color: '#64748b' }}>{t('paperKeyFingerprint')}</dt>
              <dd style={{ margin: 0, fontFamily: 'Consolas, monospace', fontWeight: 600, wordBreak: 'break-all' }}>{pk.fingerprint}</dd>
              {pk.created && (<><dt style={{ color: '#64748b' }}>{t('paperKeyCreated')}</dt><dd style={{ margin: 0 }}>{pk.created}</dd></>)}
              <dt style={{ color: '#64748b' }}>{t('paperKeyFile')}</dt>
              <dd style={{ margin: 0, wordBreak: 'break-all' }}>{pk.path}</dd>
            </dl>
          </div>
          {browserMsg && (
            <p role="status" style={{ margin: '12px 0 0', fontSize: '13px', color: browserMsg.ok ? '#2d3748' : '#c62828', background: browserMsg.ok ? '#f1f5f9' : '#fdecea', padding: '8px 10px', borderRadius: '4px' }}>
              {browserMsg.text}
            </p>
          )}
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', padding: '0 18px 18px', flexWrap: 'wrap' }}>
          <button ref={closeRef} type="button" className="btn btn-secondary" style={{ margin: 0 }} onClick={onClose}>{t('paperKeyClose')}</button>
          <button type="button" className="btn" style={{ margin: 0 }} onClick={handlePrint} disabled={busy}>🖨️ {t('paperKeyPrint')}</button>
        </div>
      </div>
    </div>
  )
}
