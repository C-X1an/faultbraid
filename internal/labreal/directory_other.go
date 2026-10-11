//go:build !windows

package labreal

import "os"

func createOwnedDirectory(path string) error { return os.Mkdir(path, 0700) }
