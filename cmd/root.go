package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "wtc",
	Aliases: []string{"worktreecli"},
	Short:   "Git worktree + 环境脚本 快捷管理工具",
	Long:    "wtc 让你在 git 项目下一键创建 worktree、执行环境初始化脚本，并进入新的工作会话。",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTUI(cmd, args)
	},
	SilenceUsage: true,
}

// Execute 是入口
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "wtc:", err)
		os.Exit(1)
	}
}
