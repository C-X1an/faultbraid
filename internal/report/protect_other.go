//go:build !windows

package report

import "os"

func openExclusive(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}

func protectFile(*os.File) error { return nil }
