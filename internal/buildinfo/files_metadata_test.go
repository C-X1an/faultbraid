package buildinfo

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var ignoredMetadataComponents = []string{".git", ".local-runs", "bin", "__pycache__", ".pytest_cache"}

// These controlled callback fixtures exercise the production identity algorithm
// over real source files. They are filesystem error regressions, not target or
// distributed integration tests and not evidence of an old hosted failure's cause.
func metadataSourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum", "dependency.lock.json", "third_party/raft-patch/election-random.patch", "source.go"} {
		writeMetadataFixture(t, root, name, "regular controlled source fixture\n")
	}
	return root
}

func writeMetadataFixture(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

type metadataErrorEntry struct {
	fs.DirEntry
	err error
}

func (entry metadataErrorEntry) Info() (fs.FileInfo, error) { return nil, entry.err }

// Real traversal and hashing run first; the injected callback models a path
// whose directory entry was observed before its metadata disappeared or failed.
func metadataErrorWalker(relative, boundary string, cause error) func(string, fs.WalkDirFunc) error {
	return func(root string, callback fs.WalkDirFunc) error {
		if err := filepath.WalkDir(root, callback); err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(filepath.Join(root, "source.go"))
		if err != nil {
			return err
		}
		entry := fs.FileInfoToDirEntry(info)
		metadataErr := &fs.PathError{Op: boundary, Path: path, Err: cause}
		if boundary == "info" {
			return callback(path, metadataErrorEntry{entry, metadataErr}, nil)
		}
		if boundary == "walk-nil-entry" {
			entry = nil
		}
		return callback(path, entry, metadataErr)
	}
}

func TestFromFilesMissingIgnoredMetadata(t *testing.T) {
	root := metadataSourceFixture(t)
	want, err := FromFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	paths := append([]string{}, ignoredMetadataComponents...)
	for _, component := range ignoredMetadataComponents {
		paths = append(paths, component+"/transient", "nested/"+component+"/transient")
	}
	for _, path := range paths {
		for _, boundary := range []string{"walk", "walk-nil-entry", "info"} {
			t.Run(path+"/"+boundary, func(t *testing.T) {
				// Wrapped ErrNotExist must work at both callback boundaries.
				got, err := fromFilesWithWalker(root, metadataErrorWalker(path, boundary, errors.Join(errors.New("controlled disappearance"), fs.ErrNotExist)))
				if err != nil || got != want {
					t.Fatalf("ignored disappearing metadata changed identity: got=%+v want=%+v err=%v", got, want, err)
				}
			})
		}
	}
}

func TestFromFilesDirectoryRemovedDuringRealWalk(t *testing.T) {
	paths := append([]string{}, ignoredMetadataComponents...)
	paths = append(paths, ".gitx", ".github", "source-dir")
	for index, component := range paths {
		t.Run(component, func(t *testing.T) {
			root := metadataSourceFixture(t)
			path := filepath.Join(root, component, "transient")
			if err := os.MkdirAll(path, 0700); err != nil {
				t.Fatal(err)
			}
			want, err := FromFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			removed := false
			missingWalkCallbacks := 0
			walker := func(root string, callback fs.WalkDirFunc) error {
				return filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
					if current == path {
						if walkErr == nil && !removed {
							// Remove only this owned empty fixture directory after
							// WalkDir has obtained its entry but before it reads it.
							if err := os.Remove(current); err != nil {
								return err
							}
							removed = true
						}
						if errors.Is(walkErr, fs.ErrNotExist) {
							missingWalkCallbacks++
						}
					}
					return callback(current, entry, walkErr)
				})
			}
			got, err := fromFilesWithWalker(root, walker)
			if !removed {
				t.Fatal("real traversal did not remove the fixture directory")
			}
			if index < len(ignoredMetadataComponents) {
				if missingWalkCallbacks == 0 || err != nil || got != want {
					t.Fatalf("ignored real disappearance: missing callbacks=%d got=%+v want=%+v err=%v", missingWalkCallbacks, got, want, err)
				}
			} else if !errors.Is(err, fs.ErrNotExist) || got != (Identity{}) {
				t.Fatalf("included real disappearance accepted: got=%+v err=%v", got, err)
			}
		})
	}
}

