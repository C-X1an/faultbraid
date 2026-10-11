//go:build !windows

package report

import "os"

func openExclusive(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}

func protectFile(*os.File) error { return nil }

func protectDirectory(string) error     { return nil }
func outputAlias(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
