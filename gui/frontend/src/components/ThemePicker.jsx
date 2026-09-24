import { useEffect, useState } from 'react'
import { useTranslation } from '../i18n/i18nContext'

// Theme = the app's accent color plus the hero banner's 3-stop gradient
// (dark end -> accent, at 60% -> light end). Presets mirror the mockup
// Mick approved; "amber" matches the app's own long-standing default so
// picking it is always a safe no-op against a brand's own accent.
const PRESETS = {
  amber: { accent: '#e87003', accentHover: '#d46100', heroStart: '#5c3a1e', heroEnd: '#fbe9d7' },
  blue: { accent: '#3d5aa8', accentHover: '#2f4680', heroStart: '#1e2a4a', heroEnd: '#e9edf7' },
  green: { accent: '#2e8c47', accentHover: '#256f38', heroStart: '#1e3a24', heroEnd: '#e6f5ea' },
  red: { accent: '#b3261e', accentHover: '#8f1e18', heroStart: '#3a0d0a', heroEnd: '#fce4e1' },
  dark: { accent: '#2b2b2b', accentHover: '#1a1a1a', heroStart: '#0d0d0d', heroEnd: '#4a4a4a' },
}

const HEX_RE = /^#[0-9a-fA-F]{6}$/

// Accepts "#b3261e", "b3261e", or either with stray whitespace — typing the
// hex digits alone (no leading #) shouldn't silently do nothing.
function normalizeHex(value) {
  const trimmed = value.trim()
  const withHash = trimmed.startsWith('#') ? trimmed : `#${trimmed}`
  return HEX_RE.test(withHash) ? withHash : null
}

function gradientFor(colors) {
  return `linear-gradient(90deg, ${colors.heroStart} 0%, ${colors.accent} 60%, ${colors.heroEnd} 100%)`
}

function applyTheme(colors) {
  const root = document.documentElement.style
  root.setProperty('--accent', colors.accent)
  root.setProperty('--accent-hover', colors.accentHover)
  root.setProperty('--hero-start', colors.heroStart)
  root.setProperty('--hero-end', colors.heroEnd)
}

// Theme is persisted via the Go backend (config.json, alongside every other
// setting) rather than browser localStorage — see gui/theme.go. Accessed as
// a plain global rather than threaded through App.jsx's props, since Theme
// is a self-contained concern nothing else in the app depends on.
function backendGetTheme() {
  return window.go?.main?.App?.GetTheme ? window.go.main.App.GetTheme() : Promise.resolve(null)
}
function backendSaveTheme(theme) {
  if (window.go?.main?.App?.SaveTheme) window.go.main.App.SaveTheme(theme)
}

// Resolves true if a theme has already been saved. Exported so App.jsx's
// brand-loading effect can skip applying the brand's own default accent when
// a saved theme should take precedence — a saved theme choice always wins,
// regardless of which effect happens to run first.
export async function hasStoredTheme() {
  const saved = await backendGetTheme()
  return !!(saved && saved.preset)
}

// Resolves a stored theme's colors and applies them (CSS vars) — the actual
// work ThemePicker's own mount-effect does, extracted so App.jsx's startup
// effect can do it too. Without this, a saved theme only ever took effect
// once the user happened to open the Preferences > Theme tab and mount
// ThemePicker itself — on a fresh launch the UI just sat on CSS defaults
// (or the brand's own accent) until then, even though hasStoredTheme()
// correctly reported a theme was saved and App.jsx correctly skipped
// applying the brand's default because of it. Confirmed live 2026-09-24:
// "the correct theme shows selected the moment I open the Theme tab, and
// the UI colors only update right then" — because that mount was genuinely
// the first time anything ever called applyTheme() this session.
export async function applyStoredTheme() {
  const saved = await backendGetTheme()
  if (!saved || !saved.preset) return
  const customColors = {
    accent: saved.accent || PRESETS.amber.accent,
    accentHover: saved.accentHover || PRESETS.amber.accentHover,
    heroStart: saved.heroStart || PRESETS.amber.heroStart,
    heroEnd: saved.heroEnd || PRESETS.amber.heroEnd,
  }
  const colors = saved.preset === 'custom' ? customColors : PRESETS[saved.preset]
  if (colors) applyTheme(colors)
}

