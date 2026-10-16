package labreal

// Real owned-process cleanup controls for the independent synthetic helper.
// They provide no Raft, journal, network conformance or acceptance credit.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func closeIndependentStream(file *os.File) error {
	if file == nil {
		return nil
	}
	err := file.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil
	}
	return err
}

// The failure path must join the actual command.Wait goroutine before closing
// its output files. Natural exit status and l.stop's verdict remain untouched.
func cleanupIndependentChild(c *child) error {
	var result error
	if c.owner != nil {
		select {
		case <-c.waited:
		default:
			result = errors.Join(result, c.owner.Kill())
		}
		timer := time.NewTimer(4 * time.Second)
		select {
		case <-c.waited:
		case <-timer.C:
			result = errors.Join(result, errors.New("independent helper Wait cleanup deadline exceeded"))
		}
		timer.Stop()
		result = errors.Join(result, c.owner.Close())
	}
	return errors.Join(result, closeIndependentStream(c.stdout), closeIndependentStream(c.stderr))
}

func TestIndependentOwnedHelperCleanupJoinsWaitAndClosesStreams(t *testing.T) {
	directory := t.TempDir()
	stdout, err := os.OpenFile(filepath.Join(directory, "helper.stdout"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeIndependentStream(stdout) })
	stderr, err := os.OpenFile(filepath.Join(directory, "helper.stderr"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeIndependentStream(stderr) })
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := &child{stdout: stdout, stderr: stderr, waited: make(chan struct{})}
	c.command = exec.Command(executable, "s42-cleanup-wait")
	c.command.Stdout, c.command.Stderr = stdout, stderr
	c.owner, err = startOwned(c.command)
	if err != nil {
		t.Fatal(err)
	}
	actualWaitReturned := make(chan struct{})
	allowWaitHandoff := make(chan struct{})
	cleanupReturned := make(chan struct{})
	cleanupLaunched := false
	var releaseOnce sync.Once
	releaseWait := func() { releaseOnce.Do(func() { close(allowWaitHandoff) }) }
	defer releaseWait()
	go func() {
		c.waitErr = c.command.Wait()
		close(actualWaitReturned)
		// A controlled handoff AFTER real Wait makes a bypass of the join
		// reliably observable. The process and Wait itself are real.
		<-allowWaitHandoff
		close(c.waited)
	}()
	t.Cleanup(func() {
		releaseWait()
		if cleanupLaunched {
			select {
			case <-cleanupReturned:
			case <-time.After(4 * time.Second):
				t.Error("first cleanup goroutine did not finish before final cleanup")
				return
			}
		}
		if err := cleanupIndependentChild(c); err != nil {
			t.Errorf("final real helper cleanup: %v", err)
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(stdout.Name())
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte("independent cleanup helper ready")) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("real synthetic cleanup helper did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// On Windows retain a separate real process handle while the helper is
	// known to be alive. Its Wait supplies an independent OS exit observation.
	// Calling Kill on the command's released handle is not a liveness oracle:
	// Go's Windows Wait deliberately releases it and later Kill returns EINVAL.
	var exitProbe *os.Process
	if runtime.GOOS == "windows" {
		exitProbe, err = os.FindProcess(c.command.Process.Pid)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = exitProbe.Release() })
	}
	done := make(chan error, 1)
	cleanupLaunched = true
	go func() {
		done <- cleanupIndependentChild(c)
		close(cleanupReturned)
	}()
	select {
	case <-actualWaitReturned:
	case <-time.After(4 * time.Second):
		t.Fatal("owned cleanup did not terminate the actual helper")
	}
	select {
	case err := <-done:
		t.Fatalf("cleanup returned before joining Wait handoff: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	releaseWait()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("cleanup did not finish after Wait handoff")
	}
	select {
	case <-c.waited:
	default:
		t.Fatal("actual Wait goroutine was not joined")
	}
	if c.command.ProcessState == nil || c.waitErr == nil {
		t.Fatal("actual helper was not stopped before its natural exit")
	}
	var exitError *exec.ExitError
	if !errors.As(c.waitErr, &exitError) || exitError.ProcessState != c.command.ProcessState {
		t.Fatal("actual Wait did not retain the unsuccessful helper exit state")
	}
	if exitProbe != nil {
		probeDone := make(chan error, 1)
		go func() {
			state, err := exitProbe.Wait()
			if err == nil && (state.Pid() != c.command.Process.Pid || state.ExitCode() != 1) {
				err = fmt.Errorf("independent Windows exit observation: %v", state)
			}
			probeDone <- err
		}()
		select {
		case err := <-probeDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(4 * time.Second):
			t.Fatal("actual helper remained live on its separately retained process handle")
		}
	}
	for _, file := range []*os.File{stdout, stderr} {
		if _, err := file.Write([]byte("must be closed")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("stream remained open: %s: %v", filepath.Base(file.Name()), err)
		}
		data, err := os.ReadFile(file.Name())
		if err != nil || bytes.Contains(data, []byte("helper late output")) {
			t.Fatalf("unexpected finalized output: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		after, err := os.ReadFile(file.Name())
		if err != nil || !bytes.Equal(data, after) {
			t.Fatalf("output changed after cleanup: %s: %v", filepath.Base(file.Name()), err)
		}
		if err := os.Rename(file.Name(), file.Name()+".closed"); err != nil {
			t.Fatal(fmt.Errorf("closed stream cannot be moved: %w", err))
		}
	}
}
