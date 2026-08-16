//go:build windows

package main

import "syscall"

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = (HANDLE)-4
const dpiContextPerMonitorV2 = ^uintptr(3) // 0xFFFF...FFFC

// init 启用 Per-Monitor V2 DPI 感知, 让 GLFW/go-flutter 能拿到真实显示器 DPI,
// 从而在 4K 高 DPI 屏幕上自动放大 UI (图标/文字/布局一起缩放)
func init() {
	user32 := syscall.NewLazyDLL("user32.dll")
	setProcessDpiAwarenessContext := user32.NewProc("SetProcessDpiAwarenessContext")
	if setProcessDpiAwarenessContext.Find() == nil {
		setProcessDpiAwarenessContext.Call(dpiContextPerMonitorV2)
	}
}
