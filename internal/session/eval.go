package session

import (
	"fmt"
	"io"
	"runtime"
	"strings"
)

type evalLauncher struct{}

func (e *evalLauncher) Label() string { return "eval" }

func (e *evalLauncher) Launch(path string, out io.Writer, evalWriter io.Writer, initScript string) error {
	var b strings.Builder
	if runtime.GOOS == "windows" {
		fmt.Fprintf(&b, "Set-Location %s\n", quotePath(path))
	} else {
		fmt.Fprintf(&b, "cd %s\n", quotePath(path))
	}
	if strings.TrimSpace(initScript) != "" {
		b.WriteString(initScript)
		if !strings.HasSuffix(initScript, "\n") {
			b.WriteByte('\n')
		}
	}
	if evalWriter != nil {
		// 输出到 evalWriter（通常是 stdout，供 wrapper 函数 eval）
		fmt.Fprint(evalWriter, b.String())
	}
	printf(out, "\n已输出 eval 片段（含 cd 和 onSpawned 脚本）。请确保通过 wrapper 函数调用 wtc，例如：\n\n  wtc() { local out; out=$(command wtc \"$@\" --eval) && eval \"$out\"; }\n\n")
	return nil
}
