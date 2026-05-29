package render

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	err := JSON(&buf, map[string]any{"name": "spa", "count": 3})
	require.NoError(t, err)

	// Output must be valid JSON and end with a newline (pipe-friendly).
	var back map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &back))
	assert.Equal(t, "spa", back["name"])
	assert.Equal(t, "\n", buf.String()[buf.Len()-1:])
}

func TestTable_PlainIsAnsiFree(t *testing.T) {
	var buf bytes.Buffer
	err := Table(&buf, []string{"name", "ip"}, [][]string{
		{"Example Hot Tub", "192.0.2.20"},
		{"Example Phone", "192.0.2.10"},
	}, false)
	require.NoError(t, err)

	out := buf.String()
	// Headers are upcased; values present; no ANSI escape sequences.
	assert.Contains(t, out, "NAME")
	assert.Contains(t, out, "IP")
	assert.Contains(t, out, "Example Hot Tub")
	assert.Contains(t, out, "192.0.2.10")
	assert.NotContains(t, out, "\x1b[", "plain table must not contain ANSI color codes")
}

func TestTable_PlainGolden(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, Table(&buf, []string{"name", "ip"}, [][]string{
		{"Example Hot Tub", "192.0.2.20"},
		{"Example Phone", "192.0.2.10"},
	}, false))
	assertGolden(t, "table_plain", buf.String())
}

func TestColorEnabled(t *testing.T) {
	// A bytes.Buffer is not a terminal.
	assert.False(t, ColorEnabled(&bytes.Buffer{}, false))

	// Explicit --no-color wins even for a real file.
	assert.False(t, ColorEnabled(os.Stdout, true))

	// NO_COLOR env disables color.
	t.Setenv("NO_COLOR", "1")
	assert.False(t, ColorEnabled(os.Stdout, false))
}
