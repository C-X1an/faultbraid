//go:build !windows

package report

import (
	"os"
	"testing"
)

func assertProtectedOutput(t *testing.T, path string) {
	t.Helper()
	st, e := os.Stat(path)
	if e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("output permissions")
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
