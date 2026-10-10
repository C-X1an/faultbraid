//go:build windows

package buildinfo

import (
	"os"
	"syscall"
)

func sourceAlias(info os.FileInfo) bool {
	metadata, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return info.Mode()&os.ModeSymlink != 0 || ok && metadata.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
