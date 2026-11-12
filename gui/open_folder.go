package main

// OpenLogsFolder opens the log directory in the platform's file manager.
//
// It deliberately does not go through the frontend's BrowserOpenURL: Wails
// 2.13 validates every URL and rejects a bare path or a file: URL, so the old
// "View Logs" button silently did nothing.
func (a *App) OpenLogsFolder() error {
	return openFolderInFileManager(a.GetLogsFolder())
}
