# Instrumented Raft register target

`New(Config)` creates one fixed voter (ID 1, 2 or 3). Call `Step(Start{})`
before other events. A single owner serializes `Step`; the caller routes copied
`SendEffect` frames and collects `ReplyEffect` completions. Time advances only
through `Tick{}`. The simulation path uses no wall clock, sockets or files unless
the caller explicitly chooses `FileStore`.

The default `Safe` variant is `raft-register-v1`. The explicit
`unsafe-local-read` variant runs the same consensus/persistence integration but
answers leader reads from local state, providing the stale-read negative control.
It is never selected by default.

Every Ready is handled in this order: copy/encode outbound frames, before-append,
Stage(entries and HardState), after-stage-before-flush, Flush,
after-flush-before-send, release messages, after-send-before-apply, apply the
committed prefix, satisfy applied-index-gated ReadIndex reads and applied writes,
Advance. A client response has after-apply-before-response and after-response
barriers. A hook returning `ErrInjectedCrash` stops the node, discards staging and
returns the effects already released, including any completed response. The
caller must preserve this prefix even when Step returns an error. Persistence
errors stop mutation; they do not establish that an admitted write failed.

Restart by constructing a new Target with the same durable Store, node ID,
variant and continued per-node entropy stream, then call Start. Recovery replays
committed log entries to rebuild the register, deduplication and fixed membership.
Uncommitted entries remain in the durable Raft log but do not change the register.
Write retries return the replicated original result; a changed payload with the
same ID is rejected. Pending write retries may repropose identical commands.
Reads use fresh node/term/counter contexts, and admit only on a leader. A restarted
node cannot admit a read before an election in a newer durable term. Read contexts
are ephemeral and read retries run a new quorum check.

The replicated command bytes are version 1 (one byte), ASCII `w` (one byte),
request ID length (one byte), a 1..64 byte ASCII ID in `[A-Za-z0-9._:-]`, then the
signed int64 value represented by eight big-endian two's-complement bytes. The
replicated retry payload hash is SHA-256 over those complete command bytes.

MemoryStore distinguishes staged and flushed state; it is a durability model,
not physical storage. FileStore uses a real append journal and `File.Sync` before
publishing the flushed view. Journal bytes start with ASCII `FBRAFTJ1`; each record
contains an eight-byte big-endian payload length, its eight-byte bitwise
complement, a 32-byte SHA-256 of the payload and that payload. Batch payloads use
version byte 1, a uint32 big-endian HardState protobuf length and deterministic
protobuf bytes (zero length means absent), then uint32 entry count and each
entry's uint32 length plus deterministic protobuf bytes. Recovery validates
checksums and semantic log/term/vote/commit constraints, and truncates an
interrupted final record to the complete prefix. Complete corrupt records fail
closed. A removed/truncated already durable record cannot be distinguished from
an interrupted append without additional external durable metadata; this journal
does not claim detection of arbitrary file deletion or physical power-loss
certification. The operator owns the path and prepares its parent directory.

Snapshots and dynamic membership are rejected. Protobuf frames are canonical,
copied and at most 64 KiB; decoding has recursion limit 8. Default log limit is
1 MiB, configurable up to 64 MiB. The journal is bounded by the smaller of
`64*log_limit+4096` and 64 MiB; reaching either limit ends progress explicitly.
Outstanding requests and replicated write identities are capped at 4096 each.
There is no compaction or automatic dedup eviction.

The target checks the complete prospective Ready log against Config.LogLimitBytes
before Stage, on followers as well as leaders. A caller-supplied Store with larger
capacity cannot enlarge that target admission bound.

Unit tests cover majority reads/writes, partitions/healing, retries, recovery,
all six crash barriers, physical journal reopening, every truncated final-record
position, corruption, failed IO, malformed frames and ten repeated fixed logical
runs. These tests do not establish system acceptance, campaign coverage or
real-process loopback conformance.
