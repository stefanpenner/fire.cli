package firewalla

import (
	"encoding/json"
	"strings"
	"testing"
)

// adversarialPayloads are nasty inputs no parser may panic on. None of these
// parsers control the bytes they receive — they come from redis-cli/Zeek log
// output over SSH — so every one of them is a hostile-input boundary.
var adversarialPayloads = []struct {
	name string
	s    string
}{
	{"empty", ""},
	{"bare newline", "\n"},
	{"crlf", "\r\n"},
	{"lone open brace", "{"},
	{"lone close brace", "}"},
	{"ansi escape sequence", "\x1b[31mkey\x1b[0m\nval\n"},
	{"embedded NUL bytes", "key\x00mid\nval\x00ue"},
	{"other control chars", "\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x1b"},
	{"truncated json object", `{"a":"b","c":[1,2,`},
	{"truncated json array", `key` + "\n" + `[{"a":1},{"b":`},
	{"deeply nested json", strings.Repeat(`{"a":`, 20000) + "1" + strings.Repeat("}", 20000)},
	{"huge single field", "key\n" + strings.Repeat("x", 4*1024*1024)},
	{"non-utf8 bytes", "\xff\xfe\xfd\xfc"},
	{"huge integer string", strings.Repeat("9", 4000)},
	{"huge float string", strings.Repeat("9", 2000) + "." + strings.Repeat("9", 2000)},
	{"odd trailing key", "key1\nval1\ntrailingkey"},
	{"marker soup", "@@DEVICE@@ @@ALARM@@ @@RULE@@ @@DL@@ @@UL@@ @@PLAN@@ @@WANU@@"},
	{"mostly whitespace", "   \t\t\n   \n\r\n  "},
}

// TestParsers_NeverPanic feeds every pure parser in this package the full
// adversarial table. A panic fails the (named) subtest instead of silently
// crashing whatever command is running when the real bytes show up.
func TestParsers_NeverPanic(t *testing.T) {
	for _, p := range adversarialPayloads {
		p := p
		t.Run(p.name, func(t *testing.T) {
			mustNotPanic(t, "parseRedisHash", func() { parseRedisHash(p.s) })
			mustNotPanic(t, "parseDevices", func() { parseDevices(p.s) })
			mustNotPanic(t, "deviceFromHash", func() { deviceFromHash(parseRedisHash(p.s)) })
			mustNotPanic(t, "parseDNSFlows", func() { _, _ = parseDNSFlows(p.s) })
			mustNotPanic(t, "parseResolvers", func() { parseResolvers(p.s, "example.com") })
			mustNotPanic(t, "parseUnixFloat", func() { parseUnixFloat(p.s) })
			mustNotPanic(t, "parseTraffic", func() { parseTraffic(p.s) })
			mustNotPanic(t, "parseScoredMembers", func() { parseScoredMembers(p.s) })
			mustNotPanic(t, "parseNetworks", func() { parseNetworks(p.s) })
			mustNotPanic(t, "parseWANs", func() { _, _ = parseWANs(p.s) })
			mustNotPanic(t, "parseDataUsage", func() { parseDataUsage(p.s) })
			mustNotPanic(t, "parseRules", func() { parseRules(p.s) })
			mustNotPanic(t, "parseAlarms", func() { parseAlarms(p.s) })
			mustNotPanic(t, "parseFeatures", func() { parseFeatures(p.s) })
			mustNotPanic(t, "featureEnabled", func() { featureEnabled(p.s) })
			mustNotPanic(t, "parseTopTalkers", func() { parseTopTalkers(p.s) })
		})
	}
}

// TestFeatureEnabled_DeeplyNestedJSON pins down what the task calls out
// specifically: a policy:system value that is valid-but-absurdly-nested JSON
// must not crash the process. encoding/json itself caps object/array nesting
// at 10000 levels (see encoding/json's internal maxNestingDepth), so
// Unmarshal returns an error well before featureEnabled's json.Unmarshal
// calls would recurse dangerously — anyTrue only ever walks an
// already-decoded value, so its own recursion depth is bounded by that same
// cap. This test proves that empirically rather than asserting it blind.
func TestFeatureEnabled_DeeplyNestedJSON(t *testing.T) {
	const depth = 20000
	nested := strings.Repeat(`{"a":`, depth) + "true" + strings.Repeat("}", depth)

	// Confirm the premise: encoding/json itself refuses this before we ever
	// reach anyTrue.
	var v any
	if err := json.Unmarshal([]byte(nested), &v); err == nil {
		t.Fatalf("expected encoding/json to reject %d levels of nesting, got no error", depth)
	}

	mustNotPanic(t, "featureEnabled(deeply-nested)", func() {
		if featureEnabled(nested) {
			t.Fatalf("unparsable deeply-nested value must not be reported enabled")
		}
	})

	// Also drive it through parseFeatures, as a policy:system hash value.
	hash := "adblock\n" + nested
	mustNotPanic(t, "parseFeatures(deeply-nested)", func() { parseFeatures(hash) })
}

// mustNotPanic runs fn and fails the test (naming which parser) if it panics,
// rather than letting the panic crash the whole test binary anonymously.
func mustNotPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked on payload: %v", name, r)
		}
	}()
	fn()
}
