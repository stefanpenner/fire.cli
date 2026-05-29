package transport

import (
	"bytes"
	"context"
	"os/exec"
)

// execRunner is the seam we mock in tests so SSHTransport.Run can be exercised
// without a real ssh binary or network. It mirrors exec.CommandContext.
type execRunner func(ctx context.Context, name string, args ...string) (stdout, stderr []byte, exitCode int, err error)

// SSHTransport runs commands on a Firewalla box by shelling out to the system
// ssh client. Shelling out (rather than using x/crypto/ssh) means we inherit
// the user's ssh config, keys, agent, and known_hosts for free.
type SSHTransport struct {
	host    string
	sshArgs []string
	exec    execRunner
}

// Option configures an SSHTransport.
type Option func(*SSHTransport)

// WithSSHArgs adds extra arguments passed to ssh before the host (e.g. "-p",
// "2222" or "-i", "/path/to/key").
func WithSSHArgs(args ...string) Option {
	return func(s *SSHTransport) { s.sshArgs = append(s.sshArgs, args...) }
}

// NewSSH creates an SSHTransport for the given ssh destination (e.g. "pi@fire").
func NewSSH(host string, opts ...Option) *SSHTransport {
	s := &SSHTransport{host: host, exec: defaultExec}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Host returns the ssh destination.
func (s *SSHTransport) Host() string { return s.host }

// Run executes command on the remote host over ssh.
func (s *SSHTransport) Run(ctx context.Context, command string) (Result, error) {
	stdout, stderr, code, err := s.exec(ctx, "ssh", buildSSHArgs(s.host, s.sshArgs, command)...)
	return Result{Stdout: string(stdout), Stderr: string(stderr), ExitCode: code}, err
}

// buildSSHArgs assembles the ssh argument vector. It is a pure function so the
// argument ordering (defaults, then extra options, then host, then the remote
// command as the final positional arg) can be asserted in tests.
func buildSSHArgs(host string, extra []string, command string) []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=8",
	}
	args = append(args, extra...)
	args = append(args, host, command)
	return args
}

// defaultExec is the production execRunner backed by os/exec.
func defaultExec(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	}
	return stdout.Bytes(), stderr.Bytes(), code, err
}
