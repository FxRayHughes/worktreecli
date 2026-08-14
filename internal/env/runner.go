package env

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Phase 表示执行阶段
type Phase string

const (
	PhaseCreate  Phase = "create"
	PhaseCleanup Phase = "cleanup"
)

// Vars 是注入到脚本环境的变量集
type Vars struct {
	SourceTreePath string // CODEX_SOURCE_TREE_PATH
	WorktreePath   string // CODEX_WORKTREE_PATH
	WorktreeName   string // CODEX_WORKTREE_NAME
	Branch         string // CODEX_BRANCH
	BaseBranch     string // CODEX_BASE_BRANCH
}

// ToEnv 转成 KEY=VALUE 列表
func (v Vars) ToEnv() []string {
	return []string{
		"CODEX_SOURCE_TREE_PATH=" + v.SourceTreePath,
		"CODEX_WORKTREE_PATH=" + v.WorktreePath,
		"CODEX_WORKTREE_NAME=" + v.WorktreeName,
		"CODEX_BRANCH=" + v.Branch,
		"CODEX_BASE_BRANCH=" + v.BaseBranch,
	}
}

// Run 执行环境的指定阶段脚本
// stdout/stderr 会被合并输出到 out
func Run(ctx context.Context, e *Environment, phase Phase, vars Vars, out io.Writer) error {
	if e == nil {
		return nil
	}
	var scripts Scripts
	switch phase {
	case PhaseCreate:
		scripts = e.OnCreate
	case PhaseCleanup:
		scripts = e.OnCleanup
	default:
		return fmt.Errorf("未知 phase: %s", phase)
	}
	script := strings.TrimSpace(scripts.PickScript(runtime.GOOS))
	if script == "" {
		fmt.Fprintf(out, "[env] 无 %s 脚本，跳过\n", phase)
		return nil
	}

	shell, args := shellCommand(script)
	cmd := exec.CommandContext(ctx, shell, args...)
	// ctx 取消时连子孙进程一起杀：脚本里常是 pnpm install / 构建这类会拉一大串子进程的命令，
	// 默认只杀最外层 shell，剩下的会变成孤儿继续跑
	setProcGroup(cmd)
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = 3 * time.Second
	cmd.Env = append(os.Environ(), vars.ToEnv()...)
	cmd.Stdout = out
	cmd.Stderr = out
	// 工作目录默认在 worktree
	if vars.WorktreePath != "" {
		cmd.Dir = vars.WorktreePath
	}

	fmt.Fprintf(out, "[env] 执行 %s（%s）\n", e.Name, phase)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("脚本执行失败: %w", err)
	}
	return nil
}

// shellCommand 返回执行 script 用的 shell + 参数
func shellCommand(script string) (string, []string) {
	if runtime.GOOS == "windows" {
		// 优先 PowerShell（更贴合 windows: 脚本块）
		if _, err := exec.LookPath("pwsh"); err == nil {
			return "pwsh", []string{"-NoProfile", "-NonInteractive", "-Command", script}
		}
		return "powershell", []string{"-NoProfile", "-NonInteractive", "-Command", script}
	}
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash", []string{"-lc", script}
	}
	return "sh", []string{"-c", script}
}