func TestFromFilesRejectsMissingIncludedMetadata(t *testing.T) {
	root := metadataSourceFixture(t)
	paths := []string{"source.go", "nested/source.go", ".github/transient", ".gitx/transient", ".local-run/transient", "binary/transient", "__pycache__x/transient", ".pytest_cachex/transient"}
	for _, path := range paths {
		for _, boundary := range []string{"walk", "walk-nil-entry", "info"} {
			t.Run(path+"/"+boundary, func(t *testing.T) {
				identity, err := fromFilesWithWalker(root, metadataErrorWalker(path, boundary, fs.ErrNotExist))
				if !errors.Is(err, fs.ErrNotExist) || identity != (Identity{}) {
					t.Fatalf("included disappearance accepted: identity=%+v err=%v", identity, err)
				}
			})
		}
	}
}

func TestFromFilesRejectsOtherMetadataErrors(t *testing.T) {
	root := metadataSourceFixture(t)
	sharingViolation := errors.New("controlled sharing violation")
	for _, cause := range []error{fs.ErrPermission, sharingViolation} {
		for _, component := range ignoredMetadataComponents {
			for _, boundary := range []string{"walk", "walk-nil-entry", "info"} {
				t.Run(cause.Error()+"/"+component+"/"+boundary, func(t *testing.T) {
					identity, err := fromFilesWithWalker(root, metadataErrorWalker(component+"/transient", boundary, cause))
					if !errors.Is(err, cause) || identity != (Identity{}) {
						t.Fatalf("non-disappearance error accepted: identity=%+v err=%v", identity, err)
					}
				})
			}
		}
	}
}

func TestFromFilesIgnoresOnlyExactPathComponents(t *testing.T) {
	root := metadataSourceFixture(t)
	want, err := FromFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, component := range ignoredMetadataComponents {
		writeMetadataFixture(t, root, component+"/metadata", "ignored metadata\n")
		writeMetadataFixture(t, root, "nested/"+component+"/metadata", "ignored nested metadata\n")
	}
	got, err := FromFiles(root)
	if err != nil || got != want {
		t.Fatalf("ignored bytes changed identity: got=%+v want=%+v err=%v", got, want, err)
	}
	for _, component := range []string{".github", ".gitx", ".local-run", "binary", "__pycache__x", ".pytest_cachex"} {
		before, err := FromFiles(root)
		if err != nil {
			t.Fatal(err)
		}
		writeMetadataFixture(t, root, component+"/source", "included source\n")
		after, err := FromFiles(root)
		if err != nil || after.SourceTreeSHA == before.SourceTreeSHA || after.ModuleLockSHA != before.ModuleLockSHA || after.PatchSHA != before.PatchSHA {
			t.Fatalf("component %q was not hashed correctly: before=%+v after=%+v err=%v", component, before, after, err)
		}
	}
}

type metadataAliasInfo struct{ fs.FileInfo }

func (info metadataAliasInfo) Mode() fs.FileMode { return info.FileInfo.Mode() | fs.ModeSymlink }

func TestFromFilesRejectsAliasesInsideIgnoredMetadata(t *testing.T) {
	root := metadataSourceFixture(t)
	for _, component := range ignoredMetadataComponents {
		t.Run(component, func(t *testing.T) {
			aliasPath := filepath.Join(root, component, "nested", "alias")
			writeMetadataFixture(t, root, component+"/nested/alias", "controlled alias entry\n")
			// Replace one real walked entry's metadata with a controlled alias
			// flag. Skipping the ignored directory would miss this entry and
			// make the expected rejection fail.
			walker := func(root string, callback fs.WalkDirFunc) error {
				return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
					if path == aliasPath && walkErr == nil {
						info, err := entry.Info()
						if err != nil {
							return err
						}
						entry = fs.FileInfoToDirEntry(metadataAliasInfo{info})
					}
					return callback(path, entry, walkErr)
				})
			}
			identity, err := fromFilesWithWalker(root, walker)
			if err == nil || !strings.Contains(err.Error(), "symbolic link or reparse point") || identity != (Identity{}) {
				t.Fatalf("alias below ignored directory accepted: identity=%+v err=%v", identity, err)
			}
		})
	}
}
