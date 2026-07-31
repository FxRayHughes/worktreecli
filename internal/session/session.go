package session

import (
	"fmt"
	"io"

	"github.com/FxRayHughes/worktreecli/internal/config"
)

// Launcher 会话启动策略
type Launcher interface {
	// Launch 让用户进入 path 对应的 worktree
	// out: 用于展示提示信息（例如 print 模式打印 cd 命令）
	// evalWriter: 非 nil 时表示外层要 eval 输出（Launcher 应把 shell 片段写到这里）
	// initScript: spawn 模式在新终端内自动执行的脚本（onSpawned），其他模式可忽略
	Launch(path string, out io.Writer, evalWriter io.Writer, initScript string) error
	// Label 用于在 TUI 里显示
	Label() string
}

// New 根据模式返回对应实现
func New(mode config.SessionMode) Launcher {
	switch mode {
	case config.SessionSpawn:
		return &spawnLauncher{}
	case config.SessionEval:
		return &evalLauncher{}
	default:
		return &printLauncher{}
	}
}

// Modes 返回全部模式（供 TUI 下拉展示）
func Modes() []config.SessionMode {
	return []config.SessionMode{config.SessionPrint, config.SessionSpawn, config.SessionEval}
}

// ModeLabel 转成中文标签
func ModeLabel(m config.SessionMode) string {
	switch m {
	case config.SessionSpawn:
		return "打开新终端窗口 (spawn)"
	case config.SessionEval:
		return "输出 shell eval 片段 (eval)"
	default:
		return "打印 cd 命令 + 剪贴板 (print)"
	}
}

// quotePath 为 shell 中的路径添加安全引号（单引号包裹，内部单引号转义）
func quotePath(p string) string {
	return "'" + escapeSingleQuotes(p) + "'"
}

func escapeSingleQuotes(s string) string {
	out := ""
	for _, r := range s {
		if r == '\'' {
			out += "'\\''"
			continue
		}
		out += string(r)
	}
	return out
}

// helper: 打印
func printf(w io.Writer, format string, a ...any) {
	fmt.Fprintf(w, format, a...)
}
