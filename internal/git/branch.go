package git

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Branch 分支引用
type Branch struct {
	Name   string // 短名，例如 main 或 origin/main
	Remote bool   // 是否远程分支
}

// ListBranches 返回本地 + 远程分支（去重）
func (r *Repo) ListBranches() ([]Branch, error) {
	cmd := exec.Command("git", "-C", r.Root, "for-each-ref",
		"--format=%(refname:short)|%(refname)",
		"refs/heads", "refs/remotes")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("列分支失败: %w", err)
	}
	var branches []Branch
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		name := parts[0]
		full := parts[1]
		if strings.HasSuffix(name, "/HEAD") {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		branches = append(branches, Branch{
			Name:   name,
			Remote: strings.HasPrefix(full, "refs/remotes/"),
		})
	}
	sort.Slice(branches, func(i, j int) bool {
		if branches[i].Remote != branches[j].Remote {
			return !branches[i].Remote // 本地在前
		}
		return branches[i].Name < branches[j].Name
	})
	return branches, nil
}

// Fetch 拉取所有远程
func (r *Repo) Fetch() error {
	cmd := exec.Command("git", "-C", r.Root, "fetch", "--all", "--prune")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git fetch 失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
