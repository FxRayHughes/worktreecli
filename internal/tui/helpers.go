package tui

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/FxRayHughes/worktreecli/internal/config"
)

// runtimeGOOS 暴露 runtime.GOOS 便于测试或跨包使用
func runtimeGOOS() string { return runtime.GOOS }

// evalStdoutWriter 用于 --eval 模式：把 shell 片段写到真正的 stdout
// 与 TUI 的 alt-screen 无关，直接 fmt.Fprint 到 os.Stdout 会在 tea 退出后由 shell 捕获
type evalStdoutWriter struct{}

func (e *evalStdoutWriter) Write(p []byte) (int, error) {
	return fmt.Fprint(os.Stdout, string(p))
}

func envPath(fileName string) string {
	dir, _ := config.EnvironmentsDir()
	return filepath.Join(dir, fileName)
}

func cfgFilePath() (string, error) {
	return config.ConfigFile()
}

func envDirPath() (string, error) {
	return config.EnvironmentsDir()
}

func saveCfg(cfg *config.Config) error {
	return config.Save(cfg)
}

// openInEditor 用系统关联程序打开文件
// Windows: rundll32 shell32.dll,ShellExec_RunDLL
// macOS: open
// Linux: xdg-open
func openInEditor(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	var cmd *osexec.Cmd
	switch runtime.GOOS {
	case "windows":
		// 用 rundll32 触发 ShellExecute 打开关联程序
		cmd = osexec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = osexec.Command("open", path)
	default:
		cmd = osexec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func createNewEnvFile() (string, error) {
	dir, err := config.EnvironmentsDir()
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("custom-%d.yml", time.Now().Unix())
	p := filepath.Join(dir, name)
	body := `# 环境名称（在 TUI 列表中显示）
name: "新环境"
# 环境描述
description: "在此填写环境说明"

# 平台块规则（onCreate / onSpawned / onCleanup 通用）:
#   default / macos / linux / windows —— 优先匹配当前系统，回退 default，留空跳过
#
# 可用环境变量:
#   $CODEX_SOURCE_TREE_PATH  — 原始 git 仓库路径
#   $CODEX_WORKTREE_PATH     — 新建 worktree 绝对路径
#   $CODEX_WORKTREE_NAME     — worktree 名称
#   $CODEX_BRANCH            — 新建分支名
#   $CODEX_BASE_BRANCH       — 基线分支名
#   (Windows 用 $env:变量名)

# onCreate: worktree 创建后立即在后台执行（wtc 子进程）
onCreate:
  default: |
    cd "$CODEX_WORKTREE_PATH"
    echo "hello from $CODEX_WORKTREE_NAME"
  macos: ""
  linux: ""
  windows: ""

# onSpawned: 会话模式为 spawn 时，在新终端窗口内自动执行的脚本
# 适合激活 venv、启动 dev server 等需要交互式 shell 的动作
onSpawned:
  default: ""
  macos: ""
  linux: ""
  windows: ""

# onCleanup: worktree 被删除前执行（可选）
onCleanup:
  default: ""
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		return "", err
	}
	return p, nil
}
