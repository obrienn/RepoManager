package gitops

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type UpdateResult struct {
	PrevFileCount int
	NewFileCount  int
	PrevSize      int64
	NewSize       int64
	DeltaPercent  float64
	PrevHead      string
	Status        string // "ok", "flagged", "error"
	Error         string
}

func Snapshot(ctx context.Context, path string) (fileCount int, size int64, head string, _ error) {
	count, err := FileCount(path)
	if err != nil {
		return 0, 0, "", fmt.Errorf("count: %w", err)
	}
	size, err = DirSize(path)
	if err != nil {
		return 0, 0, "", fmt.Errorf("size: %w", err)
	}
	headCmd := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "HEAD")
	headOut, err := headCmd.Output()
	if err != nil {
		return 0, 0, "", fmt.Errorf("rev-parse: %w", err)
	}
	return count, size, strings.TrimSpace(string(headOut)), nil
}

func Update(ctx context.Context, path string, deltaThreshold float64) *UpdateResult {
	result := &UpdateResult{Status: "error"}

	prevCount, prevSize, prevHead, err := Snapshot(ctx, path)
	if err != nil {
		result.Error = fmt.Sprintf("snapshot: %v", err)
		return result
	}
	result.PrevFileCount = prevCount
	result.PrevSize = prevSize
	result.PrevHead = prevHead

	exec.CommandContext(ctx, "git", "-C", path, "tag", "-f", "repoman-safety", prevHead).Run()

	if err := Pull(ctx, path); err != nil {
		result.Error = fmt.Sprintf("pull: %v", err)
		exec.CommandContext(ctx, "git", "-C", path, "tag", "-d", "repoman-safety").Run()
		return result
	}

	newCount, err := FileCount(path)
	if err != nil {
		result.Error = fmt.Sprintf("post count: %v", err)
		exec.CommandContext(ctx, "git", "-C", path, "tag", "-d", "repoman-safety").Run()
		return result
	}
	newSize, err := DirSize(path)
	if err != nil {
		result.Error = fmt.Sprintf("post size: %v", err)
		exec.CommandContext(ctx, "git", "-C", path, "tag", "-d", "repoman-safety").Run()
		return result
	}
	result.NewFileCount = newCount
	result.NewSize = newSize

	if prevCount > 0 {
		result.DeltaPercent = float64(newCount-prevCount) / float64(prevCount) * 100
	} else if newCount > 0 {
		result.DeltaPercent = 100
	}

	if result.DeltaPercent < -deltaThreshold {
		cmd := exec.CommandContext(ctx, "git", "-C", path, "reset", "--hard", prevHead)
		if out, err := cmd.CombinedOutput(); err != nil {
			result.Error = fmt.Sprintf("revert: %s: %v", string(out), err)
			exec.CommandContext(ctx, "git", "-C", path, "tag", "-d", "repoman-safety").Run()
			return result
		}
		result.Status = "flagged"
	} else {
		result.Status = "ok"
	}

	exec.CommandContext(ctx, "git", "-C", path, "tag", "-d", "repoman-safety").Run()

	return result
}

func CheckRemoteExists(ctx context.Context, url string) bool {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--exit-code", url)
	err := cmd.Run()
	return err == nil
}

func RepoStats(ctx context.Context, path string) (fileCount int, size int64, lastUpdated time.Time, _ error) {
	count, err := FileCount(path)
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	size, err = DirSize(path)
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", path, "log", "-1", "--format=%cI", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	return count, size, t, nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return strconv.FormatInt(b, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
