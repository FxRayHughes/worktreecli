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

// DeleteBranch 强制删除本地分支（如果存在）
// worktree 尚未 remove 前分支处于被占用状态，删不掉；因此本函数应在 worktree remove 完成后调用
// branch 不存在或分支和当前 HEAD 相同都会静默跳过
func (r *Repo) DeleteBranch(branch string) error {
	if branch == "" {
		return nil
	}
	// 判断分支是否存在
	if err := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run(); err != nil {
		return nil
	}
	// 判断是否为当前 HEAD 所在分支（当前分支不可删）
	if head, err := exec.Command("git", "-C", r.Root, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		if strings.TrimSpace(string(head)) == branch {
			return fmt.Errorf("分支 %s 是主仓库当前 HEAD，跳过删除", branch)
		}
	}
	cmd := exec.Command("git", "-C", r.Root, "branch", "-D", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("删除分支 %s 失败: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
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

// FetchBranch 强制把本地分支 branch 更新到远程最新
// 如果 branch 是远程分支「origin/main」，直接返回（worktree add 时用远程 ref 即可）
// 如果 branch 是本地分支，且存在对应远程跟踪分支「origin/<branch>」，
// 用 `git fetch origin <branch>:<branch>` 强制更新本地引用
// 若本地分支已 check out 到主 worktree，fetch 到分支的方式会失败，回退到 fast-forward 拉取
func (r *Repo) FetchBranch(branch string) error {
	// 远程分支不需要更新，worktree add 会直接用它
	if strings.Contains(branch, "/") {
		return nil
	}
	// 先看远程有没有对应分支
	remoteRef := "refs/remotes/origin/" + branch
	if err := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", remoteRef).Run(); err != nil {
		// 没有远程对应分支 → 什么都不做，本地分支为准
		return nil
	}
	// 尝试 fetch <branch>:<branch> —— 强制把本地引用推到远程最新
	// 若本地分支正被某个 worktree checkout，该操作会失败，回退到 pull --ff-only
	cmd := exec.Command("git", "-C", r.Root, "fetch", "origin", branch+":"+branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		// 分支被占用，回退：直接把远程分支的 sha 用于 worktree add
		// 这里返回一个特定错误让调用方感知，但不当作致命
		return fmt.Errorf("fetch %s 失败「本地分支可能被占用，将使用远程 origin/%s」: %s",
			branch, branch, strings.TrimSpace(string(out)))
	}
	return nil
}
