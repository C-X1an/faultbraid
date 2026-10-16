package buildinfo

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These are source-filesystem controls. The directory faults and alias metadata
// below are synthetic; they do not simulate Raft nodes or provide target credit.
func TestSourceTraversalMatchesWalkDirCensus(t *testing.T) {
	root := metadataSourceFixture(t)
	for _, component := range ignoredMetadataComponents {
		for i := range 520 {
			writeMetadataFixture(t, root, fmt.Sprintf("nested/%s/deep/d%02d/file-%04d", component, i%7, i), "ignored bytes\n")
		}
	}
	for _, name := range []string{".github/source", ".gitx/source", ".local-run/source", "binary/source", "__pycache__x/source", ".pytest_cachex/source", "A.dat", "a0.dat", "i0.dat", "\u0130.dat", "delete-\u007f.dat", "\U0001f9ea.dat", "deep/a/b/c/d/e/f/g/h/i/j/source"} {
		writeMetadataFixture(t, root, name, "included bytes: "+name+"\n")
	}
	want, err := fromFilesWithWalker(root, filepath.WalkDir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FromFiles(root)
	if err != nil || got != want {
		t.Fatalf("streaming identity differs from WalkDir: got=%+v want=%+v err=%v", got, want, err)
	}
	wantCensus := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		ignored := false
		for _, component := range strings.Split(relative, "/") {
			ignored = ignored || ignoredSourceComponent(component)
		}
		wantCensus[relative] = ignored
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	gotCensus := map[string]bool{}
	err = walkSourceTree(root, func(path, relative string, ignored bool, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if _, duplicate := gotCensus[relative]; duplicate {
			return fmt.Errorf("duplicate visit: %s", relative)
		}
		if path != filepath.Join(root, filepath.FromSlash(relative)) {
			return fmt.Errorf("incorrect native path for %s", relative)
		}
		gotCensus[relative] = ignored
		return nil
	})
	if err != nil || !reflect.DeepEqual(gotCensus, wantCensus) {
		t.Fatalf("entry census differs: got=%d want=%d err=%v", len(gotCensus), len(wantCensus), err)
	}
}

type observedSourceReader struct {
	*os.File
	path     string
	reads    int
	closed   bool
	limit    int
	decorate func(string, []fs.DirEntry) []fs.DirEntry
	fault    error
}

func (reader *observedSourceReader) ReadDir(n int) ([]fs.DirEntry, error) {
	reader.reads++
	if reader.limit > 0 && reader.limit < n {
		n = reader.limit
	}
	entries, err := reader.File.ReadDir(n)
	if reader.decorate != nil {
		entries = reader.decorate(reader.path, entries)
	}
	if reader.reads == 1 && reader.fault != nil {
		return entries, &fs.PathError{Op: "readdir", Path: reader.path, Err: reader.fault}
	}
	return entries, err
}

func (reader *observedSourceReader) Close() error {
	reader.closed = true
	return reader.File.Close()
}

func observedSourceOpener(t *testing.T, modify func(string, *observedSourceReader) error) (sourceDirectoryOpener, *[]*observedSourceReader) {
	t.Helper()
	readers := []*observedSourceReader{}
	return func(path string) (sourceDirectoryReader, error) {
		for _, previous := range readers {
			if !previous.closed {
				t.Errorf("directory %q remains open when opening %q", previous.path, path)
			}
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		reader := &observedSourceReader{File: file, path: path}
		readers = append(readers, reader)
		if modify != nil {
			if err := modify(path, reader); err != nil {
				reader.Close()
				return nil, err
			}
		}
		return reader, nil
	}, &readers
}

func assertSourceReadersClosed(t *testing.T, readers []*observedSourceReader) {
	t.Helper()
	for _, reader := range readers {
		if !reader.closed {
			t.Errorf("directory handle was not closed: %s", reader.path)
		}
	}
}

func sourceIdentityWithOpener(root string, open sourceDirectoryOpener) (Identity, error) {
	return fromFilesWithTraversal(root, func(root string, visit sourceVisitFunc) error {
		return walkSourceTreeWithOpener(root, visit, open)
	})
}

func TestSourceTraversalReadDirBatchesAndClose(t *testing.T) {
	root := metadataSourceFixture(t)
	for i := range 600 {
		writeMetadataFixture(t, root, fmt.Sprintf(".local-runs/file-%04d", i), "ignored bytes\n")
	}
	want, err := fromFilesWithWalker(root, filepath.WalkDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{1, 256} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			open, readers := observedSourceOpener(t, func(path string, reader *observedSourceReader) error {
				reader.limit = limit
				return nil
			})
			got, err := sourceIdentityWithOpener(root, open)
			if err != nil || got != want {
				t.Fatalf("batched traversal differs: got=%+v want=%+v err=%v", got, want, err)
			}
			assertSourceReadersClosed(t, *readers)
			for _, reader := range *readers {
				if reader.path == filepath.Join(root, ".local-runs") && reader.reads < 3 {
					t.Fatalf("large directory was not traversed in multiple reads: %d", reader.reads)
				}
			}
		})
	}
}

