//go:build !windows

package main

// fileManagerLacksOurRights: off Windows the file manager is started by this
// process and runs as the same user, so it reads what this process reads.
//
// A variable so a test can set it.
var fileManagerLacksOurRights = func() bool { return false }
