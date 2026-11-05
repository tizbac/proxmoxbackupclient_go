//go:build linux && amd64

package main

import "pbsnbd"

// attachNBD is the attach-nbd subcommand (the nbd tool needs Linux's nbd
// kernel module; its ioctl package is built for x86-64 only).
func attachNBD() { pbsnbd.Main() }
