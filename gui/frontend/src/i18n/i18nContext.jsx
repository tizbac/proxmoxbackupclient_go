import { createContext, useContext, useState, useEffect } from 'react'
import translations from './translations'

const I18nContext = createContext()

// Dates and numbers follow the app's language, not the computer's: a
// toLocaleString / toLocaleDateString / toLocaleTimeString call with no
// locale (or []) uses appLocale. When the app language is the computer's own
// language, the computer's locale is kept (en-GB dates on a British PC).
let appLocale
const withAppLocale = (orig) => function (locales, options) {
  const none = locales === undefined || (Array.isArray(locales) && locales.length === 0)
  return orig.call(this, none && appLocale ? appLocale : locales, options)
}
for (const [proto, names] of [
  [Date.prototype, ['toLocaleString', 'toLocaleDateString', 'toLocaleTimeString']],
  [Number.prototype, ['toLocaleString']],
]) {
  for (const n of names) proto[n] = withAppLocale(proto[n])
}
function localeFor(language) {
  const os = navigator.language || ''
  return os.split('-')[0] === language ? os : language
}

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

  // Set while rendering, so the children rendered below already use it.
  appLocale = localeFor(language)

  useEffect(() => {
    localStorage.setItem('language', language)
  }, [language])

  const t = (key, params) => {
    const keys = key.split('.')
    const lookup = (lang) => {
      let v = translations[lang]
      for (const k of keys) {
        if (v && typeof v === 'object') {
          v = v[k]
        } else {
          return undefined
        }
      }
      return v
    }

    // A string missing from this language shows in English, not as its raw key.
    let value = lookup(language)
    if (value === undefined && language !== 'en') value = lookup('en')
    if (value === undefined) return key // Return key if translation not found

    if (typeof value === 'string' && params) {
      for (const [p, v] of Object.entries(params)) {
        value = value.split(`{${p}}`).join(String(v))
      }
    }

    // Only a genuinely missing key falls back to its own name — an
    // intentionally empty string (e.g. a since-removed line like the old
    // "basedOn") must still render as empty, not as the raw key. Found
    // live 2026-09-25: the About page's "basedOn" showed the literal text
    // "basedOn" on screen because this used `value || key`, which treats
    // any falsy value (including "") the same as a missing one.
    return value === undefined ? key : value
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
