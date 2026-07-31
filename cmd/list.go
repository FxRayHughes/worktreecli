package cmd

import (
	"fmt"

	"github.com/FxRayHughes/worktreecli/internal/git"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "列出当前仓库的所有 worktree",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, err := git.DetectRepo()
		if err != nil {
			return err
		}
		trees, err := repo.ListWorktrees()
		if err != nil {
			return err
		}
		if len(trees) == 0 {
			fmt.Println("(暂无 worktree)")
			return nil
		}
		for _, t := range trees {
			fmt.Printf("%-30s  %-24s  %s\n", t.Name, t.Branch, t.Path)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
