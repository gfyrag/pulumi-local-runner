package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Stack struct {
	Name      string   `yaml:"name"`
	Env       string   `yaml:"env,omitempty"`
	Branch    string   `yaml:"branch,omitempty"`
	Ref       string   `yaml:"ref,omitempty"`
	DependsOn []string `yaml:"dependsOn,omitempty"`
	Org       string   `yaml:"org,omitempty"`
	Project   string   `yaml:"project,omitempty"`
	Bases     []string `yaml:"bases,omitempty"`
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

// AppFile is the on-disk representation of an app (app.yaml).
// The app name is derived from the directory name.
type AppFile struct {
	Repo string `yaml:"repo"`
	Path string `yaml:"path,omitempty"`
}

// StackFile is the on-disk representation of a stack (<stack>.yaml).
// The stack name is derived from the file name.
// It combines the stack definition and Pulumi config in a single file.
type StackFile struct {
	Env       string         `yaml:"env,omitempty"`
	Branch    string         `yaml:"branch,omitempty"`
	Ref       string         `yaml:"ref,omitempty"`
	DependsOn []string       `yaml:"dependsOn,omitempty"`
	Org       string         `yaml:"org,omitempty"`
	Project   string         `yaml:"project,omitempty"`
	Bases     []string       `yaml:"bases,omitempty"`
	Config    map[string]any `yaml:"config,omitempty"`
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
