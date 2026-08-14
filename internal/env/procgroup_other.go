//go:build !windows

package env

import (
	"os/exec"
	"syscall"
)

// setProcGroup 让脚本跑在自己的进程组里
// 取消时才能用 kill(-pgid) 把 pnpm / node 这些子孙进程一起收掉，
// 否则只杀得掉最外层的 bash，留下一堆孤儿进程继续跑（还在偷偷占带宽）
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killGroup 收掉整个进程组
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
