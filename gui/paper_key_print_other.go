//go:build !windows

package main

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// openInUserBrowser opens url in the default web browser.
func openInUserBrowser(ctx context.Context, url string) error {
	if ctx == nil {
		return fmt.Errorf("the window is not ready yet")
	}
	runtime.BrowserOpenURL(ctx, url)
	return nil
}
