import { createContext, useContext, useState, useEffect } from 'react'
import translations from './translations'

const I18nContext = createContext()

export function I18nProvider({ children }) {
  const [language, setLanguage] = useState(() => {
    // Check if language is already stored in localStorage
    const savedLanguage = localStorage.getItem('language')

    if (savedLanguage) {
      return savedLanguage
    }

    // Detect OS language
    const browserLanguage = navigator.language || navigator.userLanguage
    const langCode = browserLanguage.split('-')[0] // Get just the language code

    // Check if we have a translation for this language
    if (translations[langCode]) {
      return langCode
    }

    // Default to English if no translation found
    return 'en'
  })

  useEffect(() => {
    localStorage.setItem('language', language)
  }, [language])

  const t = (key) => {
    const keys = key.split('.')
    let value = translations[language]

    for (const k of keys) {
      if (value && typeof value === 'object') {
        value = value[k]
      } else {
        return key // Return key if translation not found
      }
    }

    return value || key
  }

  return (
    <I18nContext.Provider value={{ language, setLanguage, t }}>
    {children}
    </I18nContext.Provider>
  )
}

export function useTranslation() {
  const context = useContext(I18nContext)
  if (!context) {
    throw new Error('useTranslation must be used within I18nProvider')
  }
  return context
}
