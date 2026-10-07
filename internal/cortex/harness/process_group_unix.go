//go:build !windows

package harness

import (
	"os/exec"
	"syscall"
)

func processGroupAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// A negative PID addresses the process group created by processGroupAttr.
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
