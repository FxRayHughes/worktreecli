//go:build windows

package env

import (
	"os/exec"
	"strconv"
	"syscall"
)

// createNewProcessGroup = CREATE_NEW_PROCESS_GROUP
const createNewProcessGroup = 0x00000200

func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
}

// killGroup 用 taskkill /T 把整棵进程树收掉（pnpm / node 这些子孙进程）
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	if err := exec.Command("taskkill", "/T", "/F", "/PID", pid).Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
