import { useTranslation } from '../i18n/i18nContext'

function LanguageSwitcher() {
  const { language, setLanguage } = useTranslation()

  return (
    <div style={{
      display: 'flex',
      gap: '5px',
      alignItems: 'center',
      backgroundColor: '#f8f9fa',
      borderRadius: '8px',
      padding: '5px'
    }}>
      <button
        onClick={() => setLanguage('fr')}
        style={{
          padding: '6px 12px',
          border: 'none',
          borderRadius: '6px',
          cursor: 'pointer',
          fontWeight: language === 'fr' ? 'bold' : 'normal',
          backgroundColor: language === 'fr' ? '#667eea' : 'transparent',
          color: language === 'fr' ? 'white' : '#4a5568',
          transition: 'all 0.2s'
        }}
      >
        🇫🇷 FR
      </button>
      <button
        onClick={() => setLanguage('en')}
        style={{
          padding: '6px 12px',
          border: 'none',
          borderRadius: '6px',
          cursor: 'pointer',
          fontWeight: language === 'en' ? 'bold' : 'normal',
          backgroundColor: language === 'en' ? '#667eea' : 'transparent',
          color: language === 'en' ? 'white' : '#4a5568',
          transition: 'all 0.2s'
        }}
      >
        🇬🇧 EN
      </button>
    </div>
  )
}

export default LanguageSwitcher
