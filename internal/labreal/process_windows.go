//go:build windows

package labreal

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processOwner struct{ job windows.Handle }

// The built-in child is created suspended. No user workload can execute before
// successful assignment to an unnamed kill-on-close job. Every failed setup
// path kills and waits for the process; there is no uncontained fallback.
func startOwned(command *exec.Cmd) (*processOwner, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	owner := &processOwner{job: job}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = owner.Close()
		return nil, err
	}
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_SUSPENDED}
	if err = command.Start(); err != nil {
		_ = owner.Close()
		return nil, err
	}
	fail := func(err error) (*processOwner, error) {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = owner.Close()
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err != nil {
		return fail(err)
	}
	err = windows.AssignProcessToJobObject(job, process)
	_ = windows.CloseHandle(process)
	if err != nil {
		return fail(err)
	}
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(command.Process.Pid) {
			continue
		}
		thread, threadErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if threadErr != nil {
			return fail(threadErr)
		}
		previous, resumeErr := windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		if resumeErr != nil || previous != 1 {
			if resumeErr == nil {
				resumeErr = fmt.Errorf("owned child: unexpected suspend count %d", previous)
			}
			return fail(resumeErr)
		}
		return owner, nil
	}
	return fail(errors.New("owned child: suspended primary thread was not found"))
}
func (o *processOwner) Kill() error { return windows.TerminateJobObject(o.job, 1) }
func (o *processOwner) Close() error {
	if o.job == 0 {
		return nil
	}
	err := windows.CloseHandle(o.job)
	o.job = 0
	return err
}
