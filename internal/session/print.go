package session

import (
	"io"
	"runtime"
	"strings"
)

type printLauncher struct{}

func (p *printLauncher) Label() string { return "print" }

func (p *printLauncher) Launch(path string, out io.Writer, _ io.Writer, initScript string) error {
	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "Set-Location " + quotePath(path)
	} else {
		cmd = "cd " + quotePath(path)
	}
	printf(out, "\n请在你的终端里执行以下命令进入 worktree：\n\n  %s\n\n", cmd)
	if strings.TrimSpace(initScript) != "" {
		printf(out, "然后可执行以下 onSpawned 脚本：\n\n%s\n", initScript)
	}
	if err := writeClipboard(cmd); err == nil {
		printf(out, "(已尝试复制到剪贴板)\n")
	}
	return nil
}
