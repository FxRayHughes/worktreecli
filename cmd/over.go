package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"github.com/FxRayHughes/worktreecli/internal/env"
	"github.com/FxRayHughes/worktreecli/internal/git"
	"github.com/FxRayHughes/worktreecli/internal/session"
	"github.com/spf13/cobra"
)

var overCmd = &cobra.Command{
	Use:   "over",
	Short: "结束当前 worktree：删除它并返回源仓库",
	Long: `在 worktree 目录（或残留目录）内执行 wtc over 将：
1. 定位当前所在目录
2. 如果是有效 worktree：走 git worktree remove --force + 兜底清理
3. 如果是 git 引用已断的残留目录：直接强删（会 kill 占用进程）
4. 输出返回源仓库的会话切换指令（按 sessionMode）`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.EnsureInit(); err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		cwd = filepath.Clean(cwd)

		// 关键：wtc 进程立即离开当前目录，避免 cwd 锁
		parent := filepath.Dir(cwd)
		if err := os.Chdir(parent); err != nil {
			// 父目录也进不去时降级到 TEMP
			_ = os.Chdir(os.TempDir())
		}

		// 尝试判断是否为 worktree（在切换到 parent 后仍然可用 git -C）
		targetPath := cwd
		mainRoot := ""
		isWorktree := false

		if repo, rerr := detectRepoAt(cwd); rerr == nil {
			if wt, werr := repo.IsWorktree(); werr == nil && wt {
				isWorktree = true
				if mr, mrErr := repo.MainRepoRoot(); mrErr == nil {
					mainRoot = mr
				}
			}
		}

		if isWorktree {
			// 走 worktree remove 流程
			mainRepo := &git.Repo{
				Root: mainRoot,
				Name: filepath.Base(mainRoot),
			}
			// onCleanup（可选）
			if cfg.DefaultEnvironment != "" {
				if envDef, envErr := env.Load(cfg.DefaultEnvironment); envErr == nil {
					vars := env.Vars{
						SourceTreePath: mainRoot,
						WorktreePath:   targetPath,
						WorktreeName:   filepath.Base(targetPath),
					}
					_ = env.Run(context.Background(), envDef, env.PhaseCleanup, vars, os.Stderr)
				}
			}
			if err := mainRepo.RemoveWorktree(targetPath); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "✓ 已删除 worktree: %s\n", targetPath)
			fmt.Fprintf(os.Stderr, "  源仓库: %s\n", mainRoot)
		} else {
			// 孤立残留：直接强删
			fmt.Fprintf(os.Stderr, "当前目录不是 git worktree，按残留强删处理: %s\n", targetPath)
			if err := git.ForceRemoveDir(targetPath); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "✓ 已删除残留目录: %s\n", targetPath)
			// 没有主仓库信息时，回退目录默认为父目录
			mainRoot = parent
		}

		// 会话切换（复用 sessionMode）
		launcher := session.New(cfg.SessionMode)
		var evalW *evalStdoutWriter
		if evalFlag {
			evalW = &evalStdoutWriter{}
		}
		if err := launcher.Launch(mainRoot, os.Stderr, evalW, ""); err != nil {
			fmt.Fprintln(os.Stderr, "会话切换提示失败:", err)
		}
		if !evalFlag {
			fmt.Fprintln(os.Stderr, "\n提示: 当前 shell 仍在旧目录，请按上述提示切换到目标目录。")
		}
		return nil
	},
}

// detectRepoAt 在指定目录探测 git 仓库（不依赖 cwd）
func detectRepoAt(dir string) (*git.Repo, error) {
	// 手工调 git -C <dir> rev-parse --show-toplevel
	// 复用 git 包内部的话需要暴露；这里直接构造 Repo 后交给它的方法
	// 简化：如果 dir 下能找到 .git 或 .git 文件即认为是 git 目录
	if !isGitDir(dir) {
		return nil, fmt.Errorf("非 git 目录")
	}
	return &git.Repo{
		Root: dir,
		Name: filepath.Base(dir),
	}, nil
}

func isGitDir(dir string) bool {
	p := filepath.Join(dir, ".git")
	if info, err := os.Stat(p); err == nil {
		return info.IsDir() || info.Mode().IsRegular()
	}
	return false
}

// evalStdoutWriter: 写到真正的 stdout（供 wrapper 函数 eval）
type evalStdoutWriter struct{}

func (e *evalStdoutWriter) Write(p []byte) (int, error) {
	return fmt.Fprint(os.Stdout, string(p))
}

// 静默 strings 依赖（保留以便将来扩展）
var _ = strings.TrimSpace

func init() {
	rootCmd.AddCommand(overCmd)
}
