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

// CheckoutAndPull 把主仓库切到 branch 并 pull 最新
// 目标场景：用户在基线分支页按 c，主动切换 wtc 所在目录到目标分支并拉最新
// 该操作会破坏 wtc 所在目录当前分支的工作状态（若有未提交更改会失败）
// 若 branch 是远程分支「origin/xxx」，先建立本地跟踪分支再 checkout
func (r *Repo) CheckoutAndPull(branch string) error {
	if strings.Contains(branch, "/") {
		// 远程分支：切一个同名的本地跟踪分支
		parts := strings.SplitN(branch, "/", 2)
		localName := parts[1]
		// 若本地已有同名分支则直接 checkout
		if err := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+localName).Run(); err == nil {
			branch = localName
		} else {
			// 建立跟踪分支
			cmd := exec.Command("git", "-C", r.Root, "checkout", "-b", localName, "--track", branch)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("checkout -b %s --track %s 失败: %s", localName, branch, strings.TrimSpace(string(out)))
			}
			// 切完就已经是最新（远程 ref 里的 sha）
			return nil
		}
	}
	// 本地分支：checkout + pull --ff-only
	if out, err := exec.Command("git", "-C", r.Root, "checkout", branch).CombinedOutput(); err != nil {
		return fmt.Errorf("checkout %s 失败: %s", branch, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("git", "-C", r.Root, "pull", "--ff-only", "origin", branch).CombinedOutput(); err != nil {
		return fmt.Errorf("pull %s 失败: %s", branch, strings.TrimSpace(string(out)))
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

// FetchBranch 更新指定分支到远程最新
// 处理策略：
// 1. 远程分支「origin/xxx」→ 只拉远程跟踪引用「git fetch origin xxx」
// 2. 本地分支未被 checkout → git fetch origin <b>:<b> 强制更新
// 3. 本地分支被某个 worktree checkout → 在那个 worktree 目录里 git pull --ff-only
func (r *Repo) FetchBranch(branch string) error {
	if strings.Contains(branch, "/") {
		// 远程分支：只更新远程引用
		parts := strings.SplitN(branch, "/", 2)
		remote, name := parts[0], parts[1]
		cmd := exec.Command("git", "-C", r.Root, "fetch", remote, name)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("fetch %s 失败: %s", branch, strings.TrimSpace(string(out)))
		}
		return nil
	}

	// 本地分支：先确认远程有对应分支
	remoteRef := "refs/remotes/origin/" + branch
	if err := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", remoteRef).Run(); err != nil {
		return nil // 无远程对应分支，跳过
	}

	// 尝试 fetch <b>:<b> —— 未 checkout 的分支能成功
	cmd := exec.Command("git", "-C", r.Root, "fetch", "origin", branch+":"+branch)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// 失败：分支正在某个 worktree 里 checkout；找到它并在那里 pull
	wtPath := r.findWorktreeForBranch(branch)
	if wtPath == "" {
		return fmt.Errorf("fetch %s 失败: %s", branch, strings.TrimSpace(string(out)))
	}
	pullCmd := exec.Command("git", "-C", wtPath, "pull", "--ff-only", "origin", branch)
	if pullOut, pullErr := pullCmd.CombinedOutput(); pullErr != nil {
		return fmt.Errorf("在 %s 上 pull %s 失败: %s", wtPath, branch, strings.TrimSpace(string(pullOut)))
	}
	return nil
}

// findWorktreeForBranch 找出 branch 当前被哪个 worktree checkout；找不到返回空
func (r *Repo) findWorktreeForBranch(branch string) string {
	trees, err := r.ListWorktrees()
	if err != nil {
		return ""
	}
	for _, t := range trees {
		if t.Branch == branch {
			return t.Path
		}
	}
	// 主仓库也可能 checkout 了该分支
	if head, err := exec.Command("git", "-C", r.Root, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		if strings.TrimSpace(string(head)) == branch {
			return r.Root
		}
	}
	return ""
}
