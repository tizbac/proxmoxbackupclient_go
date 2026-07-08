import { useTranslation } from '../i18n/i18nContext'
import { useState } from 'react'

function LanguageSwitcher() {
  const { language, setLanguage } = useTranslation()
  const [isOpen, setIsOpen] = useState(false)

  const languages = [
    { code: 'en', name: 'English', flag: '🇬🇧' },
    { code: 'it', name: 'Italiano', flag: '🇮🇹' },
    { code: 'fr', name: 'Français', flag: '🇫🇷' },
    { code: 'pl', name: 'Polski', flag: '🇵🇱' },
    { code: 'de', name: 'Deutsch', flag: '🇩🇪' }
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
        width: '100%',
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
          padding: '10px 16px',
          border: 'none',
          backgroundColor: 'transparent',
          cursor: 'pointer',
          textAlign: 'left',
          color: language === lang.code ? '#667eea' : '#4a5568',
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
