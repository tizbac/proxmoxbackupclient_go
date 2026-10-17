//go:build windows

package main

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const canSealSecrets = true

// secretEntropy is DPAPI's optional extra secret: not a key, but it keeps
// another program's blob from being unsealed as one of ours by mistake.
var secretEntropy = []byte("ProxmoxBackupClient config.json v1")

func dataBlob(b []byte) *windows.DataBlob {
	if len(b) == 0 {
		return &windows.DataBlob{}
	}
	return &windows.DataBlob{Size: uint32(len(b)), Data: &b[0]}
}

// takeBlob copies a DPAPI output blob and frees it.
func takeBlob(b *windows.DataBlob) []byte {
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(b.Data)))
	out := make([]byte, b.Size)
	copy(out, unsafe.Slice(b.Data, b.Size))
	return out
}

func sealPlatform(s string) (string, error) {
	var out windows.DataBlob
	err := windows.CryptProtectData(dataBlob([]byte(s)), nil, dataBlob(secretEntropy), 0, nil,
		windows.CRYPTPROTECT_LOCAL_MACHINE|windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return "", fmt.Errorf("CryptProtectData: %w", err)
	}
	return sealedPrefix + base64.StdEncoding.EncodeToString(takeBlob(&out)), nil
}

func openPlatform(s string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(s[len(sealedPrefix):])
	if err != nil {
		return "", fmt.Errorf("not valid base64: %w", err)
	}
	var out windows.DataBlob
	err = windows.CryptUnprotectData(dataBlob(raw), nil, dataBlob(secretEntropy), 0, nil,
		windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	if err != nil {
		return "", fmt.Errorf("CryptUnprotectData: %w", err)
	}
	return string(takeBlob(&out)), nil
}
