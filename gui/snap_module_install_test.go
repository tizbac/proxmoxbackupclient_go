//go:build !service

package main

import "testing"

func TestCheckMokPassword(t *testing.T) {
	for pw, ok := range map[string]bool{
		"abcd1234":          true,
		"ABCDEFGH12345678":  true,
		"abc1234":           false, // 7
		"abcdefgh123456789": false, // 17
		"abcd 1234":         false,
		"abcd-1234":         false,
		"äbcd1234":          false,
		"":                  false,
	} {
		if err := checkMokPassword(pw); (err == nil) != ok {
			t.Errorf("checkMokPassword(%q) = %v, want ok=%v", pw, err, ok)
		}
	}
}
