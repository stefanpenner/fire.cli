package transport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
		s.controlPath = controlPathFor(s.host, s.sshArgs)
	}
	if s.exec == nil {
		s.exec = makeDefaultExec(s.maxOutput)
	}
	return s
}

// Unix-domain socket path budget. Getting this arithmetic wrong does not degrade
// gracefully -- ssh dies with "unix_listener: path … too long for Unix domain
// socket" and exits 255, so EVERY command reports the box unreachable.
//
// An earlier version used ssh's %C token and assumed it expanded to ~16 chars.
// It expands to a 40-char SHA-1 hex digest. ssh then binds the master socket at
// ControlPath + "." + 16 random chars before renaming it into place. On macOS,
// os.TempDir() is already ~49 chars, so the real path reached 118 bytes against
// a 104-byte limit. We now emit a fully-expanded path (no ssh tokens) so its
// length is exactly knowable here.
const (
	// Darwin's sockaddr_un.sun_path is 104 bytes incl. NUL; Linux allows 108.
	// Use the smaller so a path built on either platform is portable.
	sunPathMax = 104
	// ssh appends "." + 16 random chars while binding, then renames.
	sshControlBindSuffix = 17
	// Longest ControlPath we may hand ssh.
	maxControlPathLen = sunPathMax - sshControlBindSuffix - 1 // -1 for the NUL
)

// defaultControlPath is the host-agnostic entry point (tests, and callers that
// have no host yet).
func defaultControlPath() string { return controlPathFor("", nil) }

// controlPathFor returns a concrete ssh ControlPath for this destination, or ""
// to disable multiplexing when no candidate directory yields a bindable path.
//
// Uniqueness comes from a digest of (host, sshArgs) rather than ssh's %C, so the
// length is deterministic. Two boxes -- or the same box on a different port --
// never share a socket.
func controlPathFor(host string, sshArgs []string) string {
	sum := sha256.Sum256([]byte(host + "\x00" + strings.Join(sshArgs, "\x00")))
	name := "cm-" + hex.EncodeToString(sum[:])[:10]

	for _, dir := range candidateControlDirs() {
		path := filepath.Join(dir, name)
		if len(path) > maxControlPathLen {
			continue // try a shorter base before giving up on multiplexing
		}
		if !ensurePrivateDir(dir) {
			continue
		}
		return path
	}
	return "" // degrade to no multiplexing; never to an unbindable path
}

// candidateControlDirs lists socket homes shortest-viable-last. $TMPDIR is
// preferred (per-user and auto-cleaned), but on macOS it is long enough to blow
// the sun_path budget, so /tmp/fire-ssh-<uid> is the fallback.
func candidateControlDirs() []string {
	return []string{
		filepath.Join(os.TempDir(), "fire-ssh"),
		filepath.Join("/tmp", "fire-ssh-"+strconv.Itoa(os.Getuid())),
	}
}

// ensurePrivateDir creates dir 0700 and refuses to use one that already exists
// but is not ours or is group/world-writable. /tmp is world-writable, so without
// this check another user could pre-create the directory and have our ssh master
// socket -- and therefore our authenticated session -- land inside it.
func ensurePrivateDir(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return false
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return false // group/world accessible
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return false // someone else owns it
	}
	return true
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

	// Containment. Connection multiplexing is an optimisation; it must never be
	// the reason a command fails. If ssh refused because of the control socket --
	// path too long, stale socket left by a killed master, a directory we cannot
	// use -- fall back to a plain connection once. Slower beats broken.
	if s.controlPath != "" && isControlSocketFailure(code, string(stderr)) {
		stdout, stderr, code, err = s.exec(ctx, "ssh", buildSSHArgs(s.host, s.sshArgs, "", command)...)
	}

	return Result{
		Stdout:   capString(string(stdout), s.maxOutput),
		Stderr:   capString(string(stderr), s.maxOutput),
		ExitCode: code,
	}, err
}

// isControlSocketFailure reports whether ssh's failure is about the multiplexing
// socket rather than the remote host or the command. ssh signals every one of
// these with exit 255, indistinguishable from "host down" without reading stderr.
func isControlSocketFailure(code int, stderr string) bool {
	if code != 255 {
		return false
	}
	for _, sig := range []string{
		"too long for Unix domain socket", // path exceeded sun_path
		"unix_listener",                   // could not bind the master socket
		"ControlPath",                     // malformed / unusable path
		"control socket",                  // stale or refused socket
		"ControlSocket",                   // "… already exists, disabling multiplexing"
		"multiplexing",
	} {
		if strings.Contains(stderr, sig) {
			return true
		}
	}
	return false
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
