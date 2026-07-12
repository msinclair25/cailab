//go:build !windows

package provider

import (
	"os/exec"
	"syscall"
)

func configureDetachedCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
