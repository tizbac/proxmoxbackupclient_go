package main

import "bytes"

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// stripUTF8BOM strips a leading UTF-8 byte-order-mark, if present, from JSON
// file contents before unmarshaling. encoding/json does not skip it, so a
// file written by a BOM-emitting tool, e.g. PowerShell's `ConvertTo-Json |
// Out-File`/`Set-Content`, which defaults to UTF-8 with BOM, fails with a
// cryptic "invalid character (BOM codepoint) looking for beginning of value".
// Found live 2026-09-24: this silently locked scheduled_jobs.json's own
// SaveScheduledJob out of every existing job (a safety check refuses to
// overwrite on a load failure) after a manual PowerShell edit, and would
// have silently reset message_log.json to just one new entry, discarding
// history, since LogMessage treats a parse failure as "start a fresh log."
func stripUTF8BOM(data []byte) []byte {
	return bytes.TrimPrefix(data, utf8BOM)
}
