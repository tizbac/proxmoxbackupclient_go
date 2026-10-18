//go:build (!linux || !cgo) && !service

package main

// On Windows the app's window (WebView2) may use the camera already: Wails
// allows its permission requests. See camera_linux.go.

func prepareCamera() error { return nil }

func allowCamera(on bool) {}
