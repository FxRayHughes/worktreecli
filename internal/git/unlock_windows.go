//go:build windows

package git

import (
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// killProcessesUsingDir 通过 Windows RestartManager 找出占用目录下任意文件的进程，并强制结束它们
// 依赖 rstrtmgr.dll —— Windows 内置，不需要外部工具
// 静默返回 —— 找不到进程或 API 失败都算尽力而为
func killProcessesUsingDir(root string) {
	files := collectFiles(root, 128)
	if len(files) == 0 {
		return
	}
	pids := listBlockingPIDs(files)
	for _, pid := range pids {
		if p, err := os.FindProcess(int(pid)); err == nil {
			_ = p.Kill()
		}
	}
	// 给系统一点时间关闭句柄
	time.Sleep(400 * time.Millisecond)
}

// collectFiles 收集目录下最多 max 个文件路径
func collectFiles(root string, max int) []string {
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		out = append(out, p)
		if len(out) >= max {
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

// ─── RestartManager 绑定 ─────────────────────────────

var (
	rstrtmgr                = syscall.NewLazyDLL("rstrtmgr.dll")
	procRmStartSession      = rstrtmgr.NewProc("RmStartSession")
	procRmEndSession        = rstrtmgr.NewProc("RmEndSession")
	procRmRegisterResources = rstrtmgr.NewProc("RmRegisterResources")
	procRmGetList           = rstrtmgr.NewProc("RmGetList")
)

const (
	cchRmSessionKey     = 32*2 + 1
	cchRmMaxAppName     = 255
	cchRmMaxSvcName     = 63
	errorMoreData       = 234
)

type rmUniqueProcess struct {
	dwProcessID      uint32
	processStartTime syscall.Filetime
}

type rmProcessInfo struct {
	Process             rmUniqueProcess
	strAppName          [cchRmMaxAppName + 1]uint16
	strServiceShortName [cchRmMaxSvcName + 1]uint16
	ApplicationType     uint32
	AppStatus           uint32
	TSSessionID         uint32
	bRestartable        int32
}

func listBlockingPIDs(files []string) []uint32 {
	var handle uint32
	sessionKey := make([]uint16, cchRmSessionKey)
	r, _, _ := procRmStartSession.Call(
		uintptr(unsafe.Pointer(&handle)),
		0,
		uintptr(unsafe.Pointer(&sessionKey[0])),
	)
	if r != 0 {
		return nil
	}
	defer procRmEndSession.Call(uintptr(handle))

	// 转换文件路径为 UTF16 指针数组
	pathPtrs := make([]*uint16, 0, len(files))
	for _, f := range files {
		p, err := syscall.UTF16PtrFromString(f)
		if err != nil {
			continue
		}
		pathPtrs = append(pathPtrs, p)
	}
	if len(pathPtrs) == 0 {
		return nil
	}

	r, _, _ = procRmRegisterResources.Call(
		uintptr(handle),
		uintptr(len(pathPtrs)),
		uintptr(unsafe.Pointer(&pathPtrs[0])),
		0, 0,
		0, 0,
	)
	if r != 0 {
		return nil
	}

	var (
		procInfoNeeded uint32
		procInfoCount  uint32 = 64
		rebootReasons  uint32
	)
	buf := make([]rmProcessInfo, procInfoCount)
	r, _, _ = procRmGetList.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&procInfoNeeded)),
		uintptr(unsafe.Pointer(&procInfoCount)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&rebootReasons)),
	)
	// ERROR_MORE_DATA 表示 buffer 不够；仍可用已返回的部分
	if r != 0 && r != errorMoreData {
		return nil
	}
	out := make([]uint32, 0, procInfoCount)
	for i := uint32(0); i < procInfoCount; i++ {
		pid := buf[i].Process.dwProcessID
		if pid == 0 || pid == uint32(os.Getpid()) {
			continue
		}
		out = append(out, pid)
	}
	return out
}
