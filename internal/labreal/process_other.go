//go:build !windows

package labreal

import (
	"errors"
	"os/exec"
	"syscall"
)

type processOwner struct{ pid int }

func startOwned(command *exec.Cmd) (*processOwner, error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &processOwner{pid: command.Process.Pid}, nil
}
func (o *processOwner) Kill() error {
	err := syscall.Kill(-o.pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
func (o *processOwner) Close() error { return nil }
