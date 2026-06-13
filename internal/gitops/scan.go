package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type DiscoveredRepo struct {
	Name      string
	Owner     string
	GithubURL string
	LocalPath string
}

func ScanDirectory(root string) ([]DiscoveredRepo, error) {
	var repos []DiscoveredRepo

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		repoPath := filepath.Join(root, entry.Name())

		if isGitRepo(repoPath) {
			remote, err := getRemoteURL(repoPath)
			if err != nil || remote == "" {
				continue
			}
			owner, name := parseGitHubRemote(remote)
			if owner == "" || name == "" {
				continue
			}
			repos = append(repos, DiscoveredRepo{
				Name:      name,
				Owner:     owner,
				GithubURL: "https://github.com/" + owner + "/" + name,
				LocalPath: repoPath,
			})
			continue
		}

		subEntries, err := os.ReadDir(repoPath)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			subPath := filepath.Join(repoPath, sub.Name())
			if isGitRepo(subPath) {
				remote, err := getRemoteURL(subPath)
				if err != nil || remote == "" {
					continue
				}
				owner, name := parseGitHubRemote(remote)
				if owner == "" || name == "" {
					continue
				}
				if owner != entry.Name() && entry.Name() != "." {
					continue
				}
				repos = append(repos, DiscoveredRepo{
					Name:      name,
					Owner:     owner,
					GithubURL: "https://github.com/" + owner + "/" + name,
					LocalPath: subPath,
				})
			}
		}
	}

	return repos, nil
}

func isGitRepo(path string) bool {
	info, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func getRemoteURL(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseGitHubRemote(remote string) (owner, name string) {
	remote = strings.TrimSuffix(remote, ".git")
	if strings.Contains(remote, "github.com") {
		parts := strings.Split(remote, "github.com")
		if len(parts) < 2 {
			return "", ""
		}
		segments := strings.Split(strings.Trim(parts[1], "/:"), "/")
		if len(segments) >= 2 {
			return segments[0], segments[1]
		}
	} else if strings.Contains(remote, "github.com:") {
		parts := strings.SplitN(remote, ":", 2)
		if len(parts) == 2 {
			segments := strings.Split(strings.TrimSuffix(parts[1], ".git"), "/")
			if len(segments) >= 2 {
				return segments[0], segments[1]
			}
		}
	}
	return "", ""
}
