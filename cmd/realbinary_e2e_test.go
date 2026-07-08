package cmd

import (
	"bytes"
	"context"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_RealBinary drives the actual built `fire` executable — not cobra
// in-process — through a fake `ssh` on PATH. cmd/e2e_test.go proves the
// cmd→firewalla.Client→parser→renderer wiring; this proves the same thing
// survives linking and process startup, which in-process tests can't catch
// (init-order, argv/env parsing, real exec.Command plumbing in
// transport.SSHTransport).
func TestE2E_RealBinary(t *testing.T) {
	if _, err := osexec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	repoRoot, err := filepath.Abs("..")
	require.NoError(t, err)

	// Build the real binary once for this test.
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "fire")
	buildCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	buildCmd := osexec.CommandContext(buildCtx, "go", "build", "-o", binPath, ".")
	buildCmd.Dir = repoRoot
	var buildOut bytes.Buffer
	buildCmd.Stdout = &buildOut
	buildCmd.Stderr = &buildOut
	require.NoError(t, buildCmd.Run(), "go build failed:\n%s", buildOut.String())

	// A fake `ssh` that ignores every argument and cats an anonymized device
	// fixture to stdout, exit 0 — standing in for the real ssh binary that
	// transport.SSHTransport shells out to.
	fixture, err := os.ReadFile(filepath.Join(repoRoot, "internal", "firewalla", "testdata", "devices.txt"))
	require.NoError(t, err)
	fakeBinDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fakeBinDir, "devices_fixture.txt"), fixture, 0o644))
	sshScript := "#!/bin/sh\ncat \"$(dirname \"$0\")/devices_fixture.txt\"\n"
	sshPath := filepath.Join(fakeBinDir, "ssh")
	require.NoError(t, os.WriteFile(sshPath, []byte(sshScript), 0o755))

	fakePATH := fakeBinDir + string(os.PathListSeparator) + os.Getenv("PATH")

	t.Run("devices via fake ssh", func(t *testing.T) {
		runCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := osexec.CommandContext(runCtx, binPath, "--host", "pi@fake", "devices")
		cmd.Env = append(os.Environ(), "PATH="+fakePATH)
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		err := cmd.Run()
		require.NoError(t, err, "stderr: %s", errOut.String())
		assert.Contains(t, out.String(), "Example Phone", "anonymized device name from the fixture")
	})

	t.Run("version needs no ssh", func(t *testing.T) {
		runCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Deliberately empty PATH so a call out to `ssh` would fail loudly —
		// version must not need it.
		cmd := osexec.CommandContext(runCtx, binPath, "version")
		cmd.Env = append(os.Environ(), "PATH=")
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		err := cmd.Run()
		require.NoError(t, err, "stderr: %s", errOut.String())
		assert.NotEmpty(t, out.String())
	})
}
