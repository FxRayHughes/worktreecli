package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repo 代表一个 git 仓库
type Repo struct {
	Root string // 仓库根绝对路径
	Name string // 仓库目录名（作为项目名）
}

// DetectRepo 从当前工作目录向上探测 git 仓库
func DetectRepo() (*Repo, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("git 命令失败: %w", err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return nil, fmt.Errorf("未找到 git 仓库根")
	}
	return &Repo{
		Root: filepath.Clean(root),
		Name: filepath.Base(filepath.Clean(root)),
	}, nil
}

// CurrentBranch 返回当前所在分支
func (r *Repo) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "-C", r.Root, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsWorktree 判断当前 Repo 是否为 linked worktree（而非主仓库）
// 依据: worktree 的 --git-dir 指向 <main>/.git/worktrees/<name>，
// 而 --git-common-dir 指向 <main>/.git；两者不同即为 worktree
func (r *Repo) IsWorktree() (bool, error) {
	gitDir, err := runGit(r.Root, "rev-parse", "--git-dir")
	if err != nil {
		return false, err
	}
	commonDir, err := runGit(r.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return false, err
	}
	// 转成绝对路径再比较，避免相对路径干扰
	absGit, _ := filepath.Abs(filepath.Join(r.Root, gitDir))
	absCommon, _ := filepath.Abs(filepath.Join(r.Root, commonDir))
	return filepath.Clean(absGit) != filepath.Clean(absCommon), nil
}

// MainRepoRoot 返回主仓库根路径（当前 Repo 若为 worktree，回溯到 main）
// 当前若为主仓库，返回自身 Root
func (r *Repo) MainRepoRoot() (string, error) {
	commonDir, err := runGit(r.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	// commonDir 可能是相对路径（相对于 CWD 或 Root）
	abs := commonDir
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(r.Root, commonDir)
	}
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	// commonDir 通常以 `.git` 结尾，去掉即得仓库根
	base := filepath.Base(abs)
	if base == ".git" {
		return filepath.Clean(filepath.Dir(abs)), nil
	}
	// 少见情况：bare 或非标准布局
	return filepath.Clean(abs), nil
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s 失败: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}
