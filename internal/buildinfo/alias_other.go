//go:build !windows

package buildinfo

import "os"

func sourceAlias(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
