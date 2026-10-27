package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

// tunnelRunner opens and closes tunnels; the app talks to it through this
// interface so tests need no ssh.
type tunnelRunner interface {
	start(t Tunnel) error
	stop(id string)
}

// sshRunner keeps one ssh process per open tunnel. Its callbacks run on
// the process's goroutine.
type sshRunner struct {
	ready  func(id string)             // ssh connected and set up the forward
	failed func(id string, msg string) // a connection through the open tunnel failed
	exited func(id string, msg string) // ssh exited on its own, msg says why

	mu    sync.Mutex
	procs map[string]*exec.Cmd
}

func newSSHRunner(ready func(id string), failed, exited func(id, msg string)) *sshRunner {
	return &sshRunner{ready: ready, failed: failed, exited: exited, procs: map[string]*exec.Cmd{}}
}

func (r *sshRunner) start(t Tunnel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.procs[t.ID] != nil {
		return nil
	}
	cmd := exec.Command("ssh", t.Args()...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	// While the tunnel is open, ssh tells on stderr of each connection it
	// could not forward, as "channel 2: open failed: connect failed: ...".
	errs := &tail{onLine: func(line string) {
		if strings.Contains(line, "open failed") && r.current(t.ID, cmd) {
			r.failed(t.ID, line)
		}
	}}
	cmd.Stderr = errs
	if err := cmd.Start(); err != nil {
		return err
	}
	r.procs[t.ID] = cmd
	go r.watch(t.ID, cmd, bufio.NewScanner(stdout), errs)
	return nil
}

// watch waits for the marker LocalCommand prints, then for the process
// to exit, reporting only what the process did on its own: a tunnel
// stopped with stop is no longer in procs.
func (r *sshRunner) watch(id string, cmd *exec.Cmd, out *bufio.Scanner, errs *tail) {
	for out.Scan() {
		if strings.TrimSpace(out.Text()) == readyMarker && r.current(id, cmd) {
			r.ready(id)
		}
	}
	err := cmd.Wait()
	r.mu.Lock()
	ours := r.procs[id] == cmd
	if ours {
		delete(r.procs, id)
	}
	r.mu.Unlock()
	if ours {
		r.exited(id, exitMessage(errs.lastLine(), err))
	}
}

func (r *sshRunner) current(id string, cmd *exec.Cmd) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.procs[id] == cmd
}

func (r *sshRunner) stop(id string) {
	r.mu.Lock()
	cmd := r.procs[id]
	delete(r.procs, id)
	r.mu.Unlock()
	if cmd != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}
}

func (r *sshRunner) stopAll() {
	r.mu.Lock()
	ids := make([]string, 0, len(r.procs))
	for id := range r.procs {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		r.stop(id)
	}
}

func exitMessage(stderr string, err error) string {
	switch {
	case stderr != "":
		return stderr
	case err != nil:
		return fmt.Sprintf("ssh 已退出：%v", err)
	default:
		return "连接已关闭"
	}
}

// tail keeps the last few KB ssh writes to stderr, enough for its last
// error, however long the tunnel runs, and hands onLine each line as it
// completes.
type tail struct {
	b      []byte
	line   []byte // the line being written
	onLine func(string)
}

const tailSize = 4096

func (t *tail) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > tailSize {
		t.b = t.b[len(t.b)-tailSize:]
	}
	rest := p
	for {
		before, after, found := bytes.Cut(rest, []byte("\n"))
		t.line = append(t.line, before...)
		if !found {
			break
		}
		if line := strings.TrimSpace(string(t.line)); line != "" && t.onLine != nil {
			t.onLine(line)
		}
		t.line, rest = t.line[:0], after
	}
	if len(t.line) > tailSize { // a line without end
		t.line = t.line[:0]
	}
	return len(p), nil
}

func (t *tail) lastLine() string {
	lines := strings.Split(strings.TrimSpace(string(t.b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
