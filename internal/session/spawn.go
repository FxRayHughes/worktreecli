package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type spawnLauncher struct{}

func (s *spawnLauncher) Label() string { return "spawn" }

func (s *spawnLauncher) Launch(path string, out io.Writer, _ io.Writer, initScript string) error {
	if err := spawnTerminal(path, initScript); err != nil {
		printf(out, "\n打开新终端失败: %v\n回退到打印模式：\n", err)
		return (&printLauncher{}).Launch(path, out, nil, initScript)
	}
	printf(out, "\n已打开新终端窗口，工作目录: %s\n", path)
	if strings.TrimSpace(initScript) != "" {
		printf(out, "onSpawned 脚本已注入到新终端执行\n")
	}
	return nil
}

// writeInitScript 把 onSpawned 脚本落到临时文件
// 返回文件路径和一个清理函数（延迟 30s 由子 shell 自行删）
func writeInitScript(script string, isWindows bool) (string, error) {
	dir := os.TempDir()
	ext := ".sh"
	if isWindows {
		ext = ".ps1"
	}
	name := fmt.Sprintf("wtc-init-%d%s", time.Now().UnixNano(), ext)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(script), 0o700); err != nil {
		return "", err
	}
	return p, nil
}

func spawnTerminal(path, initScript string) error {
	switch runtime.GOOS {
	case "windows":
		return spawnWindows(path, initScript)
	case "darwin":
		return spawnMac(path, initScript)
	default:
		return spawnLinux(path, initScript)
	}
}

func spawnWindows(path, initScript string) error {
	// 把 Set-Location 与 initScript 合成一个 ps1 文件
	// 避免 wt.exe / powershell 的 -Command 参数拼接坑
	body := "Set-Location -LiteralPath " + psSingleQuote(path) + "\n"
	if strings.TrimSpace(initScript) != "" {
		body += initScript
		if !strings.HasSuffix(initScript, "\n") {
			body += "\n"
		}
	}
	scriptPath, err := writeInitScript(body, true)
	if err != nil {
		return err
	}
	// 选可用的 PowerShell
	psExe := "powershell.exe"
	if p, err := exec.LookPath("pwsh"); err == nil {
		psExe = p
	}
	// 优先 Windows Terminal
	if _, err := exec.LookPath("wt.exe"); err == nil {
		cmd := exec.Command("wt.exe", "-d", path,
			psExe, "-NoExit", "-NoProfile",
			"-ExecutionPolicy", "Bypass",
			"-File", scriptPath)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	// 回退 cmd 启动
	cmd := exec.Command("cmd", "/C", "start", psExe,
		"-NoExit", "-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath)
	return cmd.Start()
}

// psSingleQuote 把字符串包成 PowerShell 单引号字面量（内部单引号 → 两个单引号）
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func spawnMac(path, initScript string) error {
	if _, err := exec.LookPath("osascript"); err != nil {
		return errors.New("osascript 不可用")
	}
	// AppleScript do script 里 " 需要转义为 \"
	inline := "cd " + quotePath(path)
	if strings.TrimSpace(initScript) != "" {
		p, err := writeInitScript(initScript, false)
		if err != nil {
			return err
		}
		inline += "; . " + quotePath(p)
	}
	// 把双引号转成 AppleScript 字面量
	escaped := strings.ReplaceAll(inline, "\"", "\\\"")
	script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, escaped)
	return exec.Command("osascript", "-e", script).Start()
}

func spawnLinux(path, initScript string) error {
	inline := "cd " + quotePath(path)
	if strings.TrimSpace(initScript) != "" {
		p, err := writeInitScript(initScript, false)
		if err != nil {
			return err
		}
		inline += "; . " + quotePath(p)
	}
	// 大多数终端要求 -e sh -c "..." 才能执行完保留 shell
	inline += "; exec $SHELL"
	candidates := []struct {
		bin  string
		args []string
	}{
		{"x-terminal-emulator", []string{"-e", "sh", "-c", inline}},
		{"gnome-terminal", []string{"--working-directory", path, "--", "sh", "-c", inline}},
		{"konsole", []string{"--workdir", path, "-e", "sh", "-c", inline}},
		{"xfce4-terminal", []string{"--working-directory", path, "--command", "sh -c " + shellSingleQuote(inline)}},
		{"alacritty", []string{"--working-directory", path, "-e", "sh", "-c", inline}},
		{"kitty", []string{"-d", path, "sh", "-c", inline}},
		{"xterm", []string{"-e", "sh", "-c", inline}},
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err == nil {
			return exec.Command(c.bin, c.args...).Start()
		}
	}
	return errors.New("未找到可用的终端模拟器")
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
