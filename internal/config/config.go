package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Stack struct {
	Name      string   `yaml:"name"`
	Branch    string   `yaml:"branch,omitempty"`
	Ref       string   `yaml:"ref,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty"`
	Org       string   `yaml:"org,omitempty"`
	Project   string   `yaml:"project,omitempty"`
}

type App struct {
	Name   string  `yaml:"name"`
	Repo   string  `yaml:"repo"`
	Path   string  `yaml:"path,omitempty"`
	Stacks []Stack `yaml:"stacks"`
}

type Config struct {
	Apps []App `yaml:"apps"`
}

func ConfigDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "plr"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "plr"), nil
}

func CacheDir() (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "plr"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "plr", "repos"), nil
}

func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	for i := range cfg.Apps {
		if cfg.Apps[i].Path == "" {
			cfg.Apps[i].Path = "."
		}
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, data, 0o644)
}

// Validate checks the config for structural errors.
func (c *Config) Validate() error {
	appNames := make(map[string]bool)
	allStacks := make(map[string]bool)

	for i, app := range c.Apps {
		if app.Name == "" {
			return fmt.Errorf("apps[%d]: name is required", i)
		}
		if app.Repo == "" {
			return fmt.Errorf("app %q: repo is required", app.Name)
		}
		if appNames[app.Name] {
			return fmt.Errorf("duplicate app name %q", app.Name)
		}
		appNames[app.Name] = true

		if len(app.Stacks) == 0 {
			return fmt.Errorf("app %q: at least one stack is required", app.Name)
		}

		stackNames := make(map[string]bool)
		for j, stack := range app.Stacks {
			if stack.Name == "" {
				return fmt.Errorf("app %q stacks[%d]: name is required", app.Name, j)
			}
			if stackNames[stack.Name] {
				return fmt.Errorf("app %q: duplicate stack name %q", app.Name, stack.Name)
			}
			if stack.Branch != "" && stack.Ref != "" {
				return fmt.Errorf("app %q stack %q: branch and ref are mutually exclusive", app.Name, stack.Name)
			}
			stackNames[stack.Name] = true
			allStacks[app.Name+"/"+stack.Name] = true
		}
	}

	// Validate dependsOn references
	for _, app := range c.Apps {
		for _, stack := range app.Stacks {
			for _, dep := range stack.DependsOn {
				if !allStacks[dep] {
					return fmt.Errorf("app %q stack %q: dependency %q not found", app.Name, stack.Name, dep)
				}
				if dep == app.Name+"/"+stack.Name {
					return fmt.Errorf("app %q stack %q: cannot depend on itself", app.Name, stack.Name)
				}
			}
		}
	}

	return nil
}

// FindApp returns the app with the given name, or an error if not found.
func (c *Config) FindApp(name string) (*App, error) {
	for i := range c.Apps {
		if c.Apps[i].Name == name {
			return &c.Apps[i], nil
		}
	}
	return nil, fmt.Errorf("app %q not found", name)
}

// FindStack returns the stack for the given app, or an error if not found.
func (a *App) FindStack(name string) (*Stack, error) {
	for i := range a.Stacks {
		if a.Stacks[i].Name == name {
			return &a.Stacks[i], nil
		}
	}
	return nil, fmt.Errorf("stack %q not found in app %q", name, a.Name)
}
