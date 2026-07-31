package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/FxRayHughes/worktreecli/internal/git"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:     "rm <name>",
	Aliases: []string{"remove"},
	Short:   "删除指定 worktree（按名称匹配）",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		repo, err := git.DetectRepo()
		if err != nil {
			return err
		}
		trees, err := repo.ListWorktrees()
		if err != nil {
			return err
		}
		var target *git.Worktree
		for i := range trees {
			if trees[i].Name == name || filepath.Base(trees[i].Path) == name {
				target = &trees[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("未找到 worktree: %s", name)
		}
		if err := repo.RemoveWorktree(target.Path); err != nil {
			return err
		}
		_ = os.RemoveAll(target.Path)
		fmt.Printf("✓ 已删除 %s\n", target.Path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
