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

	// 校验 git 仓库
	repo, err := git.DetectRepo()
	if err != nil {
		return fmt.Errorf("当前目录不是 git 仓库: %w", err)
	}

	return tui.Run(tui.Options{
		Config: cfg,
		Repo:   repo,
		Eval:   evalFlag,
	})
}
