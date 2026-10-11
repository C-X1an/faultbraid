package labreal

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Check every existing path component with Lstat before using an operator's
// local path. This rejects symlinks and Windows reparse aliases, including an
// aliased parent. It cannot defeat a hostile local process changing a checked
// path between check and open; the lab's operator and filesystem are trusted.
func regularPath(path string, finalMissing bool) (string, error) {
	full, err := filepath.Abs(path)
	if err != nil || strings.TrimSpace(path) == "" {
		return "", ErrUnsafePath
	}
	volume := filepath.VolumeName(full)
	current := volume + string(os.PathSeparator)
	parts := strings.Split(strings.TrimPrefix(full[len(volume):], string(os.PathSeparator)), string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if finalMissing && index == len(parts)-1 && os.IsNotExist(err) {
				return full, nil
			}
			return "", ErrUnsafePath
		}
		if pathAlias(info) || index < len(parts)-1 && !info.IsDir() {
			return "", ErrUnsafePath
		}
	}
	return full, nil
}

func fileHash(path string) (string, int64, error) {
	full, err := regularPath(path, false)
	if err != nil {
		return "", 0, err
	}
	file, err := os.Open(full)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
		return "", 0, ErrUnsafePath
	}
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), n, nil
}
