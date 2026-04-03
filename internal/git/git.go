package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gfyrag/plr/internal/config"
	"github.com/gfyrag/plr/internal/store"
	"github.com/gfyrag/plr/internal/ui"
)

// RepoDir returns the local cache directory for the given app.
func RepoDir(app *config.App) (string, error) {
	cacheDir, err := config.CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, app.Name), nil
}

// WorkDir returns the working directory (repo + path) for the given app.
func WorkDir(app *config.App) (string, error) {
	repoDir, err := RepoDir(app)
	if err != nil {
		return "", err
	}
	return filepath.Join(repoDir, app.Path), nil
}

// Sync clones the repo if it doesn't exist, fetches updates, and checks out the given ref.
func Sync(s store.Store, app *config.App, stack *config.Stack) error {
	repoDir, err := RepoDir(app)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		ui.Info("Cloning %s...", app.Repo)
		if err := runGit("clone", app.Repo, repoDir); err != nil {
			return fmt.Errorf("cloning %s: %w", app.Repo, err)
		}
	} else {
		ui.Step("Fetching updates...")
		if err := runGit("-C", repoDir, "fetch", "--all", "--tags", "--prune"); err != nil {
			return fmt.Errorf("fetching %s: %w", app.Repo, err)
		}
	}

	if err := checkout(repoDir, stack); err != nil {
		return fmt.Errorf("checking out ref for stack %q: %w", stack.Name, err)
	}

	// Restore stack config from plr config store into workdir
	if err := restoreStackConfig(s, app, stack); err != nil {
		return fmt.Errorf("restoring stack config: %w", err)
	}

	return nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w", args[0], err)
	}
	return nil
}

func checkout(repoDir string, stack *config.Stack) error {
	ref := stack.Branch
	if stack.Ref != "" {
		ref = stack.Ref
	}
	if ref == "" {
		return nil
	}

	target := ref
	if stack.Branch != "" {
		target = "origin/" + ref
	}

	// Reset any local modifications (e.g. Pulumi config files from previous runs)
	// so that checkout succeeds cleanly.
	if err := runGit("-C", repoDir, "checkout", "--", "."); err != nil {
		return fmt.Errorf("cleaning workdir: %w", err)
	}

	ui.Step("Checking out %s...", ref)
	return runGit("-C", repoDir, "checkout", "--detach", target)
}

// restoreStackConfig copies the stack config from the store into the workdir.
// If no stored config exists, this is a no-op.
func restoreStackConfig(s store.Store, app *config.App, stack *config.Stack) error {
	data, err := s.ReadStackConfig(app.Name, stack.Name)
	if err != nil {
		return err
	}
	if data == nil {
		return nil
	}

	workDir, err := WorkDir(app)
	if err != nil {
		return err
	}

	dest := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))
	return os.WriteFile(dest, data, 0o644)
}

// SaveStackConfig copies the stack config from the workdir back to the store.
func SaveStackConfig(s store.Store, app *config.App, stack *config.Stack) error {
	workDir, err := WorkDir(app)
	if err != nil {
		return err
	}

	src := filepath.Join(workDir, fmt.Sprintf("Pulumi.%s.yaml", stack.Name))
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return s.WriteStackConfig(app.Name, stack.Name, data)
}
