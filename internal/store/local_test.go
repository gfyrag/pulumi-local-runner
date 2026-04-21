package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gfyrag/plr/internal/config"
)

func TestLocalStoreLoadNonExistent(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Apps) != 0 {
		t.Fatalf("expected empty config, got %d apps", len(cfg.Apps))
	}
}

func TestLocalStoreRoundtrip(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	original := &config.Config{
		Apps: []config.App{
			{
				Name: "myapp",
				Repo: "git@github.com:org/myapp.git",
				Path: "infra",
				Stacks: []config.Stack{
					{Name: "dev", Branch: "main"},
					{Name: "prod", Ref: "v1.0.0"},
				},
			},
		},
	}

	if err := s.SaveConfig(original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(loaded.Apps))
	}
	app := loaded.Apps[0]
	if app.Name != "myapp" {
		t.Errorf("name = %q, want %q", app.Name, "myapp")
	}
	if app.Repo != "git@github.com:org/myapp.git" {
		t.Errorf("repo = %q", app.Repo)
	}
	if app.Path != "infra" {
		t.Errorf("path = %q, want %q", app.Path, "infra")
	}
	if len(app.Stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d", len(app.Stacks))
	}
	if app.Stacks[0].Branch != "main" {
		t.Errorf("stack[0].branch = %q", app.Stacks[0].Branch)
	}
	if app.Stacks[1].Ref != "v1.0.0" {
		t.Errorf("stack[1].ref = %q", app.Stacks[1].Ref)
	}
}

func TestLocalStoreDefaultPath(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	cfg := &config.Config{
		Apps: []config.App{
			{
				Name:   "nopath",
				Repo:   "git@github.com:org/nopath.git",
				Stacks: []config.Stack{{Name: "dev", Branch: "main"}},
			},
		},
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Apps[0].Path != "." {
		t.Errorf("expected default path %q, got %q", ".", loaded.Apps[0].Path)
	}
}

func TestLocalStoreInvalidAppYAML(t *testing.T) {
	dir := t.TempDir()
	s := NewLocalStore(dir)

	appDir := filepath.Join(dir, "apps", "badapp")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "app.yaml"), []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.LoadConfig()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLocalStoreStackConfigRoundtrip(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	// First create the stack file so WriteStackConfig has something to merge into
	cfg := &config.Config{
		Apps: []config.App{
			{Name: "myapp", Repo: "r", Stacks: []config.Stack{{Name: "dev"}}},
		},
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	data := []byte("config:\n  key: value\n")
	if err := s.WriteStackConfig("myapp", "dev", data); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := s.ReadStackConfig("myapp", "dev")
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Verify the config key is present in the output
	if !strings.Contains(string(got), "key: value") {
		t.Errorf("expected config key in output, got %q", got)
	}
}

func TestLocalStoreStackConfigNonExistent(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	data, err := s.ReadStackConfig("myapp", "dev")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data, got %q", data)
	}
}

func TestNewDefaultLocalStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	s, err := NewDefaultLocalStore()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.configDir != filepath.Join(dir, "plr") {
		t.Errorf("configDir = %q, want %q", s.configDir, filepath.Join(dir, "plr"))
	}
}

func TestLocalStoreDeletesRemovedStacks(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	// Save with two stacks
	cfg := &config.Config{
		Apps: []config.App{
			{
				Name: "myapp",
				Repo: "git@github.com:org/myapp.git",
				Stacks: []config.Stack{
					{Name: "dev", Branch: "main"},
					{Name: "prod", Ref: "v1.0.0"},
				},
			},
		},
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Remove one stack and save again
	cfg.Apps[0].Stacks = cfg.Apps[0].Stacks[:1]
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Apps[0].Stacks) != 1 {
		t.Errorf("expected 1 stack, got %d", len(loaded.Apps[0].Stacks))
	}
}

func TestLocalStoreDeletesRemovedApps(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	cfg := &config.Config{
		Apps: []config.App{
			{Name: "app1", Repo: "r1", Stacks: []config.Stack{{Name: "dev"}}},
			{Name: "app2", Repo: "r2", Stacks: []config.Stack{{Name: "dev"}}},
		},
	}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Remove app2
	cfg.Apps = cfg.Apps[:1]
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(loaded.Apps))
	}
}

func TestLocalStoreBasesRoundtrip(t *testing.T) {
	s := NewLocalStore(t.TempDir())

	data := []byte("config:\n  key: value\n")
	if err := s.WriteBaseConfig("mybase", data); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := s.ReadBaseConfig("mybase")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}

	names, err := s.ListBases()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(names) != 1 || names[0] != "mybase" {
		t.Errorf("list = %v, want [mybase]", names)
	}

	if err := s.DeleteBaseConfig("mybase"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, err = s.ReadBaseConfig("mybase")
	if err != nil {
		t.Fatalf("read after delete: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %q", got)
	}
}
