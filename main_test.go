package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRun_OK: a clean execution returns 0 and writes nothing to stderr.
func TestRun_OK(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(&out, &errBuf, func() error { return nil })
	assert.Equal(t, 0, code)
	assert.Empty(t, errBuf.String())
}

// TestRun_Error: a returned error becomes exit 1 with an "error:" line.
func TestRun_Error(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(&out, &errBuf, func() error { return errors.New("ssh failed") })
	assert.Equal(t, 1, code)
	assert.Contains(t, errBuf.String(), "ssh failed")
}

// TestRun_ContainsPanic proves the boundary degrades a panic to a clean
// non-zero exit instead of crashing with a stack trace (containment backstop
// for any parser that slips past the fuzzers).
func TestRun_ContainsPanic(t *testing.T) {
	var out, errBuf bytes.Buffer
	var code int
	require.NotPanics(t, func() {
		code = run(&out, &errBuf, func() error { panic("boom") })
	})
	assert.Equal(t, 2, code)
	assert.Contains(t, errBuf.String(), "boom")
}
