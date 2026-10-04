import { useTranslation } from '../i18n/i18nContext'
import { useState } from 'react'

// Flags are bundled SVG images: Windows ships no flag emoji glyphs, so the
// emoji rendered as bare two-letter codes there while Linux showed flags.
const flagUrls = import.meta.glob('../assets/flags/*.svg', { eager: true, query: '?url', import: 'default' })
const Flag = ({ code }) => (
  <img src={flagUrls[`../assets/flags/${code}.svg`]} alt="" width="20" height="15"
    style={{ borderRadius: '2px', boxShadow: '0 0 0 1px rgba(0,0,0,0.15)', flexShrink: 0 }} />
)

function LanguageSwitcher() {
  const { language, setLanguage } = useTranslation()
  const [isOpen, setIsOpen] = useState(false)

  // Sorted alphabetically by native name — same convention as before, just
  // extended. Portuguese/Dutch each have two plausible flags (PT/BR,
  // NL/BE); picked the European-market one deliberately rather than
  // leaving it arbitrary.
  const languages = [
    { code: 'bg', name: 'Български', flag: 'bg' },
    { code: 'cs', name: 'Čeština', flag: 'cz' },
    { code: 'de', name: 'Deutsch', flag: 'de' },
    { code: 'en', name: 'English', flag: 'gb' },
    { code: 'es', name: 'Español', flag: 'es' },
    { code: 'el', name: 'Ελληνικά', flag: 'gr' },
    { code: 'fr', name: 'Français', flag: 'fr' },
    { code: 'hu', name: 'Magyar', flag: 'hu' },
    { code: 'it', name: 'Italiano', flag: 'it' },
    { code: 'lv', name: 'Latviešu', flag: 'lv' },
    { code: 'lt', name: 'Lietuvių', flag: 'lt' },
    { code: 'nl', name: 'Nederlands', flag: 'nl' },
    { code: 'pl', name: 'Polski', flag: 'pl' },
    { code: 'pt', name: 'Português', flag: 'pt' },
    { code: 'ro', name: 'Română', flag: 'ro' },
    { code: 'sk', name: 'Slovenčina', flag: 'sk' },
    { code: 'tr', name: 'Türkçe', flag: 'tr' },
    { code: 'uk', name: 'Українська', flag: 'ua' },
  ]

  const currentLanguage = languages.find(lang => lang.code === language) || languages[0]

  const handleSelect = (code) => {
    setLanguage(code)
    setIsOpen(false)
  }

  return (
    <div style={{
      position: 'relative',
      display: 'inline-block'
    }}>
    <button
    onClick={() => setIsOpen(!isOpen)}
    style={{
      display: 'flex',
      alignItems: 'center',
      gap: '8px',
      padding: '8px 16px',
      border: '1px solid #ddd',
      borderRadius: '8px',
      backgroundColor: '#f8f9fa',
      cursor: 'pointer',
      fontWeight: '500',
      color: '#4a5568',
      transition: 'all 0.2s'
    }}
    >
    <Flag code={currentLanguage.flag} />
    <span>{currentLanguage.name}</span>
    <span>{isOpen ? '▲' : '▼'}</span>
    </button>

    {isOpen && (
      <div style={{
        position: 'absolute',
        top: '100%',
        left: 0,
        // Fixed to fit the longest native name (Nederlands/Slovenčina/
        // Українська) regardless of which language is currently selected —
        // not tied to the trigger button's own width, so the panel never
        // has to wrap or crowd its longest entries.
        width: '210px',
        maxHeight: '70vh',
        overflowY: 'auto',
        backgroundColor: 'white',
        border: '1px solid #ddd',
        borderRadius: '8px',
        boxShadow: '0 2px 10px rgba(0,0,0,0.1)',
                zIndex: 100,
                marginTop: '4px'
      }}>
      {languages.map((lang) => (
        <button
        key={lang.code}
        onClick={() => handleSelect(lang.code)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          width: '100%',
          padding: '5px 16px',
          border: 'none',
          backgroundColor: 'transparent',
          cursor: 'pointer',
          textAlign: 'left',
          color: language === lang.code ? 'var(--accent)' : '#4a5568',
          fontWeight: language === lang.code ? 'bold' : 'normal',
          transition: 'all 0.2s'
        }}
        >
        <Flag code={lang.flag} />
        <span>{lang.name}</span>
        </button>
      ))}
      </div>
    )}
    </div>
  )
}

export default LanguageSwitcher
