//go:build windows

package service

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"syscall"
)

func getPlatformSpecificCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}
}
