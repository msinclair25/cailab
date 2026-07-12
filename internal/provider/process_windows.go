//go:build windows

package provider

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureDetachedCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
		HideWindow:    true,
	}
}
