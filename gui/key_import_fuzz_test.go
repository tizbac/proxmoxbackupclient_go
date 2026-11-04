//go:build !service

package main

import "testing"

// Fuzz test for key import: the text comes from a file, a QR code or the
// keyboard, and its key derivation settings come from the text too. Whatever
// it holds, reading it must not panic, and a key it accepts must name its
// fingerprint. As a plain test it runs the seeds; go test -fuzz=FuzzKeyImport
// explores further.
func FuzzKeyImport(f *testing.F) {
	f.Add(`{"kdf":null,"created":"2026-11-04T08:02:00Z","modified":"2026-11-04T08:02:00Z","data":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","fingerprint":"00:11"}`)
	f.Add("-----BEGIN PROXMOX BACKUP KEY-----\n{\"kdf\":{\"Scrypt\":{\"n\":65536,\"r\":8,\"p\":1,\"salt\":\"AAAA\"}},\"data\":\"AA==\"}\n-----END PROXMOX BACKUP KEY-----")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		_, _, _ = parseKeyText(s)
		out, _, fp, err := prepareImportedKey(s, "")
		if err == nil && (out == "" || fp == "") {
			t.Fatalf("accepted a key with no text or no fingerprint: %q", s)
		}
	})
}
