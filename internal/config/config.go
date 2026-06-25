// Package config loads a khealth.yaml file into a list of check specs.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tienkane/khealth/internal/check"
	"gopkg.in/yaml.v3"
)

// Config is the parsed khealth.yaml.
type Config struct {
	Checks []check.Spec `yaml:"checks"`
}

// Filenames searched for, in order, when no explicit path is given.
var Filenames = []string{"khealth.yaml", "khealth.yml", ".khealth.yaml", ".khealth.yml"}

// ErrNotFound is returned by Find when no config file exists.
var ErrNotFound = errors.New("no khealth config found")

// Find locates a config file by walking up from the current directory, then
// falling back to $XDG_CONFIG_HOME/khealth and ~/.config/khealth.
func Find() (string, error) {
	dir, err := os.Getwd()
	if err == nil {
		for {
			for _, name := range Filenames {
				p := filepath.Join(dir, name)
				if fileExists(p) {
					return p, nil
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, base := range configDirs() {
		for _, name := range Filenames {
			p := filepath.Join(base, "khealth", name)
			if fileExists(p) {
				return p, nil
			}
		}
	}
	return "", ErrNotFound
}

func configDirs() []string {
	var dirs []string
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		dirs = append(dirs, x)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config"))
	}
	return dirs
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// Load reads and validates the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if len(c.Checks) == 0 {
		return errors.New("no checks defined")
	}
	valid := map[string]bool{}
	for _, t := range check.Types() {
		valid[t] = true
	}
	seen := map[string]bool{}
	for i := range c.Checks {
		s := &c.Checks[i]
		if s.Name == "" {
			return fmt.Errorf("check %d: missing name", i+1)
		}
		if s.Type == "" {
			return fmt.Errorf("check %q: missing type", s.Name)
		}
		if !valid[s.Type] {
			return fmt.Errorf("check %q: unknown type %q", s.Name, s.Type)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate check name %q", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}
