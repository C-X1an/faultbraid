//go:build windows

package labreal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsLabDirectoryAndJournalOnlyGrantCurrentUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "owned")
	if err := createOwnedDirectory(path); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(path, "journal")
	if err := os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, file} {
		descriptor, err := windows.GetNamedSecurityInfo(name, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		dacl, _, err := descriptor.DACL()
		if err != nil || dacl == nil || dacl.AceCount != 1 {
			t.Fatal("lab path retained broad or ambiguous grants")
		}
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, 0, &ace); err != nil {
			t.Fatal(err)
		}
		grantee := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !grantee.Equals(user.User.Sid) {
			t.Fatal("lab file grants another principal")
		}
	}
}

func TestWindowsLabRejectsReparseDirectoryAndJournal(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", alias, target).CombinedOutput(); err != nil {
		t.Fatalf("actual test junction creation failed: %v %s", err, output)
	}
	if _, err := regularPath(filepath.Join(alias, "new-lab"), true); err != ErrUnsafePath {
		t.Fatal("reparse parent accepted")
	}
	if _, err := regularPath(alias, false); err != ErrUnsafePath {
		t.Fatal("reparse journal/directory leaf accepted")
	}
}
