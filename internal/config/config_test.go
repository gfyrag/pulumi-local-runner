package config

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestCacheDirDefault(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := CacheDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".cache", "plr", "repos")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
