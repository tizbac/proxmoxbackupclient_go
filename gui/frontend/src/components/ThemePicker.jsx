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

const STORAGE_KEY = 'pbsTheme'
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

// Loads any saved theme preference and applies it immediately. Exported so
// App.jsx's brand-loading effect can check for a saved preference and skip
// overwriting it with the brand's own default accent — a saved theme choice
// always wins, regardless of which effect happens to run first.
export function hasStoredTheme() {
  try {
    return !!localStorage.getItem(STORAGE_KEY)
  } catch {
    return false
  }
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
    try {
      const raw = localStorage.getItem(STORAGE_KEY)
      if (!raw) return
      const saved = JSON.parse(raw)
      if (saved.preset) setPreset(saved.preset)
      const colors = saved.preset === 'custom' ? { ...PRESETS.amber, ...saved.custom } : PRESETS[saved.preset]
      if (saved.custom) setCustom({ ...PRESETS.amber, ...saved.custom })
      if (colors) applyTheme(colors)
    } catch {
      // ignore corrupt saved value, keep defaults
    }
  }, [])

  const save = (nextPreset, nextCustom) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify({ preset: nextPreset, custom: nextCustom }))
    } catch {
      // localStorage unavailable (private mode etc.) — theme still applies for this session
    }
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
      <h3 style={{ marginTop: 0 }}>🎨 {t('themeTitle')}</h3>
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
