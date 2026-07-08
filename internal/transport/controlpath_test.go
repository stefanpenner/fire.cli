package transport

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// How ssh actually uses ControlPath, and why the naive length guard was wrong:
//
//   - %C expands to a SHA-1 hex digest: 40 characters, not the ~16 that
//     defaultControlPath assumed.
//   - Before renaming the master socket into place, ssh binds it at
//     ControlPath + "." + 16 random characters (a 17-char suffix).
//   - Darwin's sockaddr_un.sun_path holds 104 bytes including the NUL.
//
// On macOS, os.TempDir() is ~49 chars (/var/folders/xx/…/T/), so the expanded
// path lands at ~118 bytes. ssh then dies with
//
//	unix_listener: path "…" too long for Unix domain socket
//
// and exits 255 -- which surfaced as EVERY fire command reporting the box
// unreachable, while the whole unit suite stayed green.
const (
	sshPercentCLen   = 40 // %C -> SHA-1 hex
	sshBindSuffixLen = 17 // "." + 16 random chars
	darwinSunPathMax = 104
)

// realisticDarwinTmpDir mirrors the shape of a real macOS $TMPDIR so this test
// does not depend on the ambient environment of whoever runs it.
const realisticDarwinTmpDir = "/var/folders/0s/_r7rjfvn0hg408gqkppxhn7m0000gn/T"

// The real thing: whatever path we hand ssh must be bindable as a Unix socket
// on this platform, after ssh expands %C and appends its temporary suffix.
// A pure length assertion would not have caught this -- the old guard *did*
// arithmetic, it just did the wrong arithmetic. Bind the socket instead.
func TestDefaultControlPath_IsBindableAfterSSHExpansion(t *testing.T) {
	t.Setenv("TMPDIR", realisticDarwinTmpDir)

	p := defaultControlPath()
	if p == "" {
		t.Skip("multiplexing disabled; nothing to bind")
	}

	bindPath := sshExpand(p)
	require.NoError(t, os.MkdirAll(filepath.Dir(bindPath), 0o700))
	t.Cleanup(func() { os.Remove(bindPath) })

	l, err := net.Listen("unix", bindPath)
	require.NoError(t, err,
		"ssh cannot bind this ControlPath (%d bytes) -> every fire command exits 255", len(bindPath))
	require.NoError(t, l.Close())
}

// Belt and braces: the length must fit even before we try to bind, so the
// failure mode is "multiplexing off", never "every command breaks".
func TestDefaultControlPath_FitsSunPathUnderLongTmpDir(t *testing.T) {
	t.Setenv("TMPDIR", realisticDarwinTmpDir)

	p := defaultControlPath()
	if p == "" {
		return // acceptable: multiplexing disabled rather than broken
	}
	assert.Less(t, len(sshExpand(p)), darwinSunPathMax,
		"expanded ControlPath must fit sockaddr_un.sun_path")
}

// A pathological TMPDIR must degrade to no-multiplexing, not to a broken path.
func TestDefaultControlPath_DisablesMultiplexWhenItCannotFit(t *testing.T) {
	t.Setenv("TMPDIR", "/"+strings.Repeat("d", 200))

	p := defaultControlPath()
	if p == "" {
		return
	}
	assert.Less(t, len(sshExpand(p)), darwinSunPathMax,
		"must return \"\" (multiplex off) rather than an unbindable path")
}

// Distinct boxes must never share a control socket, or `fire --box a` would
// tunnel its commands through the connection opened for `fire --box b`.
func TestControlPathFor_DistinctPerHostAndArgs(t *testing.T) {
	t.Setenv("TMPDIR", realisticDarwinTmpDir)

	a := controlPathFor("pi@fire.walla", nil)
	b := controlPathFor("pi@other.box", nil)
	require.NotEmpty(t, a)
	assert.NotEqual(t, a, b, "different hosts must get different control sockets")

	c := controlPathFor("pi@fire.walla", []string{"-p", "2222"})
	assert.NotEqual(t, a, c, "different ssh args must get different control sockets")

	// Stable across calls, or multiplexing would never actually reuse anything.
	assert.Equal(t, a, controlPathFor("pi@fire.walla", nil))
}

