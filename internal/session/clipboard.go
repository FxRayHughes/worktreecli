package session

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// writeClipboard 使用系统工具尝试写入剪贴板
// 失败时静默返回错误
func writeClipboard(text string) error {
	switch runtime.GOOS {
	case "windows":
		return writeVia("clip", nil, text)
	case "darwin":
		return writeVia("pbcopy", nil, text)
	default:
		// Wayland
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return writeVia("wl-copy", nil, text)
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			return writeVia("xclip", []string{"-selection", "clipboard"}, text)
		}
		if _, err := exec.LookPath("xsel"); err == nil {
			return writeVia("xsel", []string{"--clipboard", "--input"}, text)
		}
		return errors.New("未找到剪贴板工具（wl-copy/xclip/xsel）")
	}
}

func writeVia(bin string, args []string, text string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
