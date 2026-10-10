package target

import (
	"crypto/sha256"
	"encoding/binary"
)

// The replicated command codec is fixed-width except for the bounded request
// ID. Hash covers the complete payload, including codec version and operation.
type command struct {
	id    string
	value int64
	hash  [32]byte
}

func validID(id string) bool {
	if len(id) < 1 || len(id) > 64 {
		return false
	}
	for _, c := range []byte(id) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-') {
			return false
		}
	}
	return true
}
func encodeCommand(id string, value int64) []byte {
	b := make([]byte, 3+len(id)+8)
	b[0] = 1
	b[1] = 'w'
	b[2] = byte(len(id))
	copy(b[3:], id)
	binary.BigEndian.PutUint64(b[3+len(id):], uint64(value))
	return b
}
func decodeCommand(b []byte) (command, error) {
	if len(b) < 12 || b[0] != 1 || b[1] != 'w' || int(b[2])+11 != len(b) {
		return command{}, ErrCorrupt
	}
	id := string(b[3 : 3+int(b[2])])
	if !validID(id) {
		return command{}, ErrCorrupt
	}
	return command{id: id, value: int64(binary.BigEndian.Uint64(b[len(b)-8:])), hash: sha256.Sum256(b)}, nil
}
