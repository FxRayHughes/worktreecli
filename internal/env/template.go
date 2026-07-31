package env

import "strings"

// Render 用 Vars 中的真实值替换脚本中的占位符
// 支持形式：$NAME、${NAME}、$env:NAME（PowerShell）
func Render(script string, vars Vars) string {
	if script == "" {
		return ""
	}
	repl := map[string]string{
		"CODEX_SOURCE_TREE_PATH": vars.SourceTreePath,
		"CODEX_WORKTREE_PATH":    vars.WorktreePath,
		"CODEX_WORKTREE_NAME":    vars.WorktreeName,
		"CODEX_BRANCH":           vars.Branch,
		"CODEX_BASE_BRANCH":      vars.BaseBranch,
	}
	out := script
	for k, v := range repl {
		out = strings.ReplaceAll(out, "${"+k+"}", v)
		out = strings.ReplaceAll(out, "$env:"+k, v)
		out = strings.ReplaceAll(out, "$"+k, v)
	}
	return out
}
