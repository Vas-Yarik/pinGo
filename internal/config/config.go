package config

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultTimeout   = 5 * time.Second
	DefaultInterval  = 30 * time.Second
	MinInterval      = time.Second
)

type Config struct {
	Targets  []Target      `yaml:"targets"`
	Interval time.Duration `yaml:"interval"`
}

type Target struct {
	Name    string        `yaml:"name"`
	URL     string        `yaml:"url"`
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Interval == 0 {
		cfg.Interval = DefaultInterval
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (t *Target) ResolvedTimeout() time.Duration {
	if t.Timeout > 0 {
		return t.Timeout
	}
	return DefaultTimeout
}

func (c *Config) validate() error {
	if len(c.Targets) == 0 {
		return fmt.Errorf("config: at least one target is required")
	}
	if c.Interval < MinInterval {
		return fmt.Errorf("config: interval must be at least %s, got %s", MinInterval, c.Interval)
	}
	seen := make(map[string]struct{}, len(c.Targets))
	for i := range c.Targets {
		t := &c.Targets[i]

		if t.Name == "" {
			return fmt.Errorf("config: target #%d: name is required", i+1)
		}
		if _, ok := seen[t.Name]; ok {
			return fmt.Errorf("config: duplicate target name %q", t.Name)
		}
		seen[t.Name] = struct{}{}

		u, err := url.Parse(t.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("config: target %q: url must be an absolute http(s) URL, got %q", t.Name, t.URL)
		}
		if t.Timeout < 0 {
			return fmt.Errorf("config: target %q: timeout must not be negative", t.Name)
		}
	}
	return nil
}