// The path we emit must contain no ssh tokens: their expansion is what made the
// length unknowable, and unknowable length is what broke every command.
func TestControlPathFor_ContainsNoSSHTokens(t *testing.T) {
	t.Setenv("TMPDIR", realisticDarwinTmpDir)
	assert.NotContains(t, controlPathFor("pi@fire.walla", nil), "%")
}

// A directory that already exists but is world-writable (or not ours) must be
// refused -- /tmp is world-writable, and our master socket carries an
// authenticated session.
func TestEnsurePrivateDir_RejectsWorldWritable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "loose")
	require.NoError(t, os.MkdirAll(dir, 0o777))
	require.NoError(t, os.Chmod(dir, 0o777))

	assert.False(t, ensurePrivateDir(dir), "0777 dir must be refused")
	assert.True(t, ensurePrivateDir(filepath.Join(t.TempDir(), "fresh")), "fresh 0700 dir is fine")
}

// ssh reports a broken control socket with exit 255 -- exactly what a dead box
// looks like. Only stderr distinguishes them.
func TestIsControlSocketFailure(t *testing.T) {
	assert.True(t, isControlSocketFailure(255,
		`unix_listener: path "/var/folders/…/cm-abc.WB0T6e5x" too long for Unix domain socket`))
	assert.True(t, isControlSocketFailure(255, "ControlSocket /tmp/x already exists, disabling multiplexing"))

	// A genuinely unreachable host must NOT be mistaken for a socket problem,
	// or we would silently retry every real outage.
	assert.False(t, isControlSocketFailure(255, "ssh: connect to host fire.walla port 22: No route to host"))
	assert.False(t, isControlSocketFailure(255, "Permission denied (publickey)."))
	assert.False(t, isControlSocketFailure(1, "unix_listener: whatever"), "non-255 is the remote command's own failure")
}

// Multiplexing is an optimisation. When the control socket is unusable, the
// command must still run -- once, without it.
func TestRun_FallsBackToPlainConnectionOnControlSocketFailure(t *testing.T) {
	tr := NewSSH("pi@fire")
	require.NotEmpty(t, tr.controlPath)

	var calls [][]string
	tr.exec = func(_ context.Context, _ string, args ...string) ([]byte, []byte, int, error) {
		calls = append(calls, args)
		if len(calls) == 1 {
			return nil, []byte("unix_listener: path \"…\" too long for Unix domain socket"), 255,
				errors.New("exit status 255")
		}
		return []byte("PONG\n"), nil, 0, nil
	}

	res, err := tr.Run(context.Background(), "redis-cli ping")

	require.NoError(t, err)
	assert.Equal(t, "PONG\n", res.Stdout)
	assert.Equal(t, 0, res.ExitCode)
	require.Len(t, calls, 2, "must retry exactly once")
	assert.Contains(t, strings.Join(calls[0], " "), "ControlPath=")
	assert.NotContains(t, strings.Join(calls[1], " "), "ControlPath=", "retry must drop multiplexing")
}

// A real outage must not be retried -- one connection attempt, one failure.
func TestRun_DoesNotRetryOnGenuineUnreachableHost(t *testing.T) {
	tr := NewSSH("pi@fire")

	var n int
	tr.exec = func(context.Context, string, ...string) ([]byte, []byte, int, error) {
		n++
		return nil, []byte("ssh: connect to host fire.walla port 22: No route to host"), 255,
			errors.New("exit status 255")
	}

	_, err := tr.Run(context.Background(), "uptime")

	require.Error(t, err)
	assert.Equal(t, 1, n, "a down box must not be probed twice")
}

// sshExpand mimics what ssh does to ControlPath before binding: expand %C to a
// 40-char digest, then append the 17-char temporary suffix.
func sshExpand(p string) string {
	p = strings.ReplaceAll(p, "%C", strings.Repeat("a", sshPercentCLen))
	return p + "." + strings.Repeat("x", sshBindSuffixLen-1)
}
