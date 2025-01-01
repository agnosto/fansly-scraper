//go:build !windows

package service

import (
	"os/exec"
	"syscall"
)

func getPlatformSpecificCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
