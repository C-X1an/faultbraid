//go:build !windows

package labreal

import "os"

func pathAlias(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
