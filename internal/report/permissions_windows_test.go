//go:build windows

package report

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func assertProtectedOutput(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	descriptor, err := windows.GetSecurityInfo(
		windows.Handle(f.Fd()),
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("output DACL inherits parent grants")
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 1 {
		t.Fatalf("output DACL must contain exactly one ACE: count=%d err=%v", aceCount(dacl), err)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERITED_ACE != 0 {
		t.Fatalf("unexpected or inherited ACE: %+v", ace.Header)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	grantee := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !grantee.Equals(user.User.Sid) {
		t.Fatalf("sole allow ACE is not current token user: %s", grantee.String())
	}
	for _, sidText := range []string{"S-1-1-0", "S-1-5-11", "S-1-5-32-545"} {
		sid, err := windows.StringToSid(sidText)
		if err != nil {
			t.Fatal(err)
		}
		if grantee.Equals(sid) {
			t.Fatalf("broad principal granted access: %s", sidText)
		}
	}
}

func aceCount(acl *windows.ACL) uint16 {
	if acl == nil {
		return 0
	}
	return acl.AceCount
}

func makeLeafAlias(t *testing.T, target, alias string) {
	t.Helper()
	// A directory junction is an unprivileged NTFS reparse point. Naming it as
	// the leaf proves exclusive creation will not traverse an existing alias.
	makeJunction(t, filepath.Dir(target), alias)
}

func makeParentAlias(t *testing.T, target, alias string) {
	t.Helper()
	makeJunction(t, target, alias)
}

func makeJunction(t *testing.T, target, alias string) {
	t.Helper()
	if output, err := exec.Command("cmd", "/c", "mklink", "/J", alias, target).CombinedOutput(); err != nil {
		t.Fatalf("create test junction: %v: %s", err, output)
	}
	name, err := windows.UTF16PtrFromString(alias)
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := windows.GetFileAttributes(name)
	if err != nil || attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		t.Fatalf("test alias is not a reparse point: attrs=%#x err=%v", attrs, err)
	}
}
