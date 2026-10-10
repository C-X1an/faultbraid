package buildinfo

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestActualTreeMatchesEvidenceIdentity(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	i, err := FromFiles(root)
	if err != nil || !i.Identified() {
		t.Fatalf("actual tree identity: %+v %v", i, err)
	}
	command := exec.Command("python3", "-c", "import json,sys;sys.path.insert(0,'scripts');from pathlib import Path;from run_evidence import source_identity;print(json.dumps(source_identity(Path.cwd())))")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var identity struct {
		SHA string `json:"source_tree_sha256"`
	}
	if err := json.Unmarshal(output, &identity); err != nil {
		t.Fatal(err)
	}
	if identity.SHA != i.SourceTreeSHA {
		t.Fatalf("Go/evidence identity mismatch: %s != %s", i.SourceTreeSHA, identity.SHA)
	}
	if (Identity{}).Identified() || (Identity{SourceTreeSHA: "unrecorded"}).Identified() {
		t.Fatal("unidentified source accepted")
	}
}

func TestEvidenceIdentityBoundaryFilenames(t *testing.T) {
	public, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, names := range [][]string{{"delete-\u007f.dat"}, {"i0.dat", "\u0130.dat"}, {"A.dat", "a0.dat", "\U0001f9ea.dat"}} {
		t.Run(names[0], func(t *testing.T) {
			root := t.TempDir()
			for _, path := range append(names, "go.mod", "go.sum", "dependency.lock.json", "third_party/raft-patch/election-random.patch") {
				full := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte("regular synthetic source\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			actual, err := FromFiles(root)
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("python3", "-c", "import json,sys;from pathlib import Path;sys.path.insert(0,sys.argv[1]);from run_evidence import source_identity;print(json.dumps(source_identity(Path(sys.argv[2]))))", filepath.Join(public, "scripts"), root)
			out, err := command.Output()
			if err != nil {
				t.Fatal(err)
			}
			var expected struct {
				SHA string `json:"source_tree_sha256"`
			}
			if err := json.Unmarshal(out, &expected); err != nil {
				t.Fatal(err)
			}
			if actual.SourceTreeSHA != expected.SHA {
				t.Fatalf("boundary filename identity differs: Go=%s Python=%s", actual.SourceTreeSHA, expected.SHA)
			}
		})
	}
}
