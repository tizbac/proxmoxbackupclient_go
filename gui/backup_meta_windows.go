//go:build windows
// +build windows

package main

import "pbscommon"

// NTFSMetaCollector is pbscommon's Windows collector (ACLs, attributes,
// creation times), shared with the directory CLI.
type NTFSMetaCollector = pbscommon.NTFSMetaCollector

func NewNTFSMetaCollector(root, hostname string) *NTFSMetaCollector {
	return pbscommon.NewNTFSMetaCollector(root, hostname)
}
