//go:build windows && service

package main

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// removeOldServices stops and removes the background services of earlier
// installs: the names before the rename (ProxmoxBackupClientSVC,
// ProxmoxBackupClient, NimbusBackupSVC, ...) and the other brands' services,
// which an install of this brand replaces for now. An MSI upgrade removes its
// own predecessor; this catches the copies installed without one (Run as
// Service from a portable folder). Best effort: anything it cannot remove is
// logged and left, and this service runs on regardless.
func removeOldServices() {
	m, err := mgr.Connect()
	if err != nil {
		writeDebugLog(fmt.Sprintf("Earlier services not checked: %v", err))
		return
	}
	defer func() { _ = m.Disconnect() }()
	names := append(append([]string{}, legacyServiceNames...), otherBrandServiceNames()...)
	for _, n := range names {
		if strings.EqualFold(n, serviceName()) {
			continue
		}
		s, err := m.OpenService(n)
		if err != nil {
			continue
		}
		if st, err := s.Query(); err == nil && st.State != svc.Stopped {
			_, _ = s.Control(svc.Stop)
			for i := 0; i < 60; i++ {
				if st, err := s.Query(); err != nil || st.State == svc.Stopped {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
		if err := s.Delete(); err != nil {
			writeDebugLog(fmt.Sprintf("Earlier service %s not removed: %v", n, err))
		} else {
			writeDebugLog(fmt.Sprintf("Removed the earlier service %s", n))
		}
		_ = s.Close()
	}
}
