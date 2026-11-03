package clientcommon

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"pbscommon"

	"golang.org/x/term"
)

// ConfirmFingerprint returns the certificate fingerprint to pin for a PBS
// connection. When configured is non-empty it is returned unchanged (no
// prompt). Otherwise the fingerprint the server at baseURL actually presents
// is fetched and shown to the user, who must explicitly confirm it before it
// is used; a refusal, EOF or any read failure aborts with an error so a
// non-interactive or silent run can never pin a certificate blindly.
func ConfirmFingerprint(baseURL, configured string) (string, error) {
	if configured != "" {
		return configured, nil
	}
	fp, err := pbscommon.FetchServerFingerprint(baseURL)
	if err != nil {
		return "", fmt.Errorf("fetching server certificate fingerprint: %w", err)
	}
	fmt.Printf("No certificate fingerprint was specified (-certfingerprint).\n")
	fmt.Printf("Server %s presents the following certificate fingerprint:\n    %s\n", baseURL, fp)
	fmt.Print("Trust and pin this fingerprint for this run? [y/N] ")
	answer, err := readStdinLine()
	if err != nil {
		return "", fmt.Errorf("cannot read confirmation from console: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return fp, nil
	}
	return "", fmt.Errorf("fingerprint not confirmed, aborting; pass -certfingerprint <fp> to skip the interactive check")
}

// PromptPassword asks the user for a secret on the console. When stdin is a
// terminal the input is read with echo disabled; otherwise (scripts, pipes) a
// plain line is read so automation keeps working.
func PromptPassword(label string) (string, error) {
	_, _ = fmt.Fprint(os.Stdout, label)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return readStdinLine()
}

func readStdinLine() (string, error) {
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
