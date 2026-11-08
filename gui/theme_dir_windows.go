//go:build windows

package main

// userThemeDir is the user's own folder for their preferences:
// %APPDATA%\Salvanza (the brand's; roams with the profile).
func userThemeDir() (string, error) {
	return roamingAppDir(), nil
}