func TestSourceTraversalDirectoryErrors(t *testing.T) {
	other := errors.New("controlled directory sharing error")
	wrappedEOF := fmt.Errorf("controlled nonterminal error: %w", io.EOF)
	for _, relative := range []string{"source-dir", ".local-runs/nested"} {
		for _, boundary := range []string{"open", "read"} {
			for _, cause := range []error{fs.ErrNotExist, fs.ErrPermission, other, wrappedEOF} {
				t.Run(relative+"/"+boundary+"/"+cause.Error(), func(t *testing.T) {
					root := metadataSourceFixture(t)
					writeMetadataFixture(t, root, relative+"/entry", "controlled partial directory bytes\n")
					want, err := FromFiles(root)
					if err != nil {
						t.Fatal(err)
					}
					target := filepath.Join(root, filepath.FromSlash(relative))
					open, readers := observedSourceOpener(t, func(path string, reader *observedSourceReader) error {
						if path == target {
							if boundary == "open" {
								return &fs.PathError{Op: "open", Path: path, Err: cause}
							}
							reader.fault = cause
						}
						return nil
					})
					got, err := sourceIdentityWithOpener(root, open)
					ignoredMissing := strings.HasPrefix(relative, ".local-runs/") && errors.Is(cause, fs.ErrNotExist)
					if ignoredMissing {
						if err != nil || got != want {
							t.Fatalf("ignored missing directory rejected: got=%+v want=%+v err=%v", got, want, err)
						}
					} else if !errors.Is(err, cause) || got != (Identity{}) {
						t.Fatalf("directory error accepted: got=%+v err=%v", got, err)
					}
					assertSourceReadersClosed(t, *readers)
				})
			}
		}
	}
}

func TestSourceTraversalAliasInIgnoredPartialRead(t *testing.T) {
	for _, component := range ignoredMetadataComponents {
		t.Run(component, func(t *testing.T) {
			root := metadataSourceFixture(t)
			writeMetadataFixture(t, root, component+"/nested/alias", "controlled alias metadata\n")
			target := filepath.Join(root, component, "nested")
			open, readers := observedSourceOpener(t, func(path string, reader *observedSourceReader) error {
				if path == target {
					reader.fault = fs.ErrNotExist
					reader.decorate = func(path string, entries []fs.DirEntry) []fs.DirEntry {
						for index, entry := range entries {
							info, err := entry.Info()
							if err != nil {
								t.Fatal(err)
							}
							entries[index] = fs.FileInfoToDirEntry(metadataAliasInfo{info})
						}
						return entries
					}
				}
				return nil
			})
			got, err := sourceIdentityWithOpener(root, open)
			if err == nil || !strings.Contains(err.Error(), "symbolic link or reparse point") || got != (Identity{}) {
				t.Fatalf("partial ignored read concealed an alias: got=%+v err=%v", got, err)
			}
			assertSourceReadersClosed(t, *readers)
		})
	}
}

