//go:build windows

package harness

import (
	"os/exec"
	"strconv"
	"syscall"
)

const (
	// CREATE_NEW_PROCESS_GROUP flag for Windows
	createProcessGroup = 0x00000200
)

func processGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: createProcessGroup,
	}
}

func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Try taskkill /T /F /PID to recursively terminate child processes.
	// Fall back to Process.Kill().
	pid := cmd.Process.Pid
	if pid > 0 {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	_ = cmd.Process.Kill()
}
