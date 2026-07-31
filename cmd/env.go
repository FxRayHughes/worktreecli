package cmd

import (
	"fmt"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"github.com/FxRayHughes/worktreecli/internal/env"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "管理环境（~/.wtc/environments）",
}

var envLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "列出所有环境",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.EnsureInit(); err != nil {
			return err
		}
		envs, err := env.List()
		if err != nil {
			return err
		}
		dir, _ := config.EnvironmentsDir()
		fmt.Println("环境目录:", dir)
		for _, e := range envs {
			fmt.Printf("  %-24s  %s\n", e.FileName, e.Name)
		}
		if len(envs) == 0 {
			fmt.Println("  (空)")
		}
		return nil
	},
}

var envPathCmd = &cobra.Command{
	Use:   "path",
	Short: "打印环境目录路径",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := config.EnvironmentsDir()
		if err != nil {
			return err
		}
		fmt.Println(dir)
		return nil
	},
}

func init() {
	envCmd.AddCommand(envLsCmd)
	envCmd.AddCommand(envPathCmd)
	rootCmd.AddCommand(envCmd)
}
