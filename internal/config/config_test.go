package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadValid(t *testing.T) {
	const content = `
targets:
  - name: example
    url: https://example.com
  - name: google
    url: https://www.google.com
    timeout: 2s
`
	cfg, err := Load(writeConfig(t, content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(cfg.Targets))
	}

	first, second := cfg.Targets[0], cfg.Targets[1]
	if first.Name != "example" || first.URL != "https://example.com" {
		t.Errorf("unexpected first target: %+v", first)
	}
	if second.Timeout != 2*time.Second {
		t.Errorf("want timeout 2s, got %v", second.Timeout)
	}
	if got := first.ResolvedTimeout(); got != DefaultTimeout {
		t.Errorf("want default timeout %v, got %v", DefaultTimeout, got)
	}
	if cfg.Interval != DefaultInterval {
		t.Errorf("want default interval %v, got %v", DefaultInterval, cfg.Interval)
	}
}

func TestLoadInterval(t *testing.T) {
	const content = `
interval: 2s
targets:
  - name: example
    url: https://example.com
`
	cfg, err := Load(writeConfig(t, content))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Interval != 2*time.Second {
		t.Errorf("want interval 2s, got %v", cfg.Interval)
	}
}

func TestLoadInvalidInterval(t *testing.T) {
	cases := map[string]string{
		"too small":    "interval: 500ms\n",
		"negative":     "interval: -1s\n",
		"unparsable":   "interval: soon\n",
	}
	for name, interval := range cases {
		t.Run(name, func(t *testing.T) {
			content := "targets:\n  - name: a\n    url: https://example.com\n" + interval
			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	if _, err := Load(writeConfig(t, "targets: [")); err == nil {
		t.Fatal("want error for invalid YAML")
	}
}

func TestLoadNoTargets(t *testing.T) {
	if _, err := Load(writeConfig(t, "targets: []\n")); err == nil {
		t.Fatal("want error for empty target list")
	}
}

func TestLoadInvalidTarget(t *testing.T) {
	cases := map[string]string{
		"missing name":       "targets:\n  - url: https://example.com\n",
		"relative url":       "targets:\n  - name: a\n    url: /index.html\n",
		"unsupported scheme": "targets:\n  - name: a\n    url: ftp://example.com\n",
		"negative timeout":   "targets:\n  - name: a\n    url: https://example.com\n    timeout: -1s\n",
		"unparsable timeout": "targets:\n  - name: a\n    url: https://example.com\n    timeout: later\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, content)); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestLoadDuplicateNames(t *testing.T) {
	const content = `
targets:
  - name: a
    url: https://example.com
  - name: a
    url: https://www.google.com
`
	if _, err := Load(writeConfig(t, content)); err == nil {
		t.Fatal("want error for duplicate target names")
	}
}
