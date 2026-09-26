import { useTranslation } from '../i18n/i18nContext'
import { useState } from 'react'

function LanguageSwitcher() {
  const { language, setLanguage } = useTranslation()
  const [isOpen, setIsOpen] = useState(false)

  // Sorted alphabetically by native name — same convention as before, just
  // extended. Portuguese/Dutch each have two plausible flags (PT/BR,
  // NL/BE); picked the European-market one deliberately rather than
  // leaving it arbitrary.
  const languages = [
    { code: 'bg', name: 'Български', flag: '🇧🇬' },
    { code: 'cs', name: 'Čeština', flag: '🇨🇿' },
    { code: 'de', name: 'Deutsch', flag: '🇩🇪' },
    { code: 'en', name: 'English', flag: '🇬🇧' },
    { code: 'es', name: 'Español', flag: '🇪🇸' },
    { code: 'el', name: 'Ελληνικά', flag: '🇬🇷' },
    { code: 'fr', name: 'Français', flag: '🇫🇷' },
    { code: 'hu', name: 'Magyar', flag: '🇭🇺' },
    { code: 'it', name: 'Italiano', flag: '🇮🇹' },
    { code: 'lv', name: 'Latviešu', flag: '🇱🇻' },
    { code: 'lt', name: 'Lietuvių', flag: '🇱🇹' },
    { code: 'nl', name: 'Nederlands', flag: '🇳🇱' },
    { code: 'pl', name: 'Polski', flag: '🇵🇱' },
    { code: 'pt', name: 'Português', flag: '🇵🇹' },
    { code: 'ro', name: 'Română', flag: '🇷🇴' },
    { code: 'sk', name: 'Slovenčina', flag: '🇸🇰' },
    { code: 'tr', name: 'Türkçe', flag: '🇹🇷' },
    { code: 'uk', name: 'Українська', flag: '🇺🇦' },
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
    <span>{currentLanguage.flag}</span>
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
        <span>{lang.flag}</span>
        <span>{lang.name}</span>
        </button>
      ))}
      </div>
    )}
    </div>
  )
}

export default LanguageSwitcher
