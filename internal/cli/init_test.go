package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tienkane/khealth/internal/config"
	"github.com/tienkane/khealth/internal/discover"
)

// loadGenerated writes generated YAML to a temp file and loads it through the
// real config loader, so the generator can't drift from what khealth accepts.
func loadGenerated(t *testing.T, content string) *config.Config {
	t.Helper()
	p := filepath.Join(t.TempDir(), "khealth.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("generated config does not load: %v\n---\n%s", err, content)
	}
	return cfg
}

func TestGenerateFromDiscovery(t *testing.T) {
	d := discover.Result{
		Ports: []discover.Port{
			{Port: 6379, Proc: "redis-server"},
			{Port: 5432, Proc: "postgres"},
			{Port: 5173, Proc: "node"},
		},
		Docker: []string{"app-cache"},
		PM2:    []string{"worker"},
	}
	cfg := loadGenerated(t, generateConfig(d))

	byName := map[string]string{} // name -> type
	for _, c := range cfg.Checks {
		byName[c.Name] = c.Type
	}
	want := map[string]string{
		"redis":     "redis",
		"postgres":  "tcp",
		"node-5173": "port",
		"app-cache": "docker",
		"worker":    "pm2",
	}
	for name, typ := range want {
		if got := byName[name]; got != typ {
			t.Errorf("check %q: type %q, want %q (all: %v)", name, got, typ, byName)
		}
	}
}

func TestGenerateUniqueNames(t *testing.T) {
	// Two non-special ports owned by the same runtime must not collide, and a
	// container sharing a name with a port check must be disambiguated.
	d := discover.Result{
		Ports:  []discover.Port{{Port: 3000, Proc: "node"}, {Port: 3001, Proc: "node"}},
		Docker: []string{"node-3000"},
	}
	cfg := loadGenerated(t, generateConfig(d)) // Load rejects duplicate names
	if len(cfg.Checks) != 3 {
		t.Fatalf("got %d checks, want 3", len(cfg.Checks))
	}
	seen := map[string]bool{}
	for _, c := range cfg.Checks {
		if seen[c.Name] {
			t.Errorf("duplicate name %q", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestGenerateEmptyFallsBackToScaffold(t *testing.T) {
	out := generateConfig(discover.Result{})
	if out != scaffold {
		t.Error("empty discovery should produce the static scaffold")
	}
	loadGenerated(t, out) // scaffold must itself be valid
}

func TestProcLabel(t *testing.T) {
	cases := map[string]string{
		"node":          "node",
		"Python3":       "python3",
		"redis-server":  "redis-server",
		"com.docker.x":  "comdockerx",
		"  weird name ": "weirdname",
		"--":            "",
	}
	for in, want := range cases {
		if got := procLabel(in); got != want {
			t.Errorf("procLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScaffoldHasEveryType(t *testing.T) {
	for _, typ := range []string{"http", "tcp", "port", "process", "command", "redis", "postgres", "docker", "pm2"} {
		if !strings.Contains(scaffold, "type: "+typ) {
			t.Errorf("scaffold missing type %q", typ)
		}
	}
}
