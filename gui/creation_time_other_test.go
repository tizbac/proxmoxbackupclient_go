//go:build !windows

package main

import (
	"testing"
	"time"
)

// creationTimesKept: only Windows has a settable creation time.
const creationTimesKept = false

func setCreated(t *testing.T, path string, when time.Time) {}

func readCreated(t *testing.T, path string) time.Time { return time.Time{} }
