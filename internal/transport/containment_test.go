package transport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSHTransport_Run_Timeout proves a command that never returns is bounded:
// Run cancels the context after the configured timeout and returns promptly
// (degrading to an error instead of hanging forever).
func TestSSHTransport_Run_Timeout(t *testing.T) {
	tr := NewSSH("pi@fire", WithTimeout(20*time.Millisecond))
	tr.exec = func(ctx context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
		<-ctx.Done() // simulate a hung remote command
		return nil, nil, 0, ctx.Err()
	}

	start := time.Now()
	_, err := tr.Run(context.Background(), "sleep 999")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 2*time.Second, "Run must not block past its timeout")
}

// TestSSHTransport_Run_NoTimeoutByDefault: a zero timeout leaves the caller's
// context untouched (no artificial deadline).
func TestSSHTransport_Run_NoTimeoutByDefault(t *testing.T) {
	tr := NewSSH("pi@fire")
	var hadDeadline bool
	tr.exec = func(ctx context.Context, _ string, _ ...string) ([]byte, []byte, int, error) {
		_, hadDeadline = ctx.Deadline()
		return []byte("ok"), nil, 0, nil
	}
	_, err := tr.Run(context.Background(), "uptime")
	require.NoError(t, err)
	assert.False(t, hadDeadline, "no timeout configured → no deadline imposed")
}

// TestCapWriter_TruncatesBeyondLimit proves the output cap bounds memory: bytes
// past the limit are dropped and truncation is recorded.
func TestCapWriter_TruncatesBeyondLimit(t *testing.T) {
	w := &capWriter{limit: 10}
	n, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	// Writer reports the full length written (so the command isn't seen as
	// failing with a short write) but only keeps up to the limit.
	n, err = w.Write([]byte(" world, this is way too long"))
	require.NoError(t, err)
	assert.Equal(t, len(" world, this is way too long"), n)
	assert.Equal(t, "hello worl", w.String())
	assert.True(t, w.truncated)
	assert.LessOrEqual(t, len(w.String()), 10)
}

func TestCapWriter_UnderLimit(t *testing.T) {
	w := &capWriter{limit: 100}
	_, _ = w.Write([]byte("short"))
	assert.Equal(t, "short", w.String())
	assert.False(t, w.truncated)
}

// TestSSHTransport_Run_CapsHugeOutput: an exec returning more than the cap is
// truncated before it reaches the parsers (guards against OOM on a hostile or
// runaway keyspace).
func TestSSHTransport_Run_CapsHugeOutput(t *testing.T) {
	tr := NewSSH("pi@fire", WithMaxOutput(64))
	tr.exec = func(context.Context, string, ...string) ([]byte, []byte, int, error) {
		return []byte(strings.Repeat("A", 1<<20)), nil, 0, nil
	}
	res, err := tr.Run(context.Background(), "redis-cli --scan")
	require.NoError(t, err)
	assert.LessOrEqual(t, len(res.Stdout), 64)
}
