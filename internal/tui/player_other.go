//go:build !unix

package tui

import "os"

// pauseProcess is a no-op on non-Unix platforms (SIGSTOP is not supported).
func pauseProcess(_ *os.Process) error { return nil }

// resumeProcess is a no-op on non-Unix platforms (SIGCONT is not supported).
func resumeProcess(_ *os.Process) error { return nil }
