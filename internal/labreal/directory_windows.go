//go:build windows

package labreal

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Supply a protected, owner-only inheritable DACL at creation, before writing a
// marker or any journal/output bytes. Children inherit that single-user access.
func createOwnedDirectory(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	var pinner runtime.Pinner
	pinner.Pin(descriptor)
	defer pinner.Unpin()
	attributes := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: descriptor}
	extended := "\\\\?\\" + path
	if strings.HasPrefix(path, "\\\\") {
		extended = "\\\\?\\UNC\\" + strings.TrimPrefix(path, "\\\\")
	}
	name, err := windows.UTF16PtrFromString(extended)
	if err != nil {
		return err
	}
	if err := windows.CreateDirectory(name, &attributes); err != nil {
		return err
	}
	security, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err == nil {
		control, _, controlErr := security.Control()
		if controlErr != nil || control&windows.SE_DACL_PROTECTED == 0 {
			err = errors.New("real lab: protected directory ACL was not retained")
		}
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}
