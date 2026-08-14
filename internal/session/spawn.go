package session

import (
	"context"
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
	via, err := spawnTerminal(path, initScript)
	if err != nil {
		printf(out, "\n打开新终端失败: %v\n回退到打印模式：\n", err)
		return (&printLauncher{}).Launch(path, out, nil, initScript)
	}
	printf(out, "\n已打开新终端窗口 (%s)，工作目录: %s\n", via, path)
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

// spawnTerminal 打开一个新终端窗口，返回用到的方式（用于给用户显示）
func spawnTerminal(path, initScript string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		return spawnWindows(path, initScript)
	case "darwin":
		return spawnMac(path, initScript)
	default:
		return spawnLinux(path, initScript)
	}
}

func spawnWindows(path, initScript string) (string, error) {
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
		return "", err
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
			return "Windows Terminal", nil
		}
	}
	// 回退 cmd 启动
	cmd := exec.Command("cmd", "/C", "start", psExe,
		"-NoExit", "-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", scriptPath)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return "PowerShell", nil
}

// psSingleQuote 把字符串包成 PowerShell 单引号字面量（内部单引号 → 两个单引号）
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// spawnMac 在 macOS 上开一个新终端窗口，有两条路径：
//
//  1. open 打开一个临时 .command 包装脚本：不走 Apple Events，
//     既不需要「自动化」权限，也不会弹授权框（弹框会一直挡着直到用户点，
//     而那个框常常藏在终端窗口后面）。默认走这条。
//  2. osascript 驱动 Terminal / iTerm：命令被送进一个已经加载过 rc 的交互式
//     shell，语义最原汁原味，但需要 TCC 授权，用户点过“不允许”就永久失败。
//     只在路径 1 失败时兜底。
//
// 最早只有路径 2，而且用的是 cmd.Start() —— osascript 的报错（例如权限被拒的
// -1743）根本不会被看到，于是表现成“提示已打开新终端，但什么都没发生”。
func spawnMac(path, initScript string) (string, error) {
	initPath := ""
	if strings.TrimSpace(initScript) != "" {
		p, err := writeInitScript(initScript, false)
		if err != nil {
			return "", err
		}
		initPath = p
	}

	apps := macTerminalApps()
	var errs []string

	// 路径 1：.command 包装脚本 + open
	if wrapper, err := writeMacWrapper(path, initPath); err != nil {
		errs = append(errs, "写包装脚本失败: "+err.Error())
	} else {
		for _, app := range apps {
			out, err := exec.Command("open", "-a", app.name, wrapper).CombinedOutput()
			if err != nil {
				errs = append(errs, fmt.Sprintf("open -a %s: %v %s", app.name, err, strings.TrimSpace(string(out))))
				continue
			}
			return app.name, nil
		}
	}

	// 路径 2：AppleScript 兜底
	if _, err := exec.LookPath("osascript"); err != nil {
		errs = append(errs, "osascript 不可用")
		return "", errors.New(strings.Join(errs, "; "))
	}
	inline := "cd " + quotePath(path)
	if initPath != "" {
		inline += "; . " + quotePath(initPath)
	}
	for _, app := range apps {
		if app.appleScript == nil {
			continue
		}
		if err := runOsascript(app.appleScript(inline)); err != nil {
			errs = append(errs, fmt.Sprintf("osascript→%s: %v", app.name, err))
			continue
		}
		return app.name + " (AppleScript)", nil
	}
	return "", errors.New(strings.Join(errs, "; "))
}

// macApp 一个可以拿来开新窗口的 macOS 终端 App
type macApp struct {
	name        string                     // open -a / AppleScript 里用的名字
	appleScript func(inline string) string // nil 表示不走 AppleScript
}

// macTerminalApps 返回候选终端，优先用户当前所在的那个，最后一定兜底到 Terminal.app
//
// 只挑 Terminal / iTerm：这两个都能直接执行 .command 文件。其他终端
// （Ghostty / Warp / WezTerm / VS Code / tmux 里…）统一落到 Terminal.app，
// 它在任何 macOS 上都存在。
func macTerminalApps() []macApp {
	terminal := macApp{name: "Terminal", appleScript: func(inline string) string {
		return fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, appleScriptString(inline))
	}}
	iterm := macApp{name: "iTerm", appleScript: func(inline string) string {
		return fmt.Sprintf(`tell application "iTerm"
	activate
	set w to (create window with default profile)
	tell current session of w to write text "%s"
end tell`, appleScriptString(inline))
	}}
	if macInITerm() {
		return []macApp{iterm, terminal}
	}
	return []macApp{terminal}
}

func macInITerm() bool {
	if strings.EqualFold(os.Getenv("TERM_PROGRAM"), "iTerm.app") {
		return true
	}
	return strings.Contains(strings.ToLower(os.Getenv("__CFBundleIdentifier")), "iterm")
}

// appleScriptString 转义成 AppleScript 双引号字面量（\ 和 " 都要转）
func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}

// runOsascript 同步执行 AppleScript，把 stderr 当成错误内容带回来
// （必须 Wait，否则权限被拒之类的错误会被完全吞掉）
func runOsascript(script string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%v: %s", err, msg)
	}
	return nil
}

// writeMacWrapper 生成一个 .command 包装脚本：cd 到 worktree、跑 onSpawned、留一个交互式 shell
func writeMacWrapper(path, initPath string) (string, error) {
	name := fmt.Sprintf("wtc-spawn-%d.command", time.Now().UnixNano())
	wrapper := filepath.Join(os.TempDir(), name)

	// 用 $SHELL -i 而不是 sh，这样 rc 会被加载，onSpawned 里写 nvm/conda 之类才有效；
	// 之后再 exec 一个交互式 shell 把窗口留住（导出的环境变量会被继承）
	last := `exec "$SHELL" -i`
	if initPath != "" {
		last = `exec "$SHELL" -i -c ` + shellSingleQuote(". "+quotePath(initPath)+`; exec "$SHELL" -i`)
	}
	// 临时文件延迟自删，避免 sh 还在按块读脚本时把自己删掉
	cleanup := "( sleep 30; rm -f " + quotePath(wrapper)
	if initPath != "" {
		cleanup += " " + quotePath(initPath)
	}
	cleanup += " ) >/dev/null 2>&1 &"

	body := "#!/bin/sh\n" +
		"cd " + quotePath(path) + " || exit 1\n" +
		cleanup + "\n" +
		last + "\n"
	if err := os.WriteFile(wrapper, []byte(body), 0o700); err != nil {
		return "", err
	}
	return wrapper, nil
}

func spawnLinux(path, initScript string) (string, error) {
	inline := "cd " + quotePath(path)
	if strings.TrimSpace(initScript) != "" {
		p, err := writeInitScript(initScript, false)
		if err != nil {
			return "", err
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
			if err := exec.Command(c.bin, c.args...).Start(); err != nil {
				return "", err
			}
			return c.bin, nil
		}
	}
	return "", errors.New("未找到可用的终端模拟器")
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
