package buildinfo

import (
	"encoding/json"
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
