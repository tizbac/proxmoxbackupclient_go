package main

import "fmt"

// SaveTheme persists the user's Preferences > Theme choice to config.json,
// the same place every other setting lives — previously Theme was saved only
// in the frontend's browser localStorage, which doesn't survive a WebView2
// profile reset and isn't backed up alongside everything else.
func (a *App) SaveTheme(theme ThemeSettings) error {
	a.config.Theme = &theme
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}
	return nil
}

// GetTheme returns the saved Theme, or a zero-value ThemeSettings (empty
// Preset) if none has been saved yet — the frontend treats an empty Preset
// as "nothing saved, use the built-in default".
func (a *App) GetTheme() ThemeSettings {
	if a.config.Theme == nil {
		return ThemeSettings{}
	}
	return *a.config.Theme
}
