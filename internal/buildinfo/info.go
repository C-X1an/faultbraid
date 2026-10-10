// Package buildinfo holds source identities injected by the reproducible build.
package buildinfo

import "encoding/hex"

var SourceTreeSHA = "unrecorded"
var ModuleLockSHA = "unrecorded"
var PatchSHA = "unrecorded"

const SchedulerVersion = "stable-time-sequence-v1"
const EntropyVersion = "splitmix64-v1"

type Identity struct {
	SourceTreeSHA string `json:"source_tree_sha256"`
	ModuleLockSHA string `json:"module_lock_sha256"`
	PatchSHA      string `json:"patch_sha256"`
}

func Current() Identity { return Identity{SourceTreeSHA, ModuleLockSHA, PatchSHA} }

func (i Identity) Identified() bool {
	for _, value := range []string{i.SourceTreeSHA, i.ModuleLockSHA, i.PatchSHA} {
		data, err := hex.DecodeString(value)
		if err != nil || len(data) != 32 {
			return false
		}
		var nonzero byte
		for _, b := range data {
			nonzero |= b
		}
		if nonzero == 0 {
			return false
		}
	}
	return true
}
