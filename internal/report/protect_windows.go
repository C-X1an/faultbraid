//go:build windows

package report

import (
	"fmt"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/sys/windows"
)

func openExclusive(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_WRITE|windows.WRITE_DAC,
		0,
		nil,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(handle), path)
	if f == nil {
		_ = windows.CloseHandle(handle)
		return nil, os.ErrInvalid
	}
	return f, nil
}

func protectFile(f *os.File) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	var pinner runtime.Pinner
	pinner.Pin(user.User.Sid)
	defer pinner.Unpin()
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetSecurityInfo(
		windows.Handle(f.Fd()),
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}

func outputAlias(info os.FileInfo) bool {
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return info.Mode()&os.ModeSymlink != 0 || ok && attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func protectDirectory(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.WRITE_DAC|windows.READ_CONTROL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fmt.Errorf("open directory security handle: %w", err)
	}
	f := os.NewFile(uintptr(handle), path)
	if f == nil {
		_ = windows.CloseHandle(handle)
		return os.ErrInvalid
	}
	err = protectFile(f)
	closeErr := f.Close()
	if err != nil {
		return fmt.Errorf("set directory DACL: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close directory security handle: %w", closeErr)
	}
	return nil
}
