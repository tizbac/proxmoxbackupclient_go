//go:build windows

package api

// The token file's ACL is set by the gui package; these tests only need a
// token file, so they use none.
func init() {
	SecureTokenFile = func(string) error { return nil }
}
