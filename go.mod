module github.com/C-X1an/faultbraid

go 1.26.0

require (
	github.com/anishathalye/porcupine v1.3.1 // indirect
	go.etcd.io/raft/v3 v3.7.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace go.etcd.io/raft/v3 => ./.local-runs/raft
