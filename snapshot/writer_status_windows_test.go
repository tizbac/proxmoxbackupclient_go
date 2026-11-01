//go:build windows

package snapshot

import (
	"strings"
	"testing"
)

// vssadmin list writers as Windows Server 2025 prints it with every writer
// healthy: each says "Last error: No error", which the old check took for an
// error of the System Writer.
const writersHealthy = "vssadmin 1.1 - Volume Shadow Copy Service administrative command-line tool\r\n" +
	"(C) Copyright 2001-2013 Microsoft Corp.\r\n\r\n" +
	"Writer name: 'Task Scheduler Writer'\r\n   Writer Id: {d61d61c8-d73a-4eee-8cdd-f6f9786b7124}\r\n   State: [1] Stable\r\n   Last error: No error\r\n\r\n" +
	"Writer name: 'System Writer'\r\n   Writer Id: {e8132975-6f93-4464-a53e-1050253ae220}\r\n   State: [1] Stable\r\n   Last error: No error\r\n\r\n" +
	"Writer name: 'NTDS'\r\n   Writer Id: {b2014c9e-8711-4c5c-a5a9-3cf384484757}\r\n   State: [1] Stable\r\n   Last error: No error\r\n"

func TestWriterProblemsHealthy(t *testing.T) {
	if p := writerProblems(writersHealthy); len(p) != 0 {
		t.Fatalf("healthy writers reported as %v", p)
	}
}

func TestWriterProblemsNamesTheWriter(t *testing.T) {
	out := strings.Replace(writersHealthy, "'System Writer'\r\n   Writer Id: {e8132975-6f93-4464-a53e-1050253ae220}\r\n   State: [1] Stable\r\n   Last error: No error",
		"'System Writer'\r\n   Writer Id: {e8132975-6f93-4464-a53e-1050253ae220}\r\n   State: [8] Failed\r\n   Last error: Retryable error", 1)
	out = strings.Replace(out, "'NTDS'\r\n   Writer Id: {b2014c9e-8711-4c5c-a5a9-3cf384484757}\r\n   State: [1] Stable\r\n   Last error: No error",
		"'NTDS'\r\n   Writer Id: {b2014c9e-8711-4c5c-a5a9-3cf384484757}\r\n   State: [1] Stable\r\n   Last error: Non-retryable error", 1)
	p := writerProblems(out)
	if len(p) != 2 || !strings.Contains(p[0], "'System Writer'") || !strings.Contains(p[0], "Retryable error") || !strings.Contains(p[1], "'NTDS'") {
		t.Fatalf("problems = %q", p)
	}
}

func TestWriterProblemsOtherLanguage(t *testing.T) {
	// Output in another language has none of the English labels: no warning
	// rather than a wrong one.
	if p := writerProblems("Nom de l'enregistreur : 'System Writer'\r\n   État : [1] Stable\r\n   Dernière erreur : Aucune erreur\r\n"); len(p) != 0 {
		t.Fatalf("problems = %q", p)
	}
}
