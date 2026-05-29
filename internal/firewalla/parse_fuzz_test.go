package firewalla

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// seedFromTestdata adds every fixture file's contents as a fuzz seed, so the
// corpus starts from realistic (already-anonymized) box output.
func seedFromTestdata(f *testing.F) {
	f.Helper()
	matches, _ := filepath.Glob(filepath.Join("testdata", "*"))
	for _, p := range matches {
		if b, err := os.ReadFile(p); err == nil {
			f.Add(string(b))
		}
	}
	// A few degenerate inputs the parsers must also survive.
	for _, s := range []string{"", "\n", "\r\n", "{", "}", "key", "key\nval\nodd"} {
		f.Add(s)
	}
}

// FuzzParseRedisHash: hgetall pairing must never panic and never invent more
// entries than there are line pairs.
func FuzzParseRedisHash(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) {
		m := parseRedisHash(s)
		if len(m) > len(splitLines(s)) {
			t.Fatalf("hash has %d entries from %d lines", len(m), len(splitLines(s)))
		}
	})
}

func FuzzParseDevices(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) {
		_ = parseDevices(s) // must not panic on arbitrary input
	})
}

// FuzzParseDNSFlows must either return a clean slice or a non-nil error — never
// both a result and an error, and never panic.
func FuzzParseDNSFlows(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) {
		out, err := parseDNSFlows(s)
		if err != nil && out != nil {
			t.Fatalf("both result (%d) and error (%v)", len(out), err)
		}
	})
}

// FuzzParseResolvers: output must be sorted by count desc, then client asc —
// the contract the renderer relies on.
func FuzzParseResolvers(f *testing.F) {
	if b, err := os.ReadFile(filepath.Join("testdata", "zeek_dns.log")); err == nil {
		f.Add(string(b), "example.com")
	}
	f.Add("{}", "example.com")
	f.Add("", "")
	f.Add(`{"id.orig_h":"192.0.2.10","query":"a.example.com"}`, "example")
	f.Fuzz(func(t *testing.T, s, domain string) {
		out := parseResolvers(s, domain)
		if !sort.SliceIsSorted(out, func(i, j int) bool {
			if out[i].Count != out[j].Count {
				return out[i].Count > out[j].Count
			}
			return out[i].Client < out[j].Client
		}) {
			t.Fatalf("resolvers not sorted: %+v", out)
		}
	})
}

func FuzzParseRules(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) { _ = parseRules(s) })
}

func FuzzParseFeatures(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) {
		for _, ft := range parseFeatures(s) {
			if ft.Key == "" {
				t.Fatalf("feature with empty key: %+v", ft)
			}
		}
	})
}

func FuzzParseWANs(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) {
		out, err := parseWANs(s)
		if err != nil && out != nil {
			t.Fatalf("both result and error")
		}
	})
}

func FuzzParseTraffic(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) { _ = parseTraffic(s) })
}

func FuzzParseNetworks(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) { _ = parseNetworks(s) })
}

func FuzzParseDataUsage(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) { _ = parseDataUsage(s) })
}

func FuzzParseAlarms(f *testing.F) {
	seedFromTestdata(f)
	f.Fuzz(func(t *testing.T, s string) { _ = parseAlarms(s) })
}

// FuzzParseUnixFloat: never panics, and empty/garbage yields the zero time.
func FuzzParseUnixFloat(f *testing.F) {
	for _, s := range []string{"", "0", "1700000000", "1700000000.5", "-1", "1e9", "nan", "  "} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got := parseUnixFloat(s)
		if strings.TrimSpace(s) == "" && !got.IsZero() {
			t.Fatalf("blank input produced non-zero time %v", got)
		}
	})
}
