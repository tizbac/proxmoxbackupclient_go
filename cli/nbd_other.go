//go:build !(linux && amd64)

package main

import (
	"fmt"
	"os"
)

// attachNBD says that attach-nbd needs Linux on x86-64: it uses Linux's nbd
// kernel module, which no other system has, through an ioctl package built
// for x86-64 only.
func attachNBD() {
	fmt.Fprintln(os.Stderr, "salvanza-cli attach-nbd works on Linux (x86-64) only: it attaches the image with Linux's nbd kernel module")
	os.Exit(1)
}
