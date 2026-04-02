package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cfg.Apps) != 0 {
		t.Fatalf("expected empty config, got %d apps", len(cfg.Apps))
	}
}

func TestLoadAndSaveRoundtrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	original := &Config{
		Apps: []App{
			{
				Name: "myapp",
				Repo: "git@github.com:org/myapp.git",
				Path: "infra",
				Stacks: []Stack{
					{Name: "dev", Branch: "main"},
					{Name: "prod", Ref: "v1.0.0"},
				},
			},
		},
	}

	if err := Save(original); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load()
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

func TestLoadDefaultPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := &Config{
		Apps: []App{
			{
				Name:   "nopath",
				Repo:   "git@github.com:org/nopath.git",
				Stacks: []Stack{{Name: "dev", Branch: "main"}},
			},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Apps[0].Path != "." {
		t.Errorf("expected default path %q, got %q", ".", loaded.Apps[0].Path)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	path := filepath.Join(dir, "plr", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestFindApp(t *testing.T) {
	cfg := &Config{
		Apps: []App{
			{Name: "a", Repo: "r", Stacks: []Stack{{Name: "s"}}},
			{Name: "b", Repo: "r", Stacks: []Stack{{Name: "s"}}},
		},
	}

	app, err := cfg.FindApp("b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.Name != "b" {
		t.Errorf("got %q", app.Name)
	}

	_, err = cfg.FindApp("nope")
	if err == nil {
		t.Fatal("expected error for missing app")
	}
}

func TestFindStack(t *testing.T) {
	app := &App{
		Name: "myapp",
		Stacks: []Stack{
			{Name: "dev"},
			{Name: "prod"},
		},
	}

	s, err := app.FindStack("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "prod" {
		t.Errorf("got %q", s.Name)
	}

	_, err = app.FindStack("staging")
	if err == nil {
		t.Fatal("expected error for missing stack")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev", Branch: "main"}}},
			}},
		},
		{
			name: "valid with deps",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev", Branch: "main"}}},
				{Name: "b", Repo: "r", Stacks: []Stack{{Name: "dev", Branch: "main", DependsOn: []string{"a/dev"}}}},
			}},
		},
		{
			name:    "empty app name",
			cfg:     Config{Apps: []App{{Repo: "r", Stacks: []Stack{{Name: "dev"}}}}},
			wantErr: true,
		},
		{
			name:    "empty repo",
			cfg:     Config{Apps: []App{{Name: "a", Stacks: []Stack{{Name: "dev"}}}}},
			wantErr: true,
		},
		{
			name:    "no stacks",
			cfg:     Config{Apps: []App{{Name: "a", Repo: "r"}}},
			wantErr: true,
		},
		{
			name:    "empty stack name",
			cfg:     Config{Apps: []App{{Name: "a", Repo: "r", Stacks: []Stack{{Branch: "main"}}}}},
			wantErr: true,
		},
		{
			name: "duplicate app",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev"}}},
				{Name: "a", Repo: "r2", Stacks: []Stack{{Name: "prod"}}},
			}},
			wantErr: true,
		},
		{
			name: "duplicate stack",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev"}, {Name: "dev"}}},
			}},
			wantErr: true,
		},
		{
			name: "branch and ref both set",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev", Branch: "main", Ref: "v1"}}},
			}},
			wantErr: true,
		},
		{
			name: "invalid dependency",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev", DependsOn: []string{"nope/nope"}}}},
			}},
			wantErr: true,
		},
		{
			name: "self dependency",
			cfg: Config{Apps: []App{
				{Name: "a", Repo: "r", Stacks: []Stack{{Name: "dev", DependsOn: []string{"a/dev"}}}},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigDirXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "plr") {
		t.Errorf("got %q, want %q", got, filepath.Join(dir, "plr"))
	}
}

func TestCacheDirXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	got, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "plr") {
		t.Errorf("got %q, want %q", got, filepath.Join(dir, "plr"))
	}
}
