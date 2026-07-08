package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In tests the writers are buffers (not a TTY), so `fire tui` must refuse to
// launch rather than hang on a non-interactive terminal.
func TestTUI_NonInteractiveErrors(t *testing.T) {
	_, _, err := exec(t, &fakeClient{}, "tui")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interactive terminal")
}

// Bare `fire` with no TTY falls back to printing help instead of the dashboard.
func TestRoot_BareNonInteractivePrintsHelp(t *testing.T) {
	out, _, err := exec(t, &fakeClient{})
	require.NoError(t, err)
	assert.Contains(t, out, "Available Commands")
	assert.Contains(t, out, "tui")
}
