package buildinfo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf16"
)

// FromFiles identifies a trusted local source directory. Production binaries
// use Current's build-injected identity; tests can identify their actual tree.
func FromFiles(root string) (Identity, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Identity{}, err
	}
	type sourceFile struct {
		Path string `json:"path"`
		SHA  string `json:"sha256"`
	}
	files := []sourceFile{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if sourceAlias(info) {
			return fmt.Errorf("source identity: symbolic link or reparse point")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
			switch part {
			case ".git", ".local-runs", "bin", "__pycache__", ".pytest_cache":
				return nil // Walk aliases in ignored paths too, but never hash their data.
			}
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		files = append(files, sourceFile{filepath.ToSlash(rel), hex.EncodeToString(hash[:])})
		return nil
	})
	if err != nil {
		return Identity{}, err
	}
	sort.Slice(files, func(i, j int) bool {
		left, right := files[i].Path, files[j].Path
		if runtime.GOOS == "windows" {
			left, right = strings.ToLower(left), strings.ToLower(right)
		}
		return left < right
	})
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(files); err != nil {
		return Identity{}, err
	}
	// Match the evidence runner's compact Python JSON ensure_ascii encoding.
	var canonical bytes.Buffer
	for _, r := range strings.TrimSuffix(encoded.String(), "\n") {
		if r < 128 {
			canonical.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&canonical, "\\u%04x", r)
		} else {
			first, second := utf16.EncodeRune(r)
			fmt.Fprintf(&canonical, "\\u%04x\\u%04x", first, second)
		}
	}
	source := sha256.Sum256(canonical.Bytes())
	module := sha256.New()
	for i, name := range []string{"go.mod", "go.sum", "dependency.lock.json"} {
		if i > 0 {
			module.Write([]byte{0})
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return Identity{}, err
		}
		module.Write(data)
	}
	patch, err := os.ReadFile(filepath.Join(root, "third_party", "raft-patch", "election-random.patch"))
	if err != nil {
		return Identity{}, err
	}
	patchHash := sha256.Sum256(patch)
	return Identity{hex.EncodeToString(source[:]), hex.EncodeToString(module.Sum(nil)), hex.EncodeToString(patchHash[:])}, nil
}
