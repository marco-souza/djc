//go:build unix

package tui

import (
	"os"
	"syscall"
)

// pauseProcess suspends a running process (Unix: SIGSTOP).
func pauseProcess(p *os.Process) error { return p.Signal(syscall.SIGSTOP) }

// resumeProcess resumes a suspended process (Unix: SIGCONT).
func resumeProcess(p *os.Process) error { return p.Signal(syscall.SIGCONT) }
