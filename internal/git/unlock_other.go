//go:build !windows

package git

// killProcessesUsingDir 在非 Windows 平台是 no-op
// POSIX 的删除语义无需事先解锁：文件即使被打开也能 unlink
func killProcessesUsingDir(_ string) {}
