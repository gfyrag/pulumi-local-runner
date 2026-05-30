package shared

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunGit executes a git command with stdout/stderr connected to the terminal.
func RunGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CloneSourceRepo clones (or fetches) the source repository into a local cache
// directory and optionally checks out the given ref. It returns the path to
// the cloned repo on disk. Local paths (/, ./, ../, ~) are returned as-is.
func CloneSourceRepo(repoURL, ref string) (string, error) {
	if strings.HasPrefix(repoURL, "/") || strings.HasPrefix(repoURL, "./") || strings.HasPrefix(repoURL, "../") || strings.HasPrefix(repoURL, "~") {
		if strings.HasPrefix(repoURL, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("could not determine home directory: %w", err)
			}
			repoURL = filepath.Join(home, repoURL[1:])
		}
		return filepath.Clean(repoURL), nil
	}

	cacheBase, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("could not determine cache directory: %w", err)
	}

	repoName := filepath.Base(strings.TrimSuffix(repoURL, ".git"))
	repoDir := filepath.Join(cacheBase, "plr", "repos", repoName)

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		fmt.Printf("Cloning %s...\n", repoURL)
		if err := RunGit("clone", repoURL, repoDir); err != nil {
			return "", fmt.Errorf("cloning %s: %w", repoURL, err)
		}
	} else {
		fmt.Printf("Fetching updates for %s...\n", repoName)
		if err := RunGit("-C", repoDir, "fetch", "--all", "--tags", "--prune"); err != nil {
			return "", fmt.Errorf("fetching %s: %w", repoURL, err)
		}
	}

	if ref != "" {
		_ = RunGit("-C", repoDir, "checkout", "--", ".")

		if err := RunGit("-C", repoDir, "checkout", "--detach", "origin/"+ref); err != nil {
			if err := RunGit("-C", repoDir, "checkout", "--detach", ref); err != nil {
				return "", fmt.Errorf("checking out %s: %w", ref, err)
			}
		}
	}

	return repoDir, nil
}

// GetBuildVersion generates a version string based on git commit and timestamp.
// Format: <short-commit>-<timestamp> (e.g., "abc1234-20260125-143022").
// If git is not available or the working directory is dirty, the version
// includes a "-dirty" suffix.
func GetBuildVersion(gitDir string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = gitDir
	output, err := cmd.Output()

	timestamp := time.Now().Format("20060102-150405")

	if err != nil {
		return timestamp
	}

	commit := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = gitDir
	statusOutput, _ := cmd.Output()

	if len(statusOutput) > 0 {
		return fmt.Sprintf("%s-dirty-%s", commit, timestamp)
	}

	return fmt.Sprintf("%s-%s", commit, timestamp)
}
