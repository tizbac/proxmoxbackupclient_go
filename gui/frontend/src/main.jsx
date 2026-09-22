import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'
import { I18nProvider } from './i18n/i18nContext'
import { FluentProvider, webLightTheme } from '@fluentui/react-components'

// FluentProvider is required for any @fluentui/react-components component
// (DirectoryTree's folder tree) to render styled at all — it supplies the
// Griffel theme context those components read from. webLightTheme matches
// the rest of this app's plain light styling; there's no dark mode to match
// here yet.
ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <FluentProvider theme={webLightTheme}>
      <I18nProvider>
        <App />
      </I18nProvider>
    </FluentProvider>
  </React.StrictMode>,
)
