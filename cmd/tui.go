package cmd

import (
	"fmt"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"github.com/FxRayHughes/worktreecli/internal/git"
	"github.com/FxRayHughes/worktreecli/internal/tui"
	"github.com/spf13/cobra"
)

var evalFlag bool

func init() {
	rootCmd.PersistentFlags().BoolVar(&evalFlag, "eval", false, "以 shell eval 模式输出（供 wrapper 函数使用）")
}

func runTUI(_ *cobra.Command, _ []string) error {
	// 首次运行：初始化 ~/.wtc/
	if err := config.EnsureInit(); err != nil {
		return fmt.Errorf("初始化配置目录失败: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	// 尝试探测 git 仓库；失败不阻塞，让用户仍可打开 TUI 管理环境/配置
	repo, _ := git.DetectRepo()

	return tui.Run(tui.Options{
		Config: cfg,
		Repo:   repo,
		Eval:   evalFlag,
	})
}
