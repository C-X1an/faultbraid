package buildinfo

import (
	"io"
	"io/fs"
	"os"
)

type sourceVisitFunc func(path, relative string, ignored bool, entry fs.DirEntry, walkErr error) error

func ignoredSourceComponent(name string) bool {
	switch name {
	case ".git", ".local-runs", "bin", "__pycache__", ".pytest_cache":
		return true
	}
	return false
}

type sourceDirectoryReader interface {
	ReadDir(int) ([]fs.DirEntry, error)
	Close() error
}

// Directory I/O is injectable only inside this package for filesystem error
// controls. Production always opens the actual source directories.
type sourceDirectoryOpener func(string) (sourceDirectoryReader, error)

func walkSourceTree(root string, visit sourceVisitFunc) error {
	return walkSourceTreeWithOpener(root, visit, func(path string) (sourceDirectoryReader, error) {
		return os.Open(path)
	})
}

// The final included-file list is sorted before hashing. Directory traversal
// therefore needs neither lexical sorting nor relative-path parsing per entry.
// Ignored directories are still enumerated completely to reject aliases.
func walkSourceTreeWithOpener(root string, visit sourceVisitFunc, open sourceDirectoryOpener) error {
	info, err := os.Lstat(root)
	if err != nil {
		return visit(root, ".", false, nil, err)
	}
	entry := fs.FileInfoToDirEntry(info)
	if err := visit(root, ".", false, entry, nil); err != nil || !entry.IsDir() {
		return err
	}
	type directory struct {
		path, relative string
		ignored        bool
		entry          fs.DirEntry
	}
	pending := []directory{{path: root, relative: ".", entry: entry}}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		reader, err := open(current.path)
		if err != nil {
			if err := visit(current.path, current.relative, current.ignored, current.entry, err); err != nil {
				return err
			}
			continue
		}
		// Close each directory before opening another, including every error
		// path. Like os.ReadDir used by filepath.WalkDir, Close errors do not
		// replace the directory enumeration result.
		err = func() error {
			defer reader.Close()
			prefix := current.path
			if !os.IsPathSeparator(prefix[len(prefix)-1]) {
				prefix += string(os.PathSeparator)
			}
			for {
				entries, readErr := reader.ReadDir(256)
				if len(entries) == 0 && readErr == nil {
					readErr = io.ErrNoProgress
				}
				if readErr != nil && readErr != io.EOF {
					if err := visit(current.path, current.relative, current.ignored, current.entry, readErr); err != nil {
						return err
					}
				}
				for _, child := range entries {
					name := child.Name()
					path := prefix + name
					relative := name
					if current.relative != "." {
						relative = current.relative + "/" + relative
					}
					ignored := current.ignored || ignoredSourceComponent(name)
					if err := visit(path, relative, ignored, child, nil); err != nil {
						return err
					}
					if child.IsDir() {
						pending = append(pending, directory{path, relative, ignored, child})
					}
				}
				if readErr != nil {
					return nil
				}
			}
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