export default function ThemePicker() {
  const { t } = useTranslation()
  const [preset, setPreset] = useState('amber')
  const [custom, setCustom] = useState(PRESETS.amber)

  // Load whatever was saved (if anything) once, on mount, and actually apply
  // it — App.jsx's own brand-loading effect only SKIPS setting its default
  // accent when a theme is stored (see hasStoredTheme); it never applies one
  // itself, so this is the one place a saved theme takes effect on startup.
  useEffect(() => {
    let cancelled = false
    backendGetTheme().then((saved) => {
      if (cancelled || !saved || !saved.preset) return
      setPreset(saved.preset)
      const customColors = {
        accent: saved.accent || PRESETS.amber.accent,
        accentHover: saved.accentHover || PRESETS.amber.accentHover,
        heroStart: saved.heroStart || PRESETS.amber.heroStart,
        heroEnd: saved.heroEnd || PRESETS.amber.heroEnd,
      }
      const colors = saved.preset === 'custom' ? customColors : PRESETS[saved.preset]
      if (saved.preset === 'custom') setCustom(customColors)
      if (colors) applyTheme(colors)
    }).catch(() => {
      // Backend unavailable (dev preview without Wails) — keep CSS defaults.
    })
    return () => { cancelled = true }
  }, [])

  const save = (nextPreset, nextCustom) => {
    backendSaveTheme({ preset: nextPreset, ...nextCustom })
  }

  const pick = (name) => {
    setPreset(name)
    const colors = name === 'custom' ? custom : PRESETS[name]
    applyTheme(colors)
    save(name, custom)
  }

  const editCustomField = (field, value) => {
    const next = { ...custom, [field]: value }
    setCustom(next)
    // Only apply/persist a well-formed hex — a half-typed value stays in the
    // input but doesn't flash broken colors across the app while typing.
    // The leading "#" is optional here: typing just the 6 digits still works.
    const normalized = normalizeHex(value)
    if (normalized) {
      const applied = { ...next, [field]: normalized }
      applyTheme(applied)
      save('custom', applied)
    }
  }

  const activeColors = preset === 'custom' ? custom : PRESETS[preset]

  return (
    <div className="card" style={{ marginBottom: '20px' }}>
      <h3 style={{ marginTop: 0 }}>{t('themeTitle')}</h3>
      <p style={{ color: '#718096', fontSize: '13px', marginBottom: '14px' }}>{t('themeIntro')}</p>

      <div style={{ display: 'flex', gap: '14px', flexWrap: 'wrap' }}>
        {[...Object.keys(PRESETS), 'custom'].map((name) => (
          <div
            key={name}
            onClick={() => pick(name)}
            style={{
              border: `2px solid ${preset === name ? 'var(--accent)' : '#ddd'}`,
              boxShadow: preset === name ? '0 0 0 2px rgba(232,112,3,0.15)' : 'none',
              borderRadius: '8px',
              padding: '10px',
              width: '140px',
              cursor: 'pointer',
              textAlign: 'center',
              backgroundColor: '#fff',
            }}
          >
            <div
              style={{
                height: '32px',
                borderRadius: '5px',
                marginBottom: '8px',
                background: name === 'custom' ? gradientFor(custom) : gradientFor(PRESETS[name]),
                border: name === 'custom' ? '1px dashed #999' : 'none',
              }}
            />
            <div style={{ fontSize: '12px', fontWeight: 600, color: '#333' }}>
              {t(`theme${name.charAt(0).toUpperCase()}${name.slice(1)}`)}
            </div>
          </div>
        ))}
      </div>

      {preset === 'custom' && (
        <div style={{
          marginTop: '18px', padding: '14px', backgroundColor: '#fafafa',
          border: '1px solid #e2e2e2', borderRadius: '6px',
          display: 'flex', gap: '18px', flexWrap: 'wrap',
        }}>
          {[
            ['accent', 'themeFieldAccent'],
            ['accentHover', 'themeFieldAccentHover'],
            ['heroStart', 'themeFieldHeroStart'],
            ['heroEnd', 'themeFieldHeroEnd'],
          ].map(([field, labelKey]) => (
            <div key={field}>
              <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: '#333', marginBottom: '4px' }}>
                {t(labelKey)}
              </label>
              <input
                value={custom[field]}
                onChange={(e) => editCustomField(field, e.target.value)}
                placeholder="#rrggbb"
                style={{ width: '110px', padding: '6px 9px', fontSize: '13px', border: '1px solid #ccc', borderRadius: '4px' }}
              />
            </div>
          ))}
          <div style={{ fontSize: '11px', color: '#999', width: '100%' }}>{t('themeCustomHint')}</div>
        </div>
      )}

      <div style={{ marginTop: '18px' }}>
        <div style={{ fontSize: '12px', fontWeight: 700, color: '#555', marginBottom: '6px' }}>{t('themePreview')}</div>
        <div style={{
          background: gradientFor(activeColors), color: '#fff', borderRadius: '6px',
          padding: '14px 18px', display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        }}>
          <div style={{ fontSize: '16px', fontWeight: 700, fontStyle: 'italic' }}>Proxmox Backup Client — GO</div>
          <div style={{ fontSize: '14px', fontWeight: 700 }}>{t('tabBackup').toUpperCase()}</div>
        </div>
      </div>
    </div>
  )
}
