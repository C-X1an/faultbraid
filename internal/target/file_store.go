package target

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	pb "go.etcd.io/raft/v3/raftpb"
	"google.golang.org/protobuf/proto"
)

const maxLogLimitBytes uint64 = 64 << 20
const maxJournalBytes uint64 = 64 << 20

var journalMagic = []byte("FBRAFTJ1")

// FileStore is a checksummed append journal. Flush writes a complete record and
// calls File.Sync before exposing durable state. The caller owns the journal
// path and prepares its parent directory; this does not certify power-loss
// behavior of the filesystem or parent-directory creation.
type FileStore struct {
	*MemoryStore
	file         *os.File
	poisoned     bool
	journalLimit uint64
}

func NewFileStore(path string, limit uint64) (*FileStore, error) {
	if limit == 0 {
		limit = DefaultLogLimitBytes
	}
	if limit > maxLogLimitBytes {
		return nil, ErrLimit
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	s := &FileStore{MemoryStore: NewMemoryStore(limit), file: f, journalLimit: min(limit*64+4096, maxJournalBytes)}
	if _, err = s.Recover(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return s, nil
}
func (s *FileStore) Close() error { s.poisoned = true; return s.file.Close() }
func (s *FileStore) Stage(b Batch) error {
	if s.poisoned {
		return ErrStopped
	}
	return s.MemoryStore.Stage(b)
}
func (s *FileStore) Flush() error {
	if s.poisoned {
		return ErrStopped
	}
	if s.staged == nil {
		return fmt.Errorf("target: no staged batch")
	}
	// Empty Ready batches introduce no journal record.
	if len(s.batch.Entries) == 0 && s.batch.HardState == nil {
		return s.MemoryStore.Flush()
	}
	payload, err := encodeBatch(s.batch)
	if err != nil {
		return err
	}
	record := make([]byte, 48+len(payload))
	binary.BigEndian.PutUint64(record, uint64(len(payload)))
	binary.BigEndian.PutUint64(record[8:], ^uint64(len(payload)))
	sum := sha256.Sum256(payload)
	copy(record[16:48], sum[:])
	copy(record[48:], payload)
	info, err := s.file.Stat()
	if err != nil {
		s.poisoned = true
		return err
	}
	if uint64(info.Size())+uint64(len(record)) > s.journalLimit {
		s.poisoned = true
		return ErrLimit
	}
	if _, err = s.file.Seek(0, io.SeekEnd); err == nil {
		err = writeFull(s.file, record)
	}
	if err == nil {
		err = s.file.Sync()
	}
	if err != nil {
		s.poisoned = true
		return err
	}
	return s.MemoryStore.Flush()
}
func writeFull(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func (s *FileStore) Recover() (State, error) {
	if s.poisoned {
		return State{}, ErrStopped
	}
	s.staged = nil
	s.batch = Batch{}
	info, err := s.file.Stat()
	if err != nil {
		return State{}, err
	}
	if info.Size() < 0 || uint64(info.Size()) > s.journalLimit {
		return State{}, ErrLimit
	}
	if _, err = s.file.Seek(0, io.SeekStart); err != nil {
		return State{}, err
	}
	data, err := io.ReadAll(io.LimitReader(s.file, int64(s.journalLimit)+1))
	if err != nil {
		return State{}, err
	}
	if len(data) == 0 {
		if err = writeFull(s.file, journalMagic); err == nil {
			err = s.file.Sync()
		}
		if err != nil {
			s.poisoned = true
			return State{}, err
		}
		data = append([]byte(nil), journalMagic...)
	}
	if len(data) < len(journalMagic) || !bytes.Equal(data[:len(journalMagic)], journalMagic) {
		s.poisoned = true
		return State{}, ErrCorrupt
	}
	recovered := State{ConfState: &pb.ConfState{}}
	pos := len(journalMagic)
	for pos < len(data) {
		if len(data)-pos < 48 {
			break
		} // Interrupted last record: retain complete prefix.
		n := binary.BigEndian.Uint64(data[pos:])
		if n == 0 || n > s.limit+4096 || binary.BigEndian.Uint64(data[pos+8:]) != ^n {
			s.poisoned = true
			return State{}, ErrCorrupt
		}
		if n > uint64(len(data)-pos-48) {
			break
		}
		payload := data[pos+48 : pos+48+int(n)]
		sum := sha256.Sum256(payload)
		if !bytes.Equal(sum[:], data[pos+16:pos+48]) {
			s.poisoned = true
			return State{}, ErrCorrupt
		}
		b, e := decodeBatch(payload, s.limit)
		if e != nil {
			s.poisoned = true
			return State{}, e
		}
		recovered, e = mergeBatch(recovered, b, s.limit)
		if e != nil {
			s.poisoned = true
			return State{}, e
		}
		pos += 48 + int(n)
	}
	if pos != len(data) {
		if err = s.file.Truncate(int64(pos)); err == nil {
			err = s.file.Sync()
		}
		if err != nil {
			s.poisoned = true
			return State{}, err
		}
	}
	s.durable = recovered
	return cloneState(recovered), nil
}
func encodeBatch(b Batch) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte(1)
	writeProto := func(m proto.Message) error {
		data, err := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
		if err != nil {
			return err
		}
		_ = binary.Write(&out, binary.BigEndian, uint32(len(data)))
		_, _ = out.Write(data)
		return nil
	}
	if b.HardState == nil {
		_ = binary.Write(&out, binary.BigEndian, uint32(0))
	} else if err := writeProto(b.HardState); err != nil {
		return nil, err
	}
	_ = binary.Write(&out, binary.BigEndian, uint32(len(b.Entries)))
	for _, e := range b.Entries {
		if err := writeProto(e); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}
func decodeBatch(data []byte, limit uint64) (Batch, error) {
	if len(data) < 9 || data[0] != 1 {
		return Batch{}, ErrCorrupt
	}
	r := bytes.NewReader(data[1:])
	var b Batch
	readProto := func(m proto.Message) (int, error) {
		var n uint32
		if binary.Read(r, binary.BigEndian, &n) != nil || uint64(n) > limit+1024 || uint64(n) > uint64(r.Len()) {
			return 0, ErrCorrupt
		}
		raw := make([]byte, int(n))
		if _, err := io.ReadFull(r, raw); err != nil {
			return 0, ErrCorrupt
		}
		if err := proto.Unmarshal(raw, m); err != nil || len(m.ProtoReflect().GetUnknown()) != 0 {
			return 0, ErrCorrupt
		}
		return int(n), nil
	}
	h := &pb.HardState{}
	n, err := readProto(h)
	if err != nil {
		return b, err
	}
	if n != 0 {
		b.HardState = h
	}
	var count uint32
	if binary.Read(r, binary.BigEndian, &count) != nil || uint64(count) > limit/4 || uint64(count) > uint64(r.Len()/4) {
		return b, ErrCorrupt
	}
	for i := uint32(0); i < count; i++ {
		e := &pb.Entry{}
		if _, err = readProto(e); err != nil {
			return b, err
		}
		b.Entries = append(b.Entries, e)
	}
	if r.Len() != 0 {
		return b, ErrCorrupt
	}
	return b, nil
}
