package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Worktree 表示一个 git worktree 记录
type Worktree struct {
	Path   string
	Name   string // 目录名
	Branch string
	Head   string
	Bare   bool
}

// AddWorktree 从 base 分支拉出新分支并挂到 path
// 若 newBranch 已存在则直接 checkout；否则创建
func (r *Repo) AddWorktree(path, base, newBranch string) error {
	// 探测本地分支是否已存在
	check := exec.Command("git", "-C", r.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+newBranch)
	if err := check.Run(); err == nil {
		// 已存在
		cmd := exec.Command("git", "-C", r.Root, "worktree", "add", path, newBranch)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git worktree add 失败: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	// 从 base 创建新分支
	cmd := exec.Command("git", "-C", r.Root, "worktree", "add", "-b", newBranch, path, base)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add 失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ListWorktrees 返回除主仓库外的所有 worktree
func (r *Repo) ListWorktrees() ([]Worktree, error) {
	cmd := exec.Command("git", "-C", r.Root, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("列 worktree 失败: %w", err)
	}
	var (
		result []Worktree
		cur    Worktree
	)
	flush := func() {
		if cur.Path != "" && filepath.Clean(cur.Path) != r.Root {
			cur.Name = filepath.Base(cur.Path)
			result = append(result, cur)
		}
		cur = Worktree{}
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		case line == "bare":
			cur.Bare = true
		}
	}
	flush()
	return result, nil
}

// RemoveWorktree 删除指定 worktree
// 先尝试 git worktree remove --force
// 失败时（常见于 Windows 文件锁），改用手动清理目录 + git worktree prune 兜底
// 若目录仍有被占用的文件，会先尝试标记为 stale 让 git 忘掉它，剩余文件可稍后手动清理
func (r *Repo) RemoveWorktree(path string) error {
	// 第一轮：git 原生 remove（带重试）
	var gitErr error
	for i := 0; i < 3; i++ {
		cmd := exec.Command("git", "-C", r.Root, "worktree", "remove", "--force", path)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		gitErr = fmt.Errorf("git worktree remove 失败: %s", strings.TrimSpace(string(out)))
		time.Sleep(300 * time.Millisecond)
	}
	// 第二轮：手动删目录（带重试）
	if rmErr := forceRemoveDir(path); rmErr == nil {
		if pruneErr := r.PruneWorktrees(); pruneErr != nil {
			return fmt.Errorf("目录已删除但 prune 失败: %w", pruneErr)
		}
		return nil
	}
	// 第三轮：Windows 强制解除文件占用（kill 占用进程），再删一次
	killProcessesUsingDir(path)
	if rmErr := forceRemoveDir(path); rmErr == nil {
		if pruneErr := r.PruneWorktrees(); pruneErr != nil {
			return fmt.Errorf("目录已删除但 prune 失败: %w", pruneErr)
		}
		return nil
	} else {
		// 兜底：重命名让 git 遗忘该 worktree
		stalePath := path + ".stale"
		if renameErr := os.Rename(path, stalePath); renameErr == nil {
			_ = r.PruneWorktrees()
			return fmt.Errorf("%v；部分文件仍被占用，已将残留目录标记为 %s 并 prune，请稍后手动删除\n最后错误: %w",
				gitErr, stalePath, rmErr)
		}
		return fmt.Errorf("%v；kill 占用进程后仍失败: %w", gitErr, rmErr)
	}
}

// forceRemoveDir 尽力删除目录
// Windows 下先给所有子文件加写权限，规避只读文件（如 .git/index）导致的 remove 失败
// 带重试，缓解杀软/索引器短暂占用文件的情况
func forceRemoveDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	// 递归清除只读属性
	_ = filepath.Walk(path, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info != nil {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	var lastErr error
	for i := 0; i < 5; i++ {
		lastErr = os.RemoveAll(path)
		if lastErr == nil {
			return nil
		}
		time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
	}
	return lastErr
}

// PruneWorktrees 清理无效引用
func (r *Repo) PruneWorktrees() error {
	cmd := exec.Command("git", "-C", r.Root, "worktree", "prune")
	return cmd.Run()
}

// ForceRemoveDir 对任意目录做"强删"（不依赖 git）
// 顺序：chmod + RemoveAll → kill 占用进程 → 再 RemoveAll → 重命名为 .stale
// 用于处理 git 引用已断但磁盘目录残留的情况
func ForceRemoveDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if err := forceRemoveDir(path); err == nil {
		return nil
	}
	killProcessesUsingDir(path)
	if err := forceRemoveDir(path); err == nil {
		return nil
	}
	// 最后一招：改名让路径消失
	stale := path + ".stale"
	if err := os.Rename(path, stale); err == nil {
		return fmt.Errorf("部分文件仍被占用，已重命名为 %s，请稍后手动删除", stale)
	}
	return fmt.Errorf("无法删除 %s，且无法重命名（父目录也可能被占用）", path)
}

// ShortHash 生成 6 位短哈希（基于当前纳秒时间）
func ShortHash() string {
	return fmt.Sprintf("%06x", time.Now().UnixNano()&0xFFFFFF)
}
