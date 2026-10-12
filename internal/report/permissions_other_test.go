//go:build !windows

package report

import (
	"os"
	"testing"
)

func assertProtectedOutput(t *testing.T, path string) {
	t.Helper()
	st, e := os.Lstat(path)
	if e != nil {
		t.Fatal(e)
	}
	if outputAlias(st) || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 {
		t.Fatalf("regular report file: type=%v permissions=%#o, want regular 0600", st.Mode().Type(), st.Mode().Perm())
	}
}

func assertProtectedDirectory(t *testing.T, path string) {
	t.Helper()
	st, e := os.Lstat(path)
	if e != nil {
		t.Fatal(e)
	}
	if outputAlias(st) || !st.IsDir() || st.Mode().Perm() != 0700 {
		t.Fatalf("report directory: type=%v permissions=%#o, want directory 0700", st.Mode().Type(), st.Mode().Perm())
	}
}

func makeLeafAlias(t *testing.T, target, alias string) {
	t.Helper()
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
}

func makeParentAlias(t *testing.T, target, alias string) {
	t.Helper()
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
}
