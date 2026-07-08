package transport

import (
	"strings"
	"testing"
)

// isControlSocketFailure classifies ssh's stderr -- bytes we do not produce, from
// a program whose wording changes between OpenSSH releases and locales. It gates
// a RETRY, so a false positive silently doubles the wait on a genuinely down box,
// and a false negative leaves the tool broken exactly as it was.
//
// Seed corpus is real OpenSSH output, captured from the failure this fixed.
func FuzzIsControlSocketFailure(f *testing.F) {
	seeds := []string{
		"",
		`unix_listener: path "/var/folders/0s/_r7rjfvn0hg408gqkppxhn7m0000gn/T/fire-ssh/cm-635187b9da0b07ae650eca1f4ff4b970819de540.WB0T6e5xe66frwI9" too long for Unix domain socket`,
		"ControlSocket /tmp/fire-ssh/cm-abc already exists, disabling multiplexing",
		"mux_client_hello_exchange: write packet: Broken pipe",
		"ssh: connect to host fire.walla port 22: No route to host",
		"ssh: connect to host fire.walla port 22: Operation timed out",
		"Permission denied (publickey,password).",
		"Host key verification failed.",
		"kex_exchange_identification: read: Connection reset by peer",
		"bind: Invalid argument",
		strings.Repeat("A", 1<<16),  // absurdly long line
		"\x00\x00\x00\xff\xfe",      // NULs and invalid UTF-8
		"unix_listener\n\r\x1b[31m", // escape sequences
	}
	for _, s := range seeds {
		for _, code := range []int{0, 1, 255, -1} {
			f.Add(code, s)
		}
	}

	f.Fuzz(func(t *testing.T, code int, stderr string) {
		got := isControlSocketFailure(code, stderr) // must never panic

		// Invariant 1: only ssh's own transport failures (255) may trigger a retry.
		// A non-255 exit is the REMOTE command's status; retrying would re-run it.
		if code != 255 && got {
			t.Fatalf("retry proposed for non-255 exit %d: %q", code, stderr)
		}

		// Invariant 2: classification depends on nothing but the stderr text, so a
		// 255 verdict must be reproducible.
		if got != isControlSocketFailure(code, stderr) {
			t.Fatal("classification is not deterministic")
		}
	})
}
