package cmd

import (
	"fmt"

	"github.com/FxRayHughes/worktreecli/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "查看/管理配置",
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "打印配置文件路径",
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := config.ConfigFile()
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "显示当前配置",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.EnsureInit(); err != nil {
			return err
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		fmt.Printf("worktreeRoot:       %s\n", cfg.WorktreeRoot)
		fmt.Printf("autoRemove:         %v\n", cfg.AutoRemove)
		fmt.Printf("retention:          %d\n", cfg.Retention)
		fmt.Printf("sessionMode:        %s\n", cfg.SessionMode)
		fmt.Printf("defaultEnvironment: %s\n", cfg.DefaultEnvironment)
		fmt.Printf("editor:             %s\n", cfg.Editor)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configShowCmd)
	rootCmd.AddCommand(configCmd)
}
