// Package config loads fire's optional config file: default host, named boxes,
// and default flags. Everything is optional — a missing file yields the zero
// Config and callers fall back to built-in defaults. The file is external
// bytes fire does not produce, so parsing degrades (never panics) on garbage.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Box is a named Firewalla target.
type Box struct {
	Host string `json:"host"`
}

// Config is the on-disk configuration (~/.config/fire/config.json).
type Config struct {
	DefaultBox string         `json:"default_box"`
	Host       string         `json:"host"`     // default host when no boxes are used
	NoColor    bool           `json:"no_color"` // default for --no-color
	Timeout    string         `json:"timeout"`  // default --timeout (a Go duration, e.g. "30s")
	Boxes      map[string]Box `json:"boxes"`
}

// DefaultPath returns the config file location, honoring XDG_CONFIG_HOME.
func DefaultPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "fire", "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "fire", "config.json")
}

// Load reads and parses the config at path. A missing file is not an error (it
// returns the zero Config). A malformed file returns an error so the caller can
// warn and fall back to defaults rather than silently honoring garbage.
func Load(path string) (Config, error) {
	if path == "" {
		return Config{}, nil
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return c, nil
}

// ResolveHost picks the ssh host. Precedence: an explicit box name (if given)
// wins; otherwise the configured default_box; otherwise the bare `host`;
// otherwise fallback. An unknown box name is an error so a typo is not silently
// pointed at the wrong (or default) box.
func (c Config) ResolveHost(box, fallback string) (string, error) {
	name := box
	if name == "" {
		name = c.DefaultBox
	}
	if name != "" {
		b, ok := c.Boxes[name]
		if !ok {
			return "", fmt.Errorf("unknown box %q (known: %s)", name, c.boxNames())
		}
		if b.Host == "" {
			return "", fmt.Errorf("box %q has no host", name)
		}
		return b.Host, nil
	}
	if c.Host != "" {
		return c.Host, nil
	}
	return fallback, nil
}

// TimeoutOr parses the configured timeout, or returns fallback when unset or
// invalid.
func (c Config) TimeoutOr(fallback time.Duration) time.Duration {
	if c.Timeout == "" {
		return fallback
	}
	d, err := time.ParseDuration(c.Timeout)
	if err != nil || d < 0 {
		return fallback
	}
	return d
}

func (c Config) boxNames() string {
	if len(c.Boxes) == 0 {
		return "none"
	}
	names := make([]string, 0, len(c.Boxes))
	for n := range c.Boxes {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
