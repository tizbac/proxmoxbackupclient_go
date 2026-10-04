//go:build linux && service
// +build linux,service

package main

func main() {
	writeDebugLog("ProxmoxBackupClientSVC starting...")
	RunAsService()
}