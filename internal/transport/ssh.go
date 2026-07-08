package transport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// defaultMaxOutput bounds how many bytes of stdout/stderr a single command may
// produce before the rest is dropped. It guards against OOM on a hostile or
// runaway keyspace (e.g. a huge `redis-cli --scan`). Generous enough that no
// legitimate command is truncated.
const defaultMaxOutput = 32 << 20 // 32 MiB

// execRunner is the seam we mock in tests so SSHTransport.Run can be exercised
// without a real ssh binary or network. It mirrors exec.CommandContext.
type execRunner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)

// SSHTransport runs commands on a Firewalla box by shelling out to the system
// ssh client. Shelling out (rather than using x/crypto/ssh) means we inherit
// the user's ssh config, keys, agent, and known_hosts for free.
type SSHTransport struct {
	host        string
	sshArgs     []string
	timeout     time.Duration // per-command wall-clock bound; 0 = none
	maxOutput   int           // cap on stdout/stderr bytes; <=0 = unlimited
	controlPath string        // ssh ControlPath for connection reuse; "" disables
	noMultiplex bool
	exec        execRunner
}

// Option configures an SSHTransport.
type Option func(*SSHTransport)

// WithSSHArgs adds extra arguments passed to ssh before the host (e.g. "-p",
// "2222" or "-i", "/path/to/key").
func WithSSHArgs(args ...string) Option {
	return func(s *SSHTransport) { s.sshArgs = append(s.sshArgs, args...) }
}

// WithTimeout bounds each command's total wall-clock time. Once it elapses the
// context is canceled and Run returns promptly with an error instead of
// hanging on a stalled box. 0 (the default) imposes no deadline.
func WithTimeout(d time.Duration) Option {
	return func(s *SSHTransport) { s.timeout = d }
}

// WithMaxOutput overrides the stdout/stderr byte cap. <=0 disables the cap.
func WithMaxOutput(n int) Option {
	return func(s *SSHTransport) { s.maxOutput = n }
}

// WithoutMultiplex disables ssh connection reuse (ControlMaster). Useful when
// the environment forbids control sockets.
func WithoutMultiplex() Option {
	return func(s *SSHTransport) { s.noMultiplex = true }
}

// NewSSH creates an SSHTransport for the given ssh destination (e.g. "pi@fire").
// Connection multiplexing is on by default: repeated commands (and the TUI's
// many loads) share one ssh connection instead of paying a TCP + auth
// handshake each time.
func NewSSH(host string, opts ...Option) *SSHTransport {
	s := &SSHTransport{host: host, maxOutput: defaultMaxOutput}
	for _, o := range opts {
		o(s)
	}
	if !s.noMultiplex {
		s.controlPath = defaultControlPath()
	}
	if s.exec == nil {
		s.exec = makeDefaultExec(s.maxOutput)
	}
	return s
}

// defaultControlPath returns an ssh ControlPath template in a private temp dir,
// or "" if it can't be created or would exceed the platform's socket path
// limit (macOS caps sun_path at ~104 bytes). The %C token is expanded by ssh to
// a hash of (localhost, remotehost, port, user), so distinct boxes never share
// a socket.
func defaultControlPath() string {
	dir := filepath.Join(os.TempDir(), "fire-ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	path := filepath.Join(dir, "cm-%C")
	if len(path)+16 > 100 { // %C expands to ~16 hex chars
		return ""
	}
	return path
}

// Host returns the ssh destination.
func (s *SSHTransport) Host() string { return s.host }

// Run executes command on the remote host over ssh, bounded by the configured
// timeout and output cap.
func (s *SSHTransport) Run(ctx context.Context, command string) (Result, error) {
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	stdout, stderr, code, err := s.exec(ctx, "ssh", buildSSHArgs(s.host, s.sshArgs, s.controlPath, command)...)
	return Result{
		Stdout:   capString(string(stdout), s.maxOutput),
		Stderr:   capString(string(stderr), s.maxOutput),
		ExitCode: code,
	}, err
}

// buildSSHArgs assembles the ssh argument vector. It is a pure function so the
// argument ordering (defaults, then extra options, then host, then the remote
// command as the final positional arg) can be asserted in tests.
func buildSSHArgs(host string, extra []string, controlPath, command string) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=8",
	}
	if controlPath != "" {
		args = append(args,
			"-o", "ControlMaster=auto",
			"-o", "ControlPath="+controlPath,
			"-o", "ControlPersist=30s",
		)
	}
	args = append(args, extra...)
	args = append(args, host, command)
	return args
}

// makeDefaultExec builds the production execRunner backed by os/exec, capping
// each stream to limit bytes so a runaway command can't balloon memory even
// before Run truncates the result.
func makeDefaultExec(limit int) execRunner {
	return func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		stdout := &capWriter{limit: limit}
		stderr := &capWriter{limit: limit}
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		code := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
		return stdout.Bytes(), stderr.Bytes(), code, err
	}
}

// capString truncates s to n bytes when n > 0.
func capString(s string, n int) string {
	if n > 0 && len(s) > n {
		return s[:n]
	}
	return s
}

// capWriter is an io.Writer that retains at most limit bytes and drops the
// rest, recording whether truncation occurred. It always reports the full
// write length so the writing command does not see a short-write error.
// limit <= 0 means unbounded.
type capWriter struct {
	buf       []byte
	limit     int
	truncated bool
}

func (w *capWriter) Write(p []byte) (int, error) {
	if w.limit <= 0 {
		w.buf = append(w.buf, p...)
		return len(p), nil
	}
	if room := w.limit - len(w.buf); room > 0 {
		if len(p) <= room {
			w.buf = append(w.buf, p...)
		} else {
			w.buf = append(w.buf, p[:room]...)
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}
	return len(p), nil
}

func (w *capWriter) Bytes() []byte  { return w.buf }
func (w *capWriter) String() string { return string(w.buf) }