func TestSourceTraversalRejectsIncludedAlias(t *testing.T) {
	for _, relative := range []string{"alias", "included/nested/alias"} {
		t.Run(relative, func(t *testing.T) {
			root := metadataSourceFixture(t)
			writeMetadataFixture(t, root, relative, "controlled included alias metadata\n")
			target := filepath.Join(root, filepath.FromSlash(relative))
			open, readers := observedSourceOpener(t, func(path string, reader *observedSourceReader) error {
				reader.decorate = func(path string, entries []fs.DirEntry) []fs.DirEntry {
					for index, entry := range entries {
						if filepath.Join(path, entry.Name()) == target {
							info, err := entry.Info()
							if err != nil {
								t.Fatal(err)
							}
							entries[index] = fs.FileInfoToDirEntry(metadataAliasInfo{info})
						}
					}
					return entries
				}
				return nil
			})
			got, err := sourceIdentityWithOpener(root, open)
			if err == nil || !strings.Contains(err.Error(), "symbolic link or reparse point") || got != (Identity{}) {
				t.Fatalf("included alias accepted: got=%+v err=%v", got, err)
			}
			assertSourceReadersClosed(t, *readers)
		})
	}
}

func TestSourceTraversalMetadataErrors(t *testing.T) {
	other := errors.New("controlled entry sharing error")
	for _, component := range append([]string{"source-dir"}, ignoredMetadataComponents...) {
		for _, cause := range []error{fs.ErrNotExist, fs.ErrPermission, other} {
			t.Run(component+"/"+cause.Error(), func(t *testing.T) {
				root := metadataSourceFixture(t)
				writeMetadataFixture(t, root, component+"/nested/transient", "controlled metadata error\n")
				want, err := FromFiles(root)
				if err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, component, "nested", "transient")
				open, readers := observedSourceOpener(t, func(path string, reader *observedSourceReader) error {
					reader.decorate = func(path string, entries []fs.DirEntry) []fs.DirEntry {
						for index, entry := range entries {
							if filepath.Join(path, entry.Name()) == target {
								entries[index] = metadataErrorEntry{entry, &fs.PathError{Op: "info", Path: target, Err: cause}}
							}
						}
						return entries
					}
					return nil
				})
				got, err := sourceIdentityWithOpener(root, open)
				if component != "source-dir" && errors.Is(cause, fs.ErrNotExist) {
					if err != nil || got != want {
						t.Fatalf("ignored missing entry changed identity: got=%+v want=%+v err=%v", got, want, err)
					}
				} else if !errors.Is(err, cause) || got != (Identity{}) {
					t.Fatalf("entry metadata failure accepted: got=%+v err=%v", got, err)
				}
				assertSourceReadersClosed(t, *readers)
			})
		}
	}
}

func TestSourceTraversalDirectoryRemovedBeforeOpen(t *testing.T) {
	for _, relative := range []string{"source-dir", ".git/transient", ".local-runs/transient", "bin/transient", "__pycache__/transient", ".pytest_cache/transient"} {
		t.Run(relative, func(t *testing.T) {
			root := metadataSourceFixture(t)
			target := filepath.Join(root, filepath.FromSlash(relative))
			if err := os.MkdirAll(target, 0700); err != nil {
				t.Fatal(err)
			}
			want, err := FromFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			removed := false
			open := func(path string) (sourceDirectoryReader, error) {
				if path == target {
					if err := os.Remove(path); err != nil {
						return nil, err
					}
					removed = true
				}
				return os.Open(path)
			}
			got, err := sourceIdentityWithOpener(root, open)
			if !removed {
				t.Fatal("real directory removal did not run")
			}
			if relative == "source-dir" {
				if !errors.Is(err, fs.ErrNotExist) || got != (Identity{}) {
					t.Fatalf("included removed directory accepted: got=%+v err=%v", got, err)
				}
			} else if err != nil || got != want {
				t.Fatalf("ignored removed directory changed identity: got=%+v want=%+v err=%v", got, want, err)
			}
		})
	}
}

func TestSourceTraversalMissingRoot(t *testing.T) {
	got, err := FromFiles(filepath.Join(t.TempDir(), "missing-root"))
	if !errors.Is(err, fs.ErrNotExist) || got != (Identity{}) {
		t.Fatalf("missing root accepted: got=%+v err=%v", got, err)
	}
}
